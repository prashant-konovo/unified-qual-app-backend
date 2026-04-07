package service

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/InCrowd/unified-qual-api/internal/integration"
)

// AvailabilitySlot represents a parsed availability time window for a moderator.
type AvailabilitySlot struct {
	ModeratorID int64
	ClientID    int64
	StartTime   time.Time
	EndTime     time.Time
}

// ParseAvailabilityFromGrid parses a Google Sheets grid to extract color-coded
// moderator availability slots. Matches legacy get-imported-availability-from-current-date.js.
//
// Color mapping (legacy):
//
//	green (0,1,0) → available (type=1)
//	all other colors → not available
//
// The grid is expected to have dates in row 2 (columns F+) and 15-minute time slots
// starting from row 3. Each cell's background color indicates availability.
func ParseAvailabilityFromGrid(grid *integration.SheetGridData, moderatorID, clientID int64) ([]AvailabilitySlot, error) {
	if grid == nil || len(grid.Sheets) == 0 || len(grid.Sheets[0].Data) == 0 {
		return nil, fmt.Errorf("empty grid data")
	}

	sheetData := grid.Sheets[0].Data[0]
	rows := sheetData.RowData
	if len(rows) < 3 {
		return nil, fmt.Errorf("grid has insufficient rows (need at least 3, got %d)", len(rows))
	}

	// Row 2 (index 1) contains date formulas/values starting from column F (index 5)
	dateRow := rows[1].Values
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Build date columns: columns F (index 5) through AL (index 37)
	const startCol = 5
	maxCol := len(dateRow)
	if maxCol <= startCol {
		return nil, fmt.Errorf("grid has no date columns (need columns beyond F)")
	}

	// Generate dates starting from today (legacy uses NOW()+i formulas)
	dates := make([]time.Time, 0, maxCol-startCol)
	for i := 0; i < maxCol-startCol && i <= 32; i++ {
		dates = append(dates, today.AddDate(0, 0, i))
	}

	// Time slots: each row from row 3+ represents a 15-minute block.
	// Row 3 = first time slot of the day. Legacy starts at a configured time
	// (typically row 3 = 08:00, row 4 = 08:15, etc.)
	// We parse the time from column A/B if available, else assume 15-min increments from 08:00.
	var slots []AvailabilitySlot

	for rowIdx := 2; rowIdx < len(rows); rowIdx++ {
		row := rows[rowIdx].Values

		// Try to get the time label from column A (index 0) or B (index 1)
		slotTime := resolveSlotTime(row, rowIdx-2)
		if slotTime < 0 {
			continue
		}

		slotHour := slotTime / 60
		slotMin := slotTime % 60

		for colIdx := startCol; colIdx < len(row) && colIdx-startCol < len(dates); colIdx++ {
			cell := row[colIdx]
			bg := cell.EffectiveFormat.BackgroundColor

			if isGreen(bg.Red, bg.Green, bg.Blue) {
				date := dates[colIdx-startCol]
				start := time.Date(date.Year(), date.Month(), date.Day(),
					slotHour, slotMin, 0, 0, date.Location())
				end := start.Add(15 * time.Minute)

				slots = append(slots, AvailabilitySlot{
					ModeratorID: moderatorID,
					ClientID:    clientID,
					StartTime:   start,
					EndTime:     end,
				})
			}
		}
	}

	return slots, nil
}

// isGreen checks if a background color is "green" (available) per legacy logic.
// Legacy: color.green === 1 && color.red === undefined && color.blue === undefined
// In the Sheets API, undefined maps to 0.
func isGreen(red, green, blue float64) bool {
	return green >= 0.9 && red < 0.1 && blue < 0.1
}

// resolveSlotTime tries to determine the time-of-day (in minutes since midnight)
// for a given row. Falls back to 08:00 + (rowOffset * 15min).
func resolveSlotTime(row []struct {
	FormattedValue  string `json:"formattedValue"`
	EffectiveFormat struct {
		BackgroundColor struct {
			Red   float64 `json:"red"`
			Green float64 `json:"green"`
			Blue  float64 `json:"blue"`
		} `json:"backgroundColor"`
	} `json:"effectiveFormat"`
}, rowOffset int) int {
	// Try column A for time label (e.g., "08:00", "08:15")
	if len(row) > 0 && row[0].FormattedValue != "" {
		t, err := time.Parse("15:04", row[0].FormattedValue)
		if err == nil {
			return t.Hour()*60 + t.Minute()
		}
		// Try "3:04 PM" format
		t, err = time.Parse("3:04 PM", row[0].FormattedValue)
		if err == nil {
			return t.Hour()*60 + t.Minute()
		}
	}
	// Fallback: 08:00 + 15-min increments
	return 8*60 + rowOffset*15
}

// MergeOverlappingSlots merges adjacent and overlapping availability slots
// into contiguous blocks. Matches legacy merge logic in get-imported-moderator-availability.js.
func MergeOverlappingSlots(slots []AvailabilitySlot) []AvailabilitySlot {
	if len(slots) == 0 {
		return nil
	}

	// Sort by start time
	sort.Slice(slots, func(i, j int) bool {
		return slots[i].StartTime.Before(slots[j].StartTime)
	})

	merged := []AvailabilitySlot{slots[0]}
	for i := 1; i < len(slots); i++ {
		last := &merged[len(merged)-1]
		cur := slots[i]

		// Merge if same moderator/client and adjacent or overlapping
		if cur.ModeratorID == last.ModeratorID &&
			cur.ClientID == last.ClientID &&
			!cur.StartTime.After(last.EndTime) {
			if cur.EndTime.After(last.EndTime) {
				last.EndTime = cur.EndTime
			}
		} else {
			merged = append(merged, cur)
		}
	}

	return merged
}

// RunModeratorImport executes the full moderator availability import from Google Sheets.
// It fetches the sheet grid, parses colors, merges slots, and inserts into the database.
// This runs as a background goroutine — the handler returns immediately.
func (s *ModeratorService) RunModeratorImport(ctx context.Context, moderatorID, clientID int64, spreadsheetID string) {
	go func() {
		// Use a fresh context with a generous timeout (not tied to the HTTP request)
		importCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		slog.Info("moderator import started", "moderatorId", moderatorID, "clientId", clientID)

		// Step 1: Fetch Google Sheets grid data
		if s.googleSheets == nil || !s.googleSheets.Configured() {
			slog.Error("moderator import failed: Google Sheets not configured")
			_ = s.qsUserRepo.UpdateModExternalCalendarStatusMRA(importCtx, moderatorID, "Failed")
			return
		}

		grid, err := s.googleSheets.GetGridData(importCtx, spreadsheetID)
		if err != nil {
			slog.Error("moderator import failed: fetch grid data", "error", err)
			_ = s.qsUserRepo.UpdateModExternalCalendarStatusMRA(importCtx, moderatorID, "Failed")
			return
		}

		// Step 2: Parse availability slots from grid colors
		slots, err := ParseAvailabilityFromGrid(grid, moderatorID, clientID)
		if err != nil {
			slog.Error("moderator import failed: parse grid", "error", err)
			_ = s.qsUserRepo.UpdateModExternalCalendarStatusMRA(importCtx, moderatorID, "Failed")
			return
		}

		// Step 3: Merge overlapping slots
		merged := MergeOverlappingSlots(slots)

		slog.Info("moderator import parsed", "moderatorId", moderatorID,
			"rawSlots", len(slots), "mergedSlots", len(merged))

		// Step 4: Insert merged slots into DB
		for _, slot := range merged {
			startStr := slot.StartTime.Format("2006-01-02 15:04:05")
			endStr := slot.EndTime.Format("2006-01-02 15:04:05")
			if err := s.qsUserRepo.AddModeratorAvailabilityFromImportedMRA(importCtx, moderatorID, clientID, startStr, endStr); err != nil {
				slog.Error("moderator import insert failed", "error", err, "start", startStr, "end", endStr)
			}
		}

		// Step 5: Mark import as completed
		if err := s.qsUserRepo.UpdateModExternalCalendarStatusMRA(importCtx, moderatorID, "Completed"); err != nil {
			slog.Error("moderator import status update failed", "error", err)
			return
		}

		slog.Info("moderator import completed", "moderatorId", moderatorID,
			"clientId", clientID, "slotsInserted", len(merged))
	}()
}
