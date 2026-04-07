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

// SheetGridData holds the raw grid data from a Google Sheets spreadsheet,
// including cell values and background colors for availability parsing.
type SheetGridData struct {
	Sheets []struct {
		Data []struct {
			RowData []struct {
				Values []struct {
					FormattedValue  string `json:"formattedValue"`
					EffectiveFormat struct {
						BackgroundColor struct {
							Red   float64 `json:"red"`
							Green float64 `json:"green"`
							Blue  float64 `json:"blue"`
						} `json:"backgroundColor"`
					} `json:"effectiveFormat"`
				} `json:"values"`
			} `json:"rowData"`
		} `json:"data"`
	} `json:"sheets"`
}

// GetGridData fetches the full spreadsheet grid with formatting data (cell colors).
// Uses the spreadsheets.get endpoint with includeGridData=true.
func (gs *GoogleSheetsClient) GetGridData(ctx context.Context, spreadsheetID string) (*SheetGridData, error) {
	sid := spreadsheetID
	if sid == "" {
		sid = gs.spreadsheetID
	}
	if sid == "" {
		return nil, fmt.Errorf("no spreadsheet ID provided")
	}

	apiURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s?includeGridData=true",
		url.PathEscape(sid),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google sheets grid data call: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read grid data response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("google sheets grid data returned %d: %s", resp.StatusCode, string(body))
	}

	var grid SheetGridData
	if err := json.Unmarshal(body, &grid); err != nil {
		return nil, fmt.Errorf("parse grid data: %w", err)
	}
	return &grid, nil
}
