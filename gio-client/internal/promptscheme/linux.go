//go:build linux && !android

package promptscheme

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const linuxDesktopFileName = "image-studio-gio.desktop"

func RegisterExecutable(executable string) error {
	executable = strings.TrimSpace(executable)
	if executable == "" {
		return fmt.Errorf("executable path is empty")
	}
	desktopPath, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(desktopPath), 0o755); err != nil {
		return err
	}
	content := fmt.Sprintf(`[Desktop Entry]
Name=Image Studio Gio
Exec="%s" %%u
Type=Application
NoDisplay=true
Terminal=false
MimeType=x-scheme-handler/%s;
Categories=Graphics;
`, executable, Scheme)
	if err := os.WriteFile(desktopPath, []byte(content), 0o644); err != nil {
		return err
	}
	if path, lookErr := exec.LookPath("xdg-mime"); lookErr == nil {
		if output, runErr := exec.Command(path, "default", linuxDesktopFileName, "x-scheme-handler/"+Scheme).CombinedOutput(); runErr != nil {
			return fmt.Errorf("xdg-mime default failed: %s", strings.TrimSpace(string(output)))
		}
	}
	return nil
}

func UnregisterExecutable(_ string) error {
	desktopPath, err := desktopFilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(desktopPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func StatusForExecutable(executable string) (Status, error) {
	desktopPath, err := desktopFilePath()
	if err != nil {
		return Status{}, err
	}
	data, readErr := os.ReadFile(desktopPath)
	if readErr != nil {
		return Status{}, nil
	}
	handler := strings.TrimSpace(string(data))
	registered := strings.Contains(handler, strings.TrimSpace(executable))
	if path, lookErr := exec.LookPath("xdg-mime"); lookErr == nil {
		if output, runErr := exec.Command(path, "query", "default", "x-scheme-handler/"+Scheme).Output(); runErr == nil {
			desktopName := strings.TrimSpace(string(output))
			if desktopName != "" {
				registered = registered && desktopName == linuxDesktopFileName
				handler = desktopName
			}
		}
	}
	return Status{
		Registered: registered,
		Handler:    handler,
		Detail:     desktopPath,
	}, nil
}

func desktopFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "applications", linuxDesktopFileName), nil
}
