package promptipc

import (
	"bufio"
	"encoding/json"
	"errors"
	"net"
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
	listener     net.Listener
	network      string
	address      string
	lease        endpointLease
	once         sync.Once
	closeErr     error
	stateMu      sync.Mutex
	closing      bool
	connections  map[net.Conn]struct{}
	callbackGate sync.RWMutex
	workerGroup  sync.WaitGroup
}

func TryStart(handler func(Message)) (*Server, bool, error) {
	network, address, err := endpoint()
	if err != nil {
		return nil, false, err
	}
	listener, lease, alreadyRunning, err := listenEndpoint(network, address)
	if err != nil || alreadyRunning {
		return nil, alreadyRunning, err
	}
	if listener == nil {
		if lease != nil {
			_ = lease.release(address)
		}
		return nil, false, errors.New("prompt IPC listener is nil")
	}
	server := &Server{
		listener:    listener,
		network:     network,
		address:     address,
		lease:       lease,
		connections: make(map[net.Conn]struct{}),
	}
	server.workerGroup.Add(1)
	go server.serve(handler)
	return server, false, nil
}

type endpointLease interface {
	release(address string) error
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.once.Do(func() {
		// Exclude callback dispatch while changing the lifecycle state. A callback
		// that already owns the read lock completes before shutdown proceeds.
		s.callbackGate.Lock()
		s.stateMu.Lock()
		s.closing = true
		s.stateMu.Unlock()
		s.callbackGate.Unlock()

		listenerErr := s.listener.Close()
		s.stateMu.Lock()
		connections := make([]net.Conn, 0, len(s.connections))
		for conn := range s.connections {
			connections = append(connections, conn)
		}
		s.stateMu.Unlock()
		for _, conn := range connections {
			_ = conn.Close()
		}
		s.workerGroup.Wait()

		var releaseErr error
		if s.lease != nil {
			releaseErr = s.lease.release(s.address)
		}
		s.closeErr = errors.Join(listenerErr, releaseErr)
	})
	return s.closeErr
}

func sendMessage(network, address string, msg Message) error {
	conn, err := net.DialTimeout(network, address, 500*time.Millisecond)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
	return json.NewEncoder(conn).Encode(msg)
}

func Send(msg Message) error {
	network, address, err := endpoint()
	if err != nil {
		return err
	}
	return sendMessage(network, address, msg)
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
	defer s.workerGroup.Done()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		if !s.trackConnection(conn) {
			_ = conn.Close()
			continue
		}
		go s.handleConnection(conn, handler)
	}
}

func (s *Server) trackConnection(conn net.Conn) bool {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	if s.closing {
		return false
	}
	s.connections[conn] = struct{}{}
	// The serve goroutine remains counted until it can no longer add workers.
	s.workerGroup.Add(1)
	return true
}

func (s *Server) handleConnection(conn net.Conn, handler func(Message)) {
	defer func() {
		_ = conn.Close()
		s.stateMu.Lock()
		delete(s.connections, conn)
		s.stateMu.Unlock()
		s.workerGroup.Done()
	}()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	reader := bufio.NewReader(conn)
	var msg Message
	if err := json.NewDecoder(reader).Decode(&msg); err != nil {
		return
	}

	s.callbackGate.RLock()
	defer s.callbackGate.RUnlock()
	s.stateMu.Lock()
	closing := s.closing
	s.stateMu.Unlock()
	if !closing && handler != nil {
		handler(msg)
	}
}
