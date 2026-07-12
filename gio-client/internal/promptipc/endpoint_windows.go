//go:build windows

package promptipc

import (
	"fmt"
	"hash/fnv"
	"net"

	gioCompat "image-studio/gio-client/internal/compat"
)

func endpoint() (string, string, error) {
	root, err := gioCompat.StableDataRoot()
	if err != nil {
		return "", "", err
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(root))
	port := 42000 + int(hash.Sum32()%6000)
	return "tcp", fmt.Sprintf("127.0.0.1:%d", port), nil
}

func listenEndpoint(network, address string) (net.Listener, endpointLease, bool, error) {
	if err := sendMessage(network, address, Message{Type: MessageTypePing}); err == nil {
		return nil, nil, true, nil
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		if sendErr := sendMessage(network, address, Message{Type: MessageTypePing}); sendErr == nil {
			return nil, nil, true, nil
		}
		return nil, nil, false, err
	}
	return listener, nil, false, nil
}
