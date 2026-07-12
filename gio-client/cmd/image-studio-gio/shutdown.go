package main

import "time"

const (
	desktopWindowShutdownTimeout      = 5 * time.Second
	desktopWindowShutdownPollInterval = 20 * time.Millisecond
)

type desktopWindowShutdownController interface {
	CloseAll() int
	Count() int
}

type shutdownCloser interface {
	Close() error
}

type shutdownLogFunc func(format string, args ...any)

// closeShutdownResource closes resources that must be released before the GUI
// goroutine terminates the process with os.Exit, which skips deferred calls.
func closeShutdownResource(resource shutdownCloser, name string, logf shutdownLogFunc) {
	if resource == nil {
		return
	}
	if err := resource.Close(); err != nil && logf != nil {
		logf("failed to close %s: %v", name, err)
	}
}

// shutdownDesktopResources stops new cross-process commands before tearing
// down detached windows that still publish commands to the main UI actor.
func shutdownDesktopResources(
	resource shutdownCloser,
	controller desktopWindowShutdownController,
	timeout time.Duration,
	pollInterval time.Duration,
	logf shutdownLogFunc,
) bool {
	closeShutdownResource(resource, "prompt IPC server", logf)
	return closeDesktopWindowsAndWait(controller, timeout, pollInterval, logf)
}

// closeDesktopWindowsAndWait gives detached windows time to process their close
// event and finish unregistering before the process exits.
func closeDesktopWindowsAndWait(
	controller desktopWindowShutdownController,
	timeout time.Duration,
	pollInterval time.Duration,
	logf shutdownLogFunc,
) bool {
	if controller == nil {
		return true
	}

	requested := controller.CloseAll()
	remaining := controller.Count()
	if remaining == 0 {
		return true
	}
	if logf != nil {
		logf("waiting for %d detached window(s) to close (close requested for %d)", remaining, requested)
	}
	if timeout <= 0 {
		if logf != nil {
			logf("timed out waiting for detached windows to close; %d window(s) remain", remaining)
		}
		return false
	}
	if pollInterval <= 0 {
		pollInterval = time.Millisecond
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			remaining = controller.Count()
			if remaining == 0 {
				if logf != nil {
					logf("all detached windows closed")
				}
				return true
			}
		case <-timer.C:
			// Recheck at the deadline so a concurrent final unregister is not
			// reported as a timeout merely because both channels became ready.
			remaining = controller.Count()
			if remaining == 0 {
				if logf != nil {
					logf("all detached windows closed")
				}
				return true
			}
			if logf != nil {
				logf("timed out waiting for detached windows to close; %d window(s) remain", remaining)
			}
			return false
		}
	}
}
