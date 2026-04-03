package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/InCrowd/unified-qual-api/internal/config"
)

// ──────────────────────────────────────────────
// Google Sheets Client
// Matches: QS-Tool update-google-sheet-first-date.js
// (google-spreadsheet library for moderator availability)
// ──────────────────────────────────────────────

type GoogleSheetsClient struct {
	spreadsheetID         string
	serviceAccountKeyPath string
	client                *http.Client
}

func newGoogleSheetsClient(cfg config.GoogleSheetsConfig, saKeyPath string, c *http.Client) *GoogleSheetsClient {
	return &GoogleSheetsClient{
		spreadsheetID: cfg.SpreadsheetID, serviceAccountKeyPath: saKeyPath, client: c,
	}
}

func (gs *GoogleSheetsClient) Configured() bool {
	return gs.spreadsheetID != "" && gs.serviceAccountKeyPath != ""
}

func (gs *GoogleSheetsClient) UpdateFirstDate(ctx context.Context, sheetName, cellRange, value string) error {
	if !gs.Configured() {
		return fmt.Errorf("google sheets not configured")
	}
	apiURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s!%s?valueInputOption=USER_ENTERED",
		url.PathEscape(gs.spreadsheetID),
		url.PathEscape(sheetName),
		url.PathEscape(cellRange),
	)

	payload := map[string]any{
		"values": [][]string{{value}},
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Service account auth would be applied via OAuth2 token
	slog.Info("google sheets update first date", "spreadsheet", gs.spreadsheetID, "sheet", sheetName, "value", value)

	resp, err := gs.client.Do(req)
	if err != nil {
		return fmt.Errorf("google sheets API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("google sheets returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
