package ui

import (
	"testing"
	"time"

	sharedCompat "image-studio/shared/compat"
)

func TestHistoryTimelineDataSupportsPickedDateFilter(t *testing.T) {
	day := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
	app := &App{
		historyTimelineDateFilter: "pick",
	}
	app.historyTimelinePickedDateInput.SetText("2026-06-10")

	items := []sharedCompat.HistoryItem{
		{ID: "history-1", Prompt: "cat", CreatedAt: day.Add(12 * time.Hour).UnixMilli()},
		{ID: "history-2", Prompt: "dog", CreatedAt: day.Add(36 * time.Hour).UnixMilli()},
	}

	data := app.historyTimelineData(items)
	if data.filteredCount != 1 {
		t.Fatalf("filteredCount=%d want 1", data.filteredCount)
	}
	if len(data.dayGroups) != 1 || len(data.dayGroups[0].Entries) != 1 {
		t.Fatalf("dayGroups=%+v want one filtered entry", data.dayGroups)
	}
	if entry := data.dayGroups[0].Entries[0]; entry.Item == nil || entry.Item.ID != "history-1" {
		t.Fatalf("filtered entry=%+v want history-1", entry)
	}
}

func TestOpenHistoryTimelineKeepsIndependentTimelineFilters(t *testing.T) {
	app := &App{
		historyModeFilter:         "edit",
		historyDateFilter:         "week",
		historyTimelineModeFilter: "generate",
		historyTimelineDateFilter: "pick",
	}
	app.historyQueryInput.SetText("rail-query")
	app.historyTimelineQueryInput.SetText("timeline-query")
	app.historyTimelinePickedDateInput.SetText("2026-06-10")

	app.openHistoryTimeline()

	if !app.historyTimelineOpen {
		t.Fatal("timeline should be open")
	}
	if app.historyTimelineModeFilter != "generate" {
		t.Fatalf("timeline mode=%q want generate", app.historyTimelineModeFilter)
	}
	if app.historyTimelineDateFilter != "pick" {
		t.Fatalf("timeline date filter=%q want pick", app.historyTimelineDateFilter)
	}
	if got := app.historyTimelineQueryInput.Text(); got != "timeline-query" {
		t.Fatalf("timeline query=%q want timeline-query", got)
	}
	if got := app.historyTimelinePickedDateInput.Text(); got != "2026-06-10" {
		t.Fatalf("timeline picked date=%q want 2026-06-10", got)
	}
}
