//go:build (linux && !android) || (darwin && !ios)

package promptipc

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

const (
	endpointClaimTimeout = 2 * time.Second
	endpointClaimPoll    = 10 * time.Millisecond
)

var localEndpointClaims = struct {
	sync.Mutex
	paths map[string]struct{}
}{paths: make(map[string]struct{})}

type unixEndpointLease struct {
	file       *os.File
	lockPath   string
	socketInfo os.FileInfo
	once       sync.Once
	err        error
}

func listenEndpoint(network, address string) (net.Listener, endpointLease, bool, error) {
	deadline := time.Now().Add(endpointClaimTimeout)
	for {
		lease, busy, err := tryAcquireUnixEndpointLease(address)
		if err != nil {
			return nil, nil, false, err
		}
		if busy {
			if err := sendMessage(network, address, Message{Type: MessageTypePing}); err == nil {
				return nil, nil, true, nil
			}
			if time.Now().After(deadline) {
				return nil, nil, false, fmt.Errorf("prompt IPC endpoint lock remained busy without a responding server: %q", address)
			}
			time.Sleep(endpointClaimPoll)
			continue
		}

		// A live pre-lock-protocol server may still own the socket. Preserve
		// compatibility by probing it before treating the path as stale.
		if err := sendMessage(network, address, Message{Type: MessageTypePing}); err == nil {
			_ = lease.release("")
			return nil, nil, true, nil
		}
		if err := removeStaleUnixSocket(address); err != nil {
			_ = lease.release("")
			return nil, nil, false, err
		}

		listener, err := net.ListenUnix(network, &net.UnixAddr{Name: address, Net: network})
		if err != nil {
			_ = lease.release("")
			return nil, nil, false, err
		}
		// The lease owns path cleanup. Go's automatic unlink is path-based and
		// could otherwise remove a replacement socket with a different inode.
		listener.SetUnlinkOnClose(false)
		info, err := os.Lstat(address)
		if err != nil {
			_ = listener.Close()
			_ = removeStaleUnixSocket(address)
			_ = lease.release("")
			return nil, nil, false, fmt.Errorf("record prompt IPC socket identity: %w", err)
		}
		lease.socketInfo = info
		return listener, lease, false, nil
	}
}

func tryAcquireUnixEndpointLease(address string) (*unixEndpointLease, bool, error) {
	lockPath := filepath.Clean(address + ".lock")
	if !reserveLocalEndpoint(lockPath) {
		return nil, true, nil
	}
	releaseReservation := true
	defer func() {
		if releaseReservation {
			releaseLocalEndpoint(lockPath)
		}
	}()

	fd, err := unix.Open(lockPath, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open prompt IPC endpoint lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), lockPath)
	if file == nil {
		_ = unix.Close(fd)
		return nil, false, errors.New("create prompt IPC endpoint lock file handle")
	}
	closeFile := true
	defer func() {
		if closeFile {
			_ = file.Close()
		}
	}()

	var stat unix.Stat_t
	if err := unix.Fstat(fd, &stat); err != nil {
		return nil, false, fmt.Errorf("inspect prompt IPC endpoint lock: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, false, fmt.Errorf("prompt IPC endpoint lock is not a regular file: %q", lockPath)
	}
	if stat.Uid != uint32(os.Geteuid()) {
		return nil, false, fmt.Errorf("prompt IPC endpoint lock is not owned by the current user: %q", lockPath)
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("lock prompt IPC endpoint: %w", err)
	}
	if err := unix.Fchmod(fd, 0o600); err != nil {
		_ = unix.Flock(fd, unix.LOCK_UN)
		return nil, false, fmt.Errorf("secure prompt IPC endpoint lock: %w", err)
	}

	releaseReservation = false
	closeFile = false
	return &unixEndpointLease{file: file, lockPath: lockPath}, false, nil
}

func (l *unixEndpointLease) release(address string) error {
	if l == nil {
		return nil
	}
	l.once.Do(func() {
		var removeErr error
		if address != "" && l.socketInfo != nil {
			removeErr = removeOwnedUnixSocket(address, l.socketInfo)
		}
		unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
		closeErr := l.file.Close()
		releaseLocalEndpoint(l.lockPath)
		l.err = errors.Join(removeErr, unlockErr, closeErr)
	})
	return l.err
}

func removeStaleUnixSocket(address string) error {
	info, err := os.Lstat(address)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect prompt IPC socket: %w", err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("refusing to remove non-socket prompt IPC endpoint: %q", address)
	}
	if err := os.Remove(address); err != nil {
		return fmt.Errorf("remove stale prompt IPC socket: %w", err)
	}
	return nil
}

func removeOwnedUnixSocket(address string, owned os.FileInfo) error {
	current, err := os.Lstat(address)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect owned prompt IPC socket: %w", err)
	}
	if !os.SameFile(current, owned) {
		return nil
	}
	if err := os.Remove(address); err != nil {
		return fmt.Errorf("remove owned prompt IPC socket: %w", err)
	}
	return nil
}

func reserveLocalEndpoint(lockPath string) bool {
	localEndpointClaims.Lock()
	defer localEndpointClaims.Unlock()
	if _, exists := localEndpointClaims.paths[lockPath]; exists {
		return false
	}
	localEndpointClaims.paths[lockPath] = struct{}{}
	return true
}

func releaseLocalEndpoint(lockPath string) {
	localEndpointClaims.Lock()
	delete(localEndpointClaims.paths, lockPath)
	localEndpointClaims.Unlock()
}
