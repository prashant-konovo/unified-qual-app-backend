//go:build integration

// Package testserver provides a test HTTP server that wires the real Chi router
// with mock dependencies for integration tests.
package testserver

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/config"
	"github.com/InCrowd/unified-qual-api/internal/handler"
	"github.com/InCrowd/unified-qual-api/internal/middleware"
	"github.com/InCrowd/unified-qual-api/internal/repository/iris"
	"github.com/InCrowd/unified-qual-api/internal/repository/qs"
	"github.com/InCrowd/unified-qual-api/internal/router"
	"github.com/InCrowd/unified-qual-api/internal/service"
	mocks "github.com/InCrowd/unified-qual-api/internal/testutil/mocks"
	"github.com/golang-jwt/jwt/v5"
)

const testKID = "test-kid-001"

// TestServer wraps httptest.Server with mock accessors for integration tests.
type TestServer struct {
	Server *httptest.Server

	// Mock repositories — set expectations before each request
	IrisProjectRepo  *mocks.MockIrisProjectRepository
	QsProjectRepo    *mocks.MockQsProjectRepository
	IrisSurveyRepo   *mocks.MockIrisSurveyRepository
	QsSurveyRepo     *mocks.MockQsSurveyRepository
	IrisUserRepo     *mocks.MockIrisUserRepository
	QsUserRepo       *mocks.MockQsUserRepository
	QsTimeSlotRepo   *mocks.MockQsTimeSlotRepository
	QsRespondentRepo *mocks.MockQsRespondentRepository
	QsConferenceRepo *mocks.MockQsConferenceRepository
	QsAnswerRepo     *mocks.MockQsAnswerRepository
	QsInterviewsRepo *mocks.MockQsInterviewsRepository
	IrisJobsRepo     *mocks.MockIrisJobsRepository

	// JWT signing key (for creating test tokens)
	privateKey *rsa.PrivateKey
	jwksServer *httptest.Server
}

// New creates a TestServer with a real Chi router and mock dependencies.
// The returned server must be closed after use: defer ts.Close()
func New() *TestServer {
	// Generate RSA key pair for JWT signing
	privateKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	ts := &TestServer{
		IrisProjectRepo:  &mocks.MockIrisProjectRepository{},
		QsProjectRepo:    &mocks.MockQsProjectRepository{},
		IrisSurveyRepo:   &mocks.MockIrisSurveyRepository{},
		QsSurveyRepo:     &mocks.MockQsSurveyRepository{},
		IrisUserRepo:     &mocks.MockIrisUserRepository{},
		QsUserRepo:       &mocks.MockQsUserRepository{},
		QsTimeSlotRepo:   &mocks.MockQsTimeSlotRepository{},
		QsRespondentRepo: &mocks.MockQsRespondentRepository{},
		QsConferenceRepo: &mocks.MockQsConferenceRepository{},
		QsAnswerRepo:     &mocks.MockQsAnswerRepository{},
		QsInterviewsRepo: &mocks.MockQsInterviewsRepository{},
		IrisJobsRepo:     &mocks.MockIrisJobsRepository{},
		privateKey:       privateKey,
	}

	// Start a JWKS server that serves the test public key
	ts.jwksServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pubKey := &privateKey.PublicKey
		nBytes := pubKey.N.Bytes()
		eBytes := big.NewInt(int64(pubKey.E)).Bytes()

		jwks := map[string]any{
			"keys": []map[string]any{
				{
					"kid": testKID,
					"kty": "RSA",
					"alg": "RS256",
					"use": "sig",
					"n":   base64.RawURLEncoding.EncodeToString(nBytes),
					"e":   base64.RawURLEncoding.EncodeToString(eBytes),
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))

	// Build config pointing JWT validation at our test JWKS server
	cfg := &config.Config{
		Environment: "test",
		Port:        "0",
		Cognito: config.CognitoConfig{
			UserPoolID:      "test-pool",
			AllClientIDs:    []string{"test-client-id"},
			SSOClientID:     "test-sso-client",
			SSOClientSecret: "test-sso-secret",
			SSORedirectURI:  "http://localhost:3000/login/sso-callback",
			Domain:          "test-auth.example.com",
			Region:          "us-east-1",
		},
		AuthAPIURL: "http://mock-auth-api",
		AuthAPIKey: "test-api-key",
	}

	// Wire mock repos into repository containers
	qsRepos := &qs.Repositories{
		Project:    ts.QsProjectRepo,
		User:       ts.QsUserRepo,
		TimeSlot:   ts.QsTimeSlotRepo,
		Respondent: ts.QsRespondentRepo,
		Conference: ts.QsConferenceRepo,
		Answer:     ts.QsAnswerRepo,
		Interviews: ts.QsInterviewsRepo,
	}
	irisRepos := &iris.Repositories{
		Project: ts.IrisProjectRepo,
		User:    ts.IrisUserRepo,
		Survey:  ts.IrisSurveyRepo,
	}

	// Create all services via centralized wiring
	svcs := service.NewServices(cfg, qsRepos, irisRepos, nil)
	// Override SurveyService: the mock QsSurveyRepo can't be stored in
	// qs.Repositories.Survey (concrete *SurveyRepo), so wire it manually.
	svcs.Survey = service.NewSurveyService(ts.QsSurveyRepo, ts.IrisSurveyRepo, nil, nil)

	db := &config.DBPair{} // nil DBs — health will show "not_configured"
	hs := handler.NewHandlers(cfg, db, svcs)

	// Create JWTAuth that fetches keys from our test JWKS server.
	// Override the issuer URL so token validation matches our test tokens.
	jwtAuth := middleware.NewJWTAuthWithJWKS(
		cfg.Cognito.Region,
		cfg.Cognito.UserPoolID,
		cfg.Cognito.AllClientIDs,
		ts.jwksServer.URL+"/.well-known/jwks.json",
	)

	mux := router.New(hs, jwtAuth)
	ts.Server = httptest.NewServer(mux)

	return ts
}

// Close shuts down both the app server and the JWKS server.
func (ts *TestServer) Close() {
	ts.Server.Close()
	ts.jwksServer.Close()
}

// URL returns the base URL of the test server.
func (ts *TestServer) URL() string {
	return ts.Server.URL
}

// SignToken creates a signed JWT for integration tests.
// Pass claims to customize the token payload.
func (ts *TestServer) SignToken(claims TokenClaims) string {
	now := time.Now()
	mapClaims := jwt.MapClaims{
		"sub":              claims.Sub,
		"email":            claims.Email,
		"cognito:username": claims.Username,
		"token_use":        "id",
		"aud":              "test-client-id",
		"iss":              fmt.Sprintf("https://cognito-idp.us-east-1.amazonaws.com/test-pool"),
		"iat":              now.Unix(),
		"exp":              now.Add(1 * time.Hour).Unix(),
	}

	if len(claims.Groups) > 0 {
		groups := make([]any, len(claims.Groups))
		for i, g := range claims.Groups {
			groups[i] = g
		}
		mapClaims["cognito:groups"] = groups
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, mapClaims)
	token.Header["kid"] = testKID

	signed, err := token.SignedString(ts.privateKey)
	if err != nil {
		panic(fmt.Sprintf("sign test token: %v", err))
	}
	return signed
}

// AdminToken returns a signed JWT for an admin user.
func (ts *TestServer) AdminToken() string {
	return ts.SignToken(TokenClaims{
		Sub:      "test-sub-admin",
		Email:    "admin@test.com",
		Username: "testadmin",
		Groups:   []string{"ADMIN"},
	})
}

// ManagerToken returns a signed JWT for a manager user.
func (ts *TestServer) ManagerToken() string {
	return ts.SignToken(TokenClaims{
		Sub:      "test-sub-manager",
		Email:    "manager@test.com",
		Username: "testmanager",
		Groups:   []string{"QUAL_SCHEDULER_MANAGER"},
	})
}

// TokenClaims configures a test JWT.
type TokenClaims struct {
	Sub      string
	Email    string
	Username string
	Groups   []string
}
