//go:build (linux && !android) || (darwin && !ios)

package promptipc

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	gioCompat "image-studio/gio-client/internal/compat"
)

// Darwin has the stricter sockaddr_un limit: 103 path bytes plus a trailing
// NUL. Using that limit on Linux keeps the endpoint deterministic across both
// supported Unix desktop platforms.
const maxUnixSocketPathBytes = 103

func endpoint() (string, string, error) {
	root, err := gioCompat.StableDataRoot()
	if err != nil {
		return "", "", err
	}
	controlDir := filepath.Join(root, "control")
	address := filepath.Join(controlDir, "prompt-import.sock")
	if len(address) > maxUnixSocketPathBytes {
		address, err = shortUnixSocketAddress(root)
		if err != nil {
			return "", "", err
		}
		return "unix", address, nil
	}
	if err := os.MkdirAll(controlDir, 0o700); err != nil {
		return "", "", err
	}
	return "unix", address, nil
}

func shortUnixSocketAddress(root string) (string, error) {
	canonicalRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve prompt IPC data root: %w", err)
	}

	hash := sha256.New()
	_, _ = hash.Write([]byte(filepath.Clean(canonicalRoot)))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(strconv.Itoa(os.Geteuid())))
	key := base64.RawURLEncoding.EncodeToString(hash.Sum(nil)[:16])
	tempRoot, err := filepath.Abs(os.TempDir())
	if err != nil {
		return "", fmt.Errorf("resolve prompt IPC temporary directory: %w", err)
	}
	dir := filepath.Join(tempRoot, ".isg-"+key)
	address := filepath.Join(dir, "p.sock")
	if len(address) > maxUnixSocketPathBytes {
		return "", fmt.Errorf("prompt IPC fallback socket path is %d bytes; Unix limit is %d: %q", len(address), maxUnixSocketPathBytes, address)
	}
	if err := ensurePrivateSocketDir(dir); err != nil {
		return "", fmt.Errorf("prepare prompt IPC fallback directory: %w", err)
	}
	return address, nil
}

func ensurePrivateSocketDir(dir string) error {
	err := os.Mkdir(dir, 0o700)
	if err == nil {
		// Mkdir applies the process umask; make the private directory usable even
		// under an unusually restrictive one.
		return os.Chmod(dir, 0o700)
	}
	if !errors.Is(err, fs.ErrExist) {
		return err
	}

	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%q is not a private directory", dir)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return fmt.Errorf("%q is not owned by the current user", dir)
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(dir, 0o700); err != nil {
			return err
		}
	}
	return nil
}
