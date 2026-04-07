package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ──────────────────────────────────────────────────────────────────────────────
// InCrowdAPI Auth Client — proxies AuthN/AuthZ to InCrowdAPI
// ──────────────────────────────────────────────────────────────────────────────

// InCrowdAPIAuthClient proxies authentication requests to InCrowdAPI.
type InCrowdAPIAuthClient struct {
	baseURL string
	client  *http.Client
}

func newInCrowdAPIAuthClient(baseURL string) *InCrowdAPIAuthClient {
	return &InCrowdAPIAuthClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 15 * time.Second},
	}
}

func (ic *InCrowdAPIAuthClient) Configured() bool { return ic.baseURL != "" }

// ICLoginResponse holds the relevant fields from InCrowdAPI POST /v1/user/login.
type ICLoginResponse struct {
	ID            int64    `json:"id"`
	Email         string   `json:"email"`
	FirstName     string   `json:"firstName"`
	LastName      string   `json:"lastName"`
	CognitoToken  string   `json:"cognitoToken"`
	AccessToken   string   `json:"accessToken"`
	AcceptedTerms bool     `json:"acceptedTerms"`
	Roles         []string `json:"roles"`
	MustChangePwd bool     `json:"mustChangePassword"`
}

// Login calls InCrowdAPI POST /v1/user/login and returns the response.
func (ic *InCrowdAPIAuthClient) Login(ctx context.Context, email, password string, brandID int64) (*ICLoginResponse, int, error) {
	if !ic.Configured() {
		return nil, 0, fmt.Errorf("incrowd api auth client not configured")
	}

	payload, _ := json.Marshal(map[string]any{
		"email":    email,
		"password": password,
		"brandId":  brandID,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ic.baseURL+"/v1/user/login", bytes.NewReader(payload))
	if err != nil {
		return nil, 0, fmt.Errorf("create login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ic.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("incrowd api login call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		// Return the status code and raw body so caller can forward the error
		return nil, resp.StatusCode, fmt.Errorf("%s", string(body))
	}

	var loginResp ICLoginResponse
	if err := json.Unmarshal(body, &loginResp); err != nil {
		return nil, resp.StatusCode, fmt.Errorf("parse login response: %w", err)
	}

	return &loginResp, resp.StatusCode, nil
}

// RefreshToken calls InCrowdAPI GET /v1/refresh-token/:userId to get a new idToken.
func (ic *InCrowdAPIAuthClient) RefreshToken(ctx context.Context, userID int64, icAuthToken string) (string, error) {
	if !ic.Configured() {
		return "", fmt.Errorf("incrowd api auth client not configured")
	}

	url := fmt.Sprintf("%s/v1/refresh-token/%d", ic.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create refresh request: %w", err)
	}
	req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))

	resp, err := ic.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("incrowd api refresh call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refresh failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse refresh response: %w", err)
	}

	return result.IDToken, nil
}

// AcceptTerms calls InCrowdAPI POST /v1/user/:userId/accept_terms.
func (ic *InCrowdAPIAuthClient) AcceptTerms(ctx context.Context, userID int64) error {
	if !ic.Configured() {
		return fmt.Errorf("incrowd api auth client not configured")
	}

	url := fmt.Sprintf("%s/v1/user/%d/accept_terms", ic.baseURL, userID)
	payload, _ := json.Marshal(map[string]any{})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create accept terms request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := ic.client.Do(req)
	if err != nil {
		return fmt.Errorf("incrowd api accept terms call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("accept terms failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// Logout calls InCrowdAPI GET /v1/user/logout/:userId to revoke the session.
func (ic *InCrowdAPIAuthClient) Logout(ctx context.Context, userID int64, icAuthToken string) error {
	if !ic.Configured() {
		return fmt.Errorf("incrowd api auth client not configured")
	}

	logoutURL := fmt.Sprintf("%s/v1/user/logout/%d", ic.baseURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, logoutURL, nil)
	if err != nil {
		return fmt.Errorf("create logout request: %w", err)
	}
	if icAuthToken != "" {
		req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))
	}

	resp, err := ic.client.Do(req)
	if err != nil {
		return fmt.Errorf("incrowd api logout call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("logout failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// ChangePassword calls InCrowdAPI to update the user's password.
func (ic *InCrowdAPIAuthClient) ChangePassword(ctx context.Context, userID int64, icAuthToken, oldPassword, newPassword string) error {
	if !ic.Configured() {
		return fmt.Errorf("incrowd api auth client not configured")
	}

	changePwdURL := fmt.Sprintf("%s/v1/user/%d/change_password", ic.baseURL, userID)
	payload, _ := json.Marshal(map[string]any{
		"oldPassword": oldPassword,
		"newPassword": newPassword,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, changePwdURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create change password request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if icAuthToken != "" {
		req.Header.Set("IC-Auth", fmt.Sprintf("%d:%s", userID, icAuthToken))
	}

	resp, err := ic.client.Do(req)
	if err != nil {
		return fmt.Errorf("incrowd api change password call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("change password failed (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

// PasswordMatches calls InCrowdAPI PUT /v1/user/:userId/password_matches_qstool
// to validate that the provided password matches the user's current password.
func (ic *InCrowdAPIAuthClient) PasswordMatches(ctx context.Context, userID int64, password, authToken string) (bool, error) {
	if !ic.Configured() {
		return false, fmt.Errorf("incrowd api auth client not configured")
	}

	url := fmt.Sprintf("%s/v1/user/%d/password_matches_qstool", ic.baseURL, userID)
	payload, _ := json.Marshal(map[string]any{"password": password})

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("create password matches request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := ic.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("incrowd api password matches call: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("password matches failed (%d): %s", resp.StatusCode, string(body))
	}

	var result struct {
		PasswordMatch bool `json:"passwordMatch"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return false, fmt.Errorf("parse password matches response: %w", err)
	}
	return result.PasswordMatch, nil
}
