//go:build !windows && (!linux || android)

package promptscheme

import "fmt"

func RegisterExecutable(_ string) error {
	return fmt.Errorf("protocol registration is not supported on this platform")
}

func UnregisterExecutable(_ string) error {
	return fmt.Errorf("protocol registration is not supported on this platform")
}

func StatusForExecutable(_ string) (Status, error) {
	return Status{}, fmt.Errorf("protocol registration is not supported on this platform")
}
