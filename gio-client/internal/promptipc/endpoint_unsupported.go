//go:build !windows && (!linux || android) && (!darwin || ios)

package promptipc

import (
	"fmt"
	"net"
)

func endpoint() (string, string, error) {
	return "", "", fmt.Errorf("prompt import control is not supported on this platform")
}

func listenEndpoint(_, _ string) (net.Listener, endpointLease, bool, error) {
	return nil, nil, false, fmt.Errorf("prompt import control is not supported on this platform")
}
