//go:build (linux && !android) || (darwin && !ios)

package promptipc

import (
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
)

func TestUnixEndpointKeepsShortStableDataPath(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "isg-promptipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	setStableDataRoot(t, root)

	network, address, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "control", "prompt-import.sock")
	if network != "unix" || address != want {
		t.Fatalf("endpoint() = %q, %q; want unix, %q", network, address, want)
	}
}

func TestUnixEndpointLongPathUsesDeterministicPrivateFallback(t *testing.T) {
	root := longStableDataRoot(t)
	setStableDataRoot(t, root)

	network, address, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}
	cleanupFallbackEndpoint(t, address)
	if network != "unix" {
		t.Fatalf("network = %q; want unix", network)
	}
	if len(address) > maxUnixSocketPathBytes {
		t.Fatalf("fallback address is %d bytes; limit is %d: %q", len(address), maxUnixSocketPathBytes, address)
	}
	if !pathWithin(address, os.TempDir()) {
		t.Fatalf("fallback address %q is not under temporary directory %q", address, os.TempDir())
	}
	if address == filepath.Join(root, "control", "prompt-import.sock") {
		t.Fatalf("long endpoint did not use fallback: %q", address)
	}

	_, secondAddress, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}
	if secondAddress != address {
		t.Fatalf("fallback changed between calls: %q != %q", secondAddress, address)
	}
	otherAddress, err := shortUnixSocketAddress(root + "-other")
	if err != nil {
		t.Fatal(err)
	}
	cleanupFallbackEndpoint(t, otherAddress)
	if otherAddress == address {
		t.Fatalf("different stable data roots share fallback %q", address)
	}

	dir := filepath.Dir(address)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, _, err := endpoint(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o700 {
		t.Fatalf("fallback directory permissions = %#o; want 0700", got)
	}
}

func TestUnixEndpointLongPathSupportsSendAndSingleton(t *testing.T) {
	root := longStableDataRoot(t)
	setStableDataRoot(t, root)

	received := make(chan Message, 4)
	server, alreadyRunning, err := TryStart(func(msg Message) {
		received <- msg
	})
	if err != nil {
		t.Fatal(err)
	}
	if alreadyRunning || server == nil {
		t.Fatalf("TryStart() = server %v, alreadyRunning %v; want a new server", server, alreadyRunning)
	}
	_, address, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}
	cleanupFallbackEndpoint(t, address)
	t.Cleanup(func() { _ = server.Close() })

	if err := SendToken("long-path-token"); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-received:
		if msg.Type != MessageTypeToken || msg.Token != "long-path-token" {
			t.Fatalf("received message = %#v", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for long-path IPC message")
	}

	second, alreadyRunning, err := TryStart(nil)
	if err != nil {
		t.Fatal(err)
	}
	if second != nil || !alreadyRunning {
		if second != nil {
			_ = second.Close()
		}
		t.Fatalf("second TryStart() = server %v, alreadyRunning %v; want nil, true", second, alreadyRunning)
	}

	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(address); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket remains after close: %v", err)
	}
}

func TestShortUnixSocketAddressRejectsSymlinkDirectory(t *testing.T) {
	root := longStableDataRoot(t)
	address, err := shortUnixSocketAddress(root)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(address)
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	if err := os.Symlink(target, dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dir) })

	if _, err := shortUnixSocketAddress(root); err == nil || !strings.Contains(err.Error(), "not a private directory") {
		t.Fatalf("shortUnixSocketAddress() error = %v; want private-directory error", err)
	}
}

func TestServerCloseRejectsHeldAcceptedConnection(t *testing.T) {
	root := shortStableDataRoot(t)
	setStableDataRoot(t, root)
	callbacks := make(chan Message, 1)
	server, alreadyRunning, err := TryStart(func(msg Message) {
		callbacks <- msg
	})
	if err != nil {
		t.Fatal(err)
	}
	if alreadyRunning {
		t.Fatal("fresh endpoint reported an existing server")
	}
	t.Cleanup(func() { _ = server.Close() })

	conn, err := net.Dial(server.network, server.address)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	waitForConnectionCount(t, server, 1)
	if _, err := conn.Write([]byte(`{"type":"token"`)); err != nil {
		t.Fatal(err)
	}

	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	// A local write may be buffered even after the peer closes; regardless of
	// its result, completing the JSON must never dispatch after Close.
	_, _ = conn.Write([]byte(`,"token":"after-close"}`))
	select {
	case msg := <-callbacks:
		t.Fatalf("handler ran after close for held connection: %#v", msg)
	case <-time.After(100 * time.Millisecond):
	}
	waitForConnectionCount(t, server, 0)
}

func TestServerCloseWaitsForActiveHandler(t *testing.T) {
	root := shortStableDataRoot(t)
	setStableDataRoot(t, root)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	server, alreadyRunning, err := TryStart(func(Message) {
		close(handlerStarted)
		<-releaseHandler
	})
	if err != nil {
		t.Fatal(err)
	}
	if alreadyRunning {
		t.Fatal("fresh endpoint reported an existing server")
	}
	t.Cleanup(func() {
		select {
		case <-releaseHandler:
		default:
			close(releaseHandler)
		}
		_ = server.Close()
	})

	if err := SendToken("block-handler"); err != nil {
		t.Fatal(err)
	}
	select {
	case <-handlerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not start")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- server.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before active handler completed: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	close(releaseHandler)
	select {
	case err := <-closeDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not return after handler completed")
	}
}

func TestConcurrentTryStartRecoversOneStaleUnixEndpoint(t *testing.T) {
	root := shortStableDataRoot(t)
	setStableDataRoot(t, root)
	network, address, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}
	stale, err := net.ListenUnix(network, &net.UnixAddr{Name: address, Net: network})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	if err := stale.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(address); err != nil {
		t.Fatalf("stale socket was not preserved: %v", err)
	}

	const contenderCount = 12
	type result struct {
		server         *Server
		alreadyRunning bool
		err            error
	}
	start := make(chan struct{})
	results := make(chan result, contenderCount)
	var contenders sync.WaitGroup
	for range contenderCount {
		contenders.Add(1)
		go func() {
			defer contenders.Done()
			<-start
			server, alreadyRunning, err := TryStart(nil)
			results <- result{server: server, alreadyRunning: alreadyRunning, err: err}
		}()
	}
	close(start)
	contenders.Wait()
	close(results)

	var winner *Server
	started := 0
	alreadyRunning := 0
	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent TryStart failed: %v", result.err)
		}
		if result.server != nil {
			started++
			winner = result.server
		}
		if result.alreadyRunning {
			alreadyRunning++
		}
	}
	if started != 1 || alreadyRunning != contenderCount-1 {
		if winner != nil {
			_ = winner.Close()
		}
		t.Fatalf("started=%d alreadyRunning=%d; want 1 and %d", started, alreadyRunning, contenderCount-1)
	}
	t.Cleanup(func() { _ = winner.Close() })

	lockInfo, err := os.Lstat(address + ".lock")
	if err != nil {
		t.Fatal(err)
	}
	if !lockInfo.Mode().IsRegular() || lockInfo.Mode().Perm() != 0o600 {
		t.Fatalf("lock mode = %v; want regular 0600", lockInfo.Mode())
	}
	if err := winner.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(address); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("owned socket remains after close: %v", err)
	}
}

func TestServerCloseDoesNotUnlinkReplacementSocket(t *testing.T) {
	root := shortStableDataRoot(t)
	setStableDataRoot(t, root)
	server, alreadyRunning, err := TryStart(nil)
	if err != nil {
		t.Fatal(err)
	}
	if alreadyRunning {
		t.Fatal("fresh endpoint reported an existing server")
	}
	t.Cleanup(func() { _ = server.Close() })

	if err := os.Remove(server.address); err != nil {
		t.Fatal(err)
	}
	replacement, err := net.ListenUnix(server.network, &net.UnixAddr{Name: server.address, Net: server.network})
	if err != nil {
		t.Fatal(err)
	}
	replacement.SetUnlinkOnClose(false)
	t.Cleanup(func() {
		_ = replacement.Close()
		_ = os.Remove(server.address)
	})
	replacementInfo, err := os.Lstat(server.address)
	if err != nil {
		t.Fatal(err)
	}

	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	currentInfo, err := os.Lstat(server.address)
	if err != nil {
		t.Fatalf("replacement socket was unlinked: %v", err)
	}
	if !os.SameFile(currentInfo, replacementInfo) {
		t.Fatal("socket path no longer refers to the replacement inode")
	}
}

func TestUnixEndpointLeaseIsInterprocessAndCrashRecoverable(t *testing.T) {
	root := shortStableDataRoot(t)
	setStableDataRoot(t, root)
	_, address, err := endpoint()
	if err != nil {
		t.Fatal(err)
	}

	lease, busy, err := tryAcquireUnixEndpointLease(address)
	if err != nil {
		t.Fatal(err)
	}
	if busy || lease == nil {
		t.Fatal("parent process did not acquire endpoint lease")
	}
	if output, err := runEndpointLeaseHelper("expect-busy", address); err != nil {
		_ = lease.release("")
		t.Fatalf("cross-process busy check failed: %v\n%s", err, output)
	}
	if err := lease.release(""); err != nil {
		t.Fatal(err)
	}

	if output, err := runEndpointLeaseHelper("crash-owner", address); err != nil {
		t.Fatalf("crash-owner helper failed: %v\n%s", err, output)
	}
	recovered, busy, err := tryAcquireUnixEndpointLease(address)
	if err != nil {
		t.Fatal(err)
	}
	if busy || recovered == nil {
		t.Fatal("OS did not release crashed process endpoint lease")
	}
	if err := recovered.release(""); err != nil {
		t.Fatal(err)
	}
}

func TestUnixEndpointLeaseSubprocessHelper(t *testing.T) {
	mode := os.Getenv("IMAGE_STUDIO_PROMPTIPC_LEASE_HELPER")
	if mode == "" {
		t.Skip("subprocess helper")
	}
	address := os.Getenv("IMAGE_STUDIO_PROMPTIPC_LEASE_ADDRESS")
	lease, busy, err := tryAcquireUnixEndpointLease(address)
	if err != nil {
		t.Fatal(err)
	}
	switch mode {
	case "expect-busy":
		if !busy || lease != nil {
			t.Fatal("subprocess unexpectedly acquired parent endpoint lease")
		}
	case "crash-owner":
		if busy || lease == nil {
			t.Fatal("subprocess did not acquire endpoint lease")
		}
		// Deliberately bypass cleanup to model abrupt process termination. flock
		// must be released by the kernel while the persistent lock file remains.
		os.Exit(0)
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
}

func setStableDataRoot(t *testing.T, root string) {
	t.Helper()
	original := gioCompat.StableDataRootForTest()
	gioCompat.SetStableDataRootForTest(func() (string, error) { return root, nil })
	t.Cleanup(func() { gioCompat.SetStableDataRootForTest(original) })
}

func longStableDataRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), strings.Repeat("long-path-", 20))
	address := filepath.Join(root, "control", "prompt-import.sock")
	if len(address) <= maxUnixSocketPathBytes {
		t.Fatalf("test stable data root only produces a %d-byte socket path", len(address))
	}
	return root
}

func shortStableDataRoot(t *testing.T) string {
	t.Helper()
	root, err := os.MkdirTemp("/tmp", "isg-promptipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func waitForConnectionCount(t *testing.T, server *Server, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		server.stateMu.Lock()
		got := len(server.connections)
		server.stateMu.Unlock()
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("connection count = %d; want %d", got, want)
		}
		time.Sleep(time.Millisecond)
	}
}

func runEndpointLeaseHelper(mode, address string) ([]byte, error) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestUnixEndpointLeaseSubprocessHelper$")
	cmd.Env = append(os.Environ(),
		"IMAGE_STUDIO_PROMPTIPC_LEASE_HELPER="+mode,
		"IMAGE_STUDIO_PROMPTIPC_LEASE_ADDRESS="+address,
	)
	return cmd.CombinedOutput()
}

func cleanupFallbackEndpoint(t *testing.T, address string) {
	t.Helper()
	t.Cleanup(func() {
		_ = os.Remove(address)
		_ = os.Remove(address + ".lock")
		_ = os.Remove(filepath.Dir(address))
	})
}

func pathWithin(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
