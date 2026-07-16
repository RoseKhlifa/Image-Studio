package ui

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func chooseImageFiles() ([]string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(
			"powershell",
			"-NoProfile",
			"-Command",
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$dlg = New-Object System.Windows.Forms.OpenFileDialog; `+
				`$dlg.Multiselect = $true; `+
				`$dlg.Filter = "Images|*.png;*.jpg;*.jpeg;*.webp"; `+
				`if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8; $dlg.FileNames -join "`+"`n"+`" }`,
		)
		out, err := cmd.Output()
		if err != nil {
			return nil, err
		}
		return parseDialogPaths(string(out)), nil
	case "darwin":
		out, err := exec.Command(
			"osascript",
			"-e",
			`set selectedFiles to choose file with prompt "选择图片" of type {"public.image"} with multiple selections allowed
set selectedPaths to ""
repeat with selectedFile in selectedFiles
	set selectedPaths to selectedPaths & POSIX path of selectedFile & linefeed
end repeat
return selectedPaths`,
		).Output()
		if err != nil {
			return nil, err
		}
		return parseDialogPaths(string(out)), nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			out, err := exec.Command(path, "--file-selection", "--multiple", "--separator=\n", "--file-filter=Images | *.png *.jpg *.jpeg *.webp").Output()
			if err != nil {
				return nil, err
			}
			return parseDialogPaths(string(out)), nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(path, "--getopenfilename", ".", "Images (*.png *.jpg *.jpeg *.webp)", "--multiple", "--separate-output").Output()
			if err != nil {
				return nil, err
			}
			return parseDialogPaths(string(out)), nil
		}
		return nil, fmt.Errorf("当前系统没有可用的文件选择器")
	}
}

func chooseDirectory() (string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(
			"powershell",
			"-NoProfile",
			"-Command",
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$dlg = New-Object System.Windows.Forms.FolderBrowserDialog; `+
				`if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8; $dlg.SelectedPath }`,
		)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	case "darwin":
		out, err := exec.Command(
			"osascript",
			"-e",
			`POSIX path of (choose folder with prompt "选择目录")`,
		).Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			out, err := exec.Command(path, "--file-selection", "--directory").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(path, "--getexistingdirectory", ".").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		return "", fmt.Errorf("当前系统没有可用的目录选择器")
	}
}

func chooseJSONFile() (string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(
			"powershell",
			"-NoProfile",
			"-Command",
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$dlg = New-Object System.Windows.Forms.OpenFileDialog; `+
				`$dlg.Multiselect = $false; `+
				`$dlg.Filter = "JSON|*.json"; `+
				`if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8; $dlg.FileName }`,
		)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	case "darwin":
		out, err := exec.Command(
			"osascript",
			"-e",
			`POSIX path of (choose file of type {"public.json"})`,
		).Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			out, err := exec.Command(path, "--file-selection", "--file-filter=JSON | *.json").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(path, "--getopenfilename", ".", "JSON (*.json)").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		return "", fmt.Errorf("当前系统没有可用的 JSON 文件选择器")
	}
}

func chooseAudioFile() (string, error) {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(
			"powershell",
			"-NoProfile",
			"-Command",
			`Add-Type -AssemblyName System.Windows.Forms; `+
				`$dlg = New-Object System.Windows.Forms.OpenFileDialog; `+
				`$dlg.Multiselect = $false; `+
				`$dlg.Filter = "Audio|*.mp3;*.wav;*.ogg;*.m4a;*.aac;*.webm"; `+
				`if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8; $dlg.FileName }`,
		)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	case "darwin":
		out, err := exec.Command(
			"osascript",
			"-e",
			`POSIX path of (choose file)`,
		).Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			out, err := exec.Command(path, "--file-selection", "--file-filter=Audio | *.mp3 *.wav *.ogg *.m4a *.aac *.webm").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(path, "--getopenfilename", ".", "Audio (*.mp3 *.wav *.ogg *.m4a *.aac *.webm)").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		return "", fmt.Errorf("当前系统没有可用的音频文件选择器")
	}
}

func chooseSaveJSONFile(suggestedName string) (string, error) {
	suggestedName = strings.TrimSpace(suggestedName)
	if suggestedName == "" {
		suggestedName = "image-studio-history.json"
	}
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command(
			"powershell",
			"-NoProfile",
			"-Command",
			fmt.Sprintf(
				`Add-Type -AssemblyName System.Windows.Forms; `+
					`$dlg = New-Object System.Windows.Forms.SaveFileDialog; `+
					`$dlg.Filter = "JSON|*.json"; `+
					`$dlg.FileName = %q; `+
					`if ($dlg.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK) { [Console]::OutputEncoding = [System.Text.UTF8Encoding]::UTF8; $dlg.FileName }`,
				suggestedName,
			),
		)
		out, err := cmd.Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	case "darwin":
		out, err := exec.Command(
			"osascript",
			"-e",
			fmt.Sprintf(`POSIX path of (choose file name with prompt "导出历史 JSON" default name %q)`, suggestedName),
		).Output()
		if err != nil {
			return "", err
		}
		paths := parseDialogPaths(string(out))
		if len(paths) == 0 {
			return "", nil
		}
		return paths[0], nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			out, err := exec.Command(path, "--file-selection", "--save", "--confirm-overwrite", "--filename="+suggestedName, "--file-filter=JSON | *.json").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			out, err := exec.Command(path, "--getsavefilename", filepath.Join(".", suggestedName), "JSON (*.json)").Output()
			if err != nil {
				return "", err
			}
			paths := parseDialogPaths(string(out))
			if len(paths) == 0 {
				return "", nil
			}
			return paths[0], nil
		}
		return "", fmt.Errorf("当前系统没有可用的 JSON 文件保存器")
	}
}

func parseDialogPaths(raw string) []string {
	lines := strings.FieldsFunc(raw, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	out := make([]string, 0, len(lines))
	seen := map[string]struct{}{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, `"'`)
		if line == "" {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func openPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("路径为空")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

func openExternalURL(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("URL 无效: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅支持 http/https URL")
	}
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", parsed.String()).Start()
	case "darwin":
		return exec.Command("open", parsed.String()).Start()
	default:
		return exec.Command("xdg-open", parsed.String()).Start()
	}
}

func systemThemeMode() string {
	if env := strings.ToLower(strings.TrimSpace(os.Getenv("GTK_THEME"))); strings.Contains(env, "dark") {
		return "dark"
	}
	if env := strings.TrimSpace(os.Getenv("COLORFGBG")); env != "" {
		parts := strings.Split(env, ";")
		if len(parts) > 0 {
			last := strings.TrimSpace(parts[len(parts)-1])
			if last == "0" || last == "1" || last == "2" || last == "3" || last == "4" || last == "5" || last == "6" || last == "8" {
				return "dark"
			}
		}
	}
	switch runtime.GOOS {
	case "darwin":
		out, err := exec.Command("defaults", "read", "-g", "AppleInterfaceStyle").Output()
		if err == nil && strings.Contains(strings.ToLower(string(out)), "dark") {
			return "dark"
		}
	case "windows":
		out, err := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "/v", "AppsUseLightTheme").Output()
		if err == nil {
			lower := strings.ToLower(string(out))
			if strings.Contains(lower, "0x0") {
				return "dark"
			}
			if strings.Contains(lower, "0x1") {
				return "light"
			}
		}
	default:
		if path, err := exec.LookPath("gsettings"); err == nil {
			if out, err := exec.Command(path, "get", "org.gnome.desktop.interface", "color-scheme").Output(); err == nil {
				if strings.Contains(strings.ToLower(string(out)), "dark") {
					return "dark"
				}
				if strings.Contains(strings.ToLower(string(out)), "light") {
					return "light"
				}
			}
			if out, err := exec.Command(path, "get", "org.gnome.desktop.interface", "gtk-theme").Output(); err == nil {
				if strings.Contains(strings.ToLower(string(out)), "dark") {
					return "dark"
				}
			}
		}
	}
	return "light"
}
