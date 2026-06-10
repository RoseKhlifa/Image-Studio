package ui

import "testing"

func TestSyncLoopSettingsFromInputsParsesAndClamps(t *testing.T) {
	app := &App{}
	app.loopTotalCountInput.SetText("120")
	app.loopConcurrencyInput.SetText("0")

	app.syncLoopSettingsFromInputs()

	if app.loopTotalCount != maxLoopGenerationCount {
		t.Fatalf("loop total count=%d want %d", app.loopTotalCount, maxLoopGenerationCount)
	}
	if app.loopTotalCountInput.Text() != "99" {
		t.Fatalf("loop total count input=%q want 99", app.loopTotalCountInput.Text())
	}
	if app.loopConcurrency != defaultLoopGenerationConcurrency {
		t.Fatalf("loop concurrency=%d want %d", app.loopConcurrency, defaultLoopGenerationConcurrency)
	}
	if app.loopConcurrencyInput.Text() != "2" {
		t.Fatalf("loop concurrency input=%q want 2", app.loopConcurrencyInput.Text())
	}
}

func TestSetLoopAutoSaveEnabledUsesCurrentOutputDir(t *testing.T) {
	app := &App{}
	app.outputDirInput.SetText("/tmp/output")

	app.setLoopAutoSaveEnabled(true)

	if !app.loopAutoSave {
		t.Fatal("loop auto save should be enabled")
	}
	if app.loopAutoSaveDirInput.Text() != "/tmp/output" {
		t.Fatalf("loop auto save dir=%q want /tmp/output", app.loopAutoSaveDirInput.Text())
	}
}

func TestApplyWorkspaceSyncsLoopEditors(t *testing.T) {
	app := &App{}
	app.applyWorkspace(workspaceState{
		LoopEnabled:       true,
		LoopTotalCount:    20,
		LoopConcurrency:   4,
		LoopAutoSave:      true,
		LoopAutoSaveDir:   "/tmp/loop",
		LoopLivePreview:   false,
		SourcePathsText:   "",
		BatchResultIDs:    nil,
		ResultGridOpen:    false,
		SelectedHistoryID: "",
	})

	if app.loopTotalCountInput.Text() != "20" {
		t.Fatalf("loop total count input=%q want 20", app.loopTotalCountInput.Text())
	}
	if app.loopConcurrencyInput.Text() != "4" {
		t.Fatalf("loop concurrency input=%q want 4", app.loopConcurrencyInput.Text())
	}
	if app.loopAutoSaveDirInput.Text() != "/tmp/loop" {
		t.Fatalf("loop auto save dir=%q want /tmp/loop", app.loopAutoSaveDirInput.Text())
	}
	if app.loopLivePreview {
		t.Fatal("loop live preview should be disabled from workspace state")
	}
}
