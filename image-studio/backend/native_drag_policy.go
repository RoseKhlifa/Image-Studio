package backend

import "time"

const nativeDragMaxDuration = 30 * time.Second

type nativeDragAction uint8

const (
	nativeDragContinue nativeDragAction = iota
	nativeDragDrop
	nativeDragCancel
)

func nativeDragNextAction(escape, leftButtonDown, asyncLeftButtonDown bool, elapsed time.Duration) nativeDragAction {
	if escape || elapsed >= nativeDragMaxDuration {
		return nativeDragCancel
	}
	if !leftButtonDown || !asyncLeftButtonDown {
		return nativeDragDrop
	}
	return nativeDragContinue
}
