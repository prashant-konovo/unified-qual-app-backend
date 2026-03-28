package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTAuth validates Cognito JWTs using the public JWKS.
type JWTAuth struct {
	region     string
	userPoolID string
	clientIDs  map[string]bool // recognised Cognito app client IDs

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey // kid → RSA public key
}

// NewJWTAuth creates a new JWT authentication middleware that validates
// tokens issued by the given Cognito user pool.
// clientIDs lists all recognised app client IDs (for audience validation).
func NewJWTAuth(region, userPoolID string, clientIDs []string) *JWTAuth {
	cidMap := make(map[string]bool, len(clientIDs))
	for _, id := range clientIDs {
		if id != "" {
			cidMap[id] = true
		}
	}
	j := &JWTAuth{
		region:     region,
		userPoolID: userPoolID,
		clientIDs:  cidMap,
		keys:       make(map[string]*rsa.PublicKey),
	}
	// Pre-fetch JWKS at startup (non-fatal).
	if err := j.refreshKeys(); err != nil {
		log.Printf("[auth] WARNING failed to fetch JWKS at startup: %v", err)
	}
	return j
}

// Middleware returns an http.Handler middleware that validates JWTs.
func (j *JWTAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := extractToken(r)
		if tokenStr == "" {
			http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
			return
		}

		claims, err := j.validateToken(tokenStr)
		if err != nil {
			log.Printf("[auth] token validation failed: %v", err)
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}

		r = SetUser(r, claims)
		next.ServeHTTP(w, r)
	})
}

// extractToken pulls the JWT from Authorization header or CognitoToken header.
func extractToken(r *http.Request) string {
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return parts[1]
		}
	}
	// Legacy header used by QS-Tool / InCrowdWeb
	if tok := r.Header.Get("CognitoToken"); tok != "" {
		return tok
	}
	return ""
}

// validateToken parses and validates the JWT, returning extracted claims.
func (j *JWTAuth) validateToken(tokenStr string) (*UserClaims, error) {
	token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, fmt.Errorf("missing kid in token header")
		}

		key := j.getKey(kid)
		if key == nil {
			// Key not found — try refreshing JWKS (key rotation).
			if err := j.refreshKeys(); err != nil {
				return nil, fmt.Errorf("jwks refresh failed: %w", err)
			}
			key = j.getKey(kid)
			if key == nil {
				return nil, fmt.Errorf("unknown kid: %s", kid)
			}
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(j.issuerURL()),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, err
	}

	mapClaims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Validate token_use is "id" (id tokens carry user attributes).
	// We also accept "access" tokens for API calls.
	tokenUse, _ := mapClaims["token_use"].(string)
	if tokenUse != "id" && tokenUse != "access" {
		return nil, fmt.Errorf("invalid token_use: %s", tokenUse)
	}

	// For id tokens, validate audience matches one of our client IDs.
	if tokenUse == "id" {
		aud, _ := mapClaims["aud"].(string)
		if len(j.clientIDs) > 0 && !j.clientIDs[aud] {
			return nil, fmt.Errorf("audience mismatch: %s not in recognised clients", aud)
		}
	}

	// For access tokens, validate client_id claim.
	if tokenUse == "access" {
		cid, _ := mapClaims["client_id"].(string)
		if len(j.clientIDs) > 0 && !j.clientIDs[cid] {
			return nil, fmt.Errorf("client_id mismatch: %s not in recognised clients", cid)
		}
	}

	return extractClaims(mapClaims), nil
}

// extractClaims pulls user identity from JWT map claims.
func extractClaims(m jwt.MapClaims) *UserClaims {
	c := &UserClaims{}
	c.Sub, _ = m["sub"].(string)
	c.Email, _ = m["email"].(string)
	c.Username, _ = m["cognito:username"].(string)

	// cognito:groups is a JSON array in the token.
	if groups, ok := m["cognito:groups"].([]any); ok {
		for _, g := range groups {
			if s, ok := g.(string); ok {
				c.Groups = append(c.Groups, s)
			}
		}
	}

	// Derive unified roles from Cognito groups
	c.Roles = MapCognitoGroupsToRoles(c.Groups)

	return c
}

func (j *JWTAuth) issuerURL() string {
	return fmt.Sprintf("https://cognito-idp.%s.amazonaws.com/%s", j.region, j.userPoolID)
}

func (j *JWTAuth) jwksURL() string {
	return j.issuerURL() + "/.well-known/jwks.json"
}

func (j *JWTAuth) getKey(kid string) *rsa.PublicKey {
	j.mu.RLock()
	defer j.mu.RUnlock()
	return j.keys[kid]
}

// refreshKeys fetches the JWKS endpoint and caches RSA public keys.
func (j *JWTAuth) refreshKeys() error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(j.jwksURL())
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks returned status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			Alg string `json:"alg"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	newKeys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, k := range jwks.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := parseRSAPublicKey(k.N, k.E)
		if err != nil {
			log.Printf("[auth] skipping kid %s: %v", k.Kid, err)
			continue
		}
		newKeys[k.Kid] = pub
	}

	j.mu.Lock()
	j.keys = newKeys
	j.mu.Unlock()

	log.Printf("[auth] JWKS refreshed: %d keys cached", len(newKeys))
	return nil
}

// parseRSAPublicKey constructs an RSA public key from Base64url-encoded N and E.
func parseRSAPublicKey(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, fmt.Errorf("decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, fmt.Errorf("decode E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{
		N: n,
		E: int(e.Int64()),
	}, nil
}
