package backend

import (
	"testing"
	"time"
)

func TestNativeDragStopsOnReleaseEscapeAndDeadline(t *testing.T) {
	for _, tc := range []struct {
		name                        string
		escape, leftDown, asyncDown bool
		elapsed                     time.Duration
		want                        nativeDragAction
	}{
		{"held", false, true, true, time.Second, nativeDragContinue},
		{"released", false, false, true, time.Second, nativeDragDrop},
		{"missed mouseup", false, true, false, time.Second, nativeDragDrop},
		{"escape", true, true, true, time.Second, nativeDragCancel},
		{"deadline", false, true, true, nativeDragMaxDuration, nativeDragCancel},
		{"expired release", false, false, false, nativeDragMaxDuration, nativeDragCancel},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nativeDragNextAction(tc.escape, tc.leftDown, tc.asyncDown, tc.elapsed); got != tc.want {
				t.Fatalf("drag action=%v want %v", got, tc.want)
			}
		})
	}
}
