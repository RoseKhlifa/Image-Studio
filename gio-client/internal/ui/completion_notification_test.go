package ui

import (
	"errors"
	"testing"
	"time"

	sharedCompat "image-studio/shared/compat"
)

func TestFailedRunNotifiesAndPlaysConfiguredSound(t *testing.T) {
	app := &App{
		completionSound:                  sharedCompat.CompletionSoundSettings{Enabled: true, Mode: "default"},
		completionNotification:           sharedCompat.CompletionNotificationSettings{Enabled: true},
		completionNotificationPermission: systemNotificationPermissionGranted,
	}
	sounds := make(chan bool, 1)
	notifications := make(chan string, 1)
	origSound, origNotification := playCompletionSoundFunc, showSystemNotificationFunc
	t.Cleanup(func() {
		playCompletionSoundFunc, showSystemNotificationFunc = origSound, origNotification
	})
	playCompletionSoundFunc = func(config sharedCompat.CompletionSoundSettings, force bool) error {
		sounds <- force
		return nil
	}
	showSystemNotificationFunc = func(title, body string, _ notificationOpenResultAction) error {
		notifications <- title + ":" + body
		return nil
	}
	app.finishWithError(errors.New("HTTP 503"), "")
	select {
	case force := <-sounds:
		if force {
			t.Fatal("failure should honor the configured sound setting")
		}
	case <-time.After(time.Second):
		t.Fatal("failure did not trigger sound")
	}
	select {
	case got := <-notifications:
		if got != "Image Studio · 生成失败:HTTP 503" {
			t.Fatalf("notification=%q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("failure did not trigger background notification")
	}
}

func TestShouldSendCompletionNotificationOnlyOnFinalBackgroundResult(t *testing.T) {
	config := sharedCompat.CompletionNotificationSettings{Enabled: true}
	if shouldSendCompletionNotification(config, 1, 3, false) {
		t.Fatal("should not notify before final result")
	}
	if shouldSendCompletionNotification(config, 3, 3, true) {
		t.Fatal("should not notify while window is focused")
	}
	if !shouldSendCompletionNotification(config, 3, 3, false) {
		t.Fatal("should notify on final background result")
	}
}

func TestSetCompletionNotificationEnabledHonoursPermission(t *testing.T) {
	app := &App{
		completionNotification: sharedCompat.CompletionNotificationSettings{Enabled: false},
	}
	orig := requestSystemNotificationPermissionFunc
	requestSystemNotificationPermissionFunc = func() systemNotificationPermissionState {
		return systemNotificationPermissionUnsupported
	}
	defer func() { requestSystemNotificationPermissionFunc = orig }()

	permission := app.setCompletionNotificationEnabled(true)
	if permission != systemNotificationPermissionUnsupported {
		t.Fatalf("permission=%q want unsupported", permission)
	}
	if app.completionNotification.Enabled {
		t.Fatal("notification should remain disabled when permission is unavailable")
	}

	requestSystemNotificationPermissionFunc = func() systemNotificationPermissionState {
		return systemNotificationPermissionGranted
	}
	permission = app.setCompletionNotificationEnabled(true)
	if permission != systemNotificationPermissionGranted {
		t.Fatalf("permission=%q want granted", permission)
	}
	if !app.completionNotification.Enabled {
		t.Fatal("notification should enable after permission is granted")
	}
}

func TestMaybeSendCompletionNotificationRequiresBackgroundAndPermission(t *testing.T) {
	app := &App{
		completionNotification:           sharedCompat.CompletionNotificationSettings{Enabled: true},
		completionNotificationPermission: systemNotificationPermissionGranted,
		windowFocused:                    true,
	}
	calls := make(chan string, 1)
	orig := showSystemNotificationFunc
	showSystemNotificationFunc = func(title string, body string, action notificationOpenResultAction) error {
		calls <- title + "\n" + body + "\n" + action.ResultID + "\n" + action.SavedPath
		return nil
	}
	defer func() { showSystemNotificationFunc = orig }()

	item := sharedCompat.HistoryItem{
		ID:            "history-1",
		Prompt:        "生成一张雪山海报",
		RevisedPrompt: "cinematic snow mountain poster",
		SavedPath:     "/tmp/snow.png",
		Mode:          "generate",
	}
	app.maybeSendCompletionNotification(item, 1, 1)
	select {
	case <-calls:
		t.Fatal("should not notify while window is focused")
	case <-time.After(200 * time.Millisecond):
	}

	app.windowFocused = false
	app.maybeSendCompletionNotification(item, 1, 1)
	select {
	case got := <-calls:
		if got != "Image Studio · 已完成\ncinematic snow mountain poster\nhistory-1\n/tmp/snow.png" {
			t.Fatalf("notification=%q want title/body/action payload", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completion notification")
	}
}
