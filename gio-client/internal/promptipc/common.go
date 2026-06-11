package promptipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type Message struct {
	Type      string `json:"type"`
	Token     string `json:"token,omitempty"`
	ResultID  string `json:"resultID,omitempty"`
	SavedPath string `json:"savedPath,omitempty"`
}

const (
	MessageTypePing       = "ping"
	MessageTypeRaise      = "raise"
	MessageTypeToken      = "token"
	MessageTypeInvalid    = "invalid"
	MessageTypeOpenResult = "open-result"
)

type Server struct {
	listener net.Listener
	network  string
	address  string
	once     sync.Once
}

func TryStart(handler func(Message)) (*Server, bool, error) {
	network, address, err := endpoint()
	if err != nil {
		return nil, false, err
	}
	if network == "unix" {
		if _, statErr := os.Stat(address); statErr == nil {
			if err := Send(Message{Type: MessageTypePing}); err == nil {
				return nil, true, nil
			}
			_ = os.Remove(address)
		}
	} else if err := Send(Message{Type: MessageTypePing}); err == nil {
		return nil, true, nil
	}
	listener, err := net.Listen(network, address)
	if err != nil {
		if sendErr := Send(Message{Type: MessageTypePing}); sendErr == nil {
			return nil, true, nil
		}
		return nil, false, err
	}
	server := &Server{
		listener: listener,
		network:  network,
		address:  address,
	}
	go server.serve(handler)
	return server, false, nil
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	var closeErr error
	s.once.Do(func() {
		closeErr = s.listener.Close()
		if s.network == "unix" {
			_ = os.Remove(s.address)
		}
	})
	return closeErr
}

func Send(msg Message) error {
	network, address, err := endpoint()
	if err != nil {
		return err
	}
	conn, err := net.DialTimeout(network, address, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	return json.NewEncoder(conn).Encode(msg)
}

func SendRaise() error {
	return Send(Message{Type: MessageTypeRaise})
}

func SendToken(token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("token is empty")
	}
	return Send(Message{Type: MessageTypeToken, Token: token})
}

func SendInvalid() error {
	return Send(Message{Type: MessageTypeInvalid})
}

func SendOpenResult(resultID string, savedPath string) error {
	resultID = strings.TrimSpace(resultID)
	savedPath = strings.TrimSpace(savedPath)
	if resultID == "" && savedPath == "" {
		return errors.New("result target is empty")
	}
	return Send(Message{
		Type:      MessageTypeOpenResult,
		ResultID:  resultID,
		SavedPath: savedPath,
	})
}

func (s *Server) serve(handler func(Message)) {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		go func(conn net.Conn) {
			defer conn.Close()
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			reader := bufio.NewReader(conn)
			var msg Message
			if err := json.NewDecoder(reader).Decode(&msg); err != nil {
				return
			}
			if handler != nil {
				handler(msg)
			}
		}(conn)
	}
}
