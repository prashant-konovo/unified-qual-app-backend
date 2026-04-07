package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ──────────────────────────────────────────────
// Google Sheets: BatchUpdateFirstDate
// ──────────────────────────────────────────────

// UpdateFirstDateCalendar implements the legacy 30-day rolling calendar update on Google Sheets.
// It writes weekday names in row 1 and NOW()+i formulas in row 2 for columns F through AL (indices 5..37).
func (gs *GoogleSheetsClient) UpdateFirstDateCalendar(ctx context.Context) error {
	if !gs.Configured() {
		return fmt.Errorf("google sheets not configured")
	}

	// Build the batch update: row 1 = weekday names, row 2 = NOW()+i formulas
	weekdays := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	now := time.Now()

	var row1Values []any // weekday names
	var row2Values []any // formulas

	// Column F (index 5) = today, then 32 more days
	for i := 0; i <= 32; i++ {
		d := now.AddDate(0, 0, i)
		dayName := weekdays[int(d.Weekday())]
		row1Values = append(row1Values, dayName)
		if i == 0 {
			row2Values = append(row2Values, "=NOW()")
		} else {
			row2Values = append(row2Values, fmt.Sprintf("=NOW()+%d", i))
		}
	}

	// Update row 1 (weekday names) — F1:AL1
	if err := gs.batchUpdateRange(ctx, "Sheet1!F1:AL1", [][]any{row1Values}); err != nil {
		return fmt.Errorf("update row 1: %w", err)
	}

	// Update row 2 (formulas) — F2:AL2
	if err := gs.batchUpdateRange(ctx, "Sheet1!F2:AL2", [][]any{row2Values}); err != nil {
		return fmt.Errorf("update row 2: %w", err)
	}

	return nil
}

func (gs *GoogleSheetsClient) batchUpdateRange(ctx context.Context, rangeStr string, values [][]any) error {
	apiURL := fmt.Sprintf(
		"https://sheets.googleapis.com/v4/spreadsheets/%s/values/%s?valueInputOption=USER_ENTERED",
		url.PathEscape(gs.spreadsheetID),
		url.PathEscape(rangeStr),
	)

	payload := map[string]any{
		"range":  rangeStr,
		"values": values,
	}
	b, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, apiURL, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := gs.client.Do(req)
	if err != nil {
		return fmt.Errorf("sheets API call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("sheets returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
