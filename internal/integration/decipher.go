package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Decipher Client
// Matches: QS-Tool helpers.js Decipher API calls
// (survey.opinionsite.com — responder data retrieval)
// ──────────────────────────────────────────────

// DecipherRespondent holds the typed fields returned by the Decipher survey API.
type DecipherRespondent struct {
	Identifier string `json:"identifier"`
	SessKey    string `json:"sesskey"`
	FirstName  string `json:"QContactr1"`
	LastName   string `json:"QContactr2"`
	Email      string `json:"QContactr6"`
	Phone      string `json:"QContactr7"`
	State      string `json:"xState"`
	ID         string `json:"ID"`
	Honorarium string `json:"Honorarium"`
}

type DecipherClient struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newDecipherClient(cfg config.DecipherConfig, c *http.Client) *DecipherClient {
	return &DecipherClient{baseURL: strings.TrimRight(cfg.BaseURL, "/"), apiKey: cfg.APIKey, client: c}
}

func (d *DecipherClient) Configured() bool { return d.baseURL != "" && d.apiKey != "" }

// GetRespondentData fetches all respondent data for a survey from Decipher.
func (d *DecipherClient) GetRespondentData(ctx context.Context, surveyID string) ([]DecipherRespondent, error) {
	if !d.Configured() {
		return nil, fmt.Errorf("decipher not configured")
	}
	apiURL := fmt.Sprintf("%s/surveys/selfserve/20dc/%s/data?format=json&cond=%%22all%%22",
		d.baseURL, url.PathEscape(surveyID))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-apikey", d.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("decipher API call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("decipher read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("decipher returned %d: %s", resp.StatusCode, string(body))
	}

	var result []DecipherRespondent
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decipher response parse error: %w", err)
	}
	return result, nil
}

// GetRespondentByIdentifier fetches survey data and filters to a single respondent.
// Returns an error if no match or multiple matches are found.
func (d *DecipherClient) GetRespondentByIdentifier(ctx context.Context, surveyID, identifier string) (*DecipherRespondent, error) {
	data, err := d.GetRespondentData(ctx, surveyID)
	if err != nil {
		return nil, err
	}

	var matches []DecipherRespondent
	for _, r := range data {
		if r.Identifier == identifier {
			matches = append(matches, r)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no respondent found with identifier %q", identifier)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple respondents (%d) found with identifier %q", len(matches), identifier)
	}

	resp := &matches[0]
	if resp.Identifier == "" || resp.Email == "" {
		return nil, fmt.Errorf("respondent missing required fields (identifier=%q, email=%q)", resp.Identifier, resp.Email)
	}
	return resp, nil
}
