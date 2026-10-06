package backend

import (
	"encoding/base64"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var supportedBatchInputExtensions = map[string]struct{}{
	".png":  {},
	".jpg":  {},
	".jpeg": {},
	".webp": {},
}

func (s *Service) BeginNativeFileDrag(path string) error {
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return err
	}
	s.nativeDragMu.Lock()
	if s.nativeDragActive {
		s.nativeDragMu.Unlock()
		return errors.New("已有文件正在拖动，请先松开鼠标或按 Esc 取消")
	}
	s.nativeDragActive = true
	s.nativeDragMu.Unlock()
	defer func() {
		s.nativeDragMu.Lock()
		s.nativeDragActive = false
		s.nativeDragMu.Unlock()
	}()
	if s.ctx != nil {
		runtime.EventsEmit(s.ctx, "native-file-drag", allowed)
	}
	return beginNativeFileDrag(allowed)
}

// OpenImageDialog shows a file picker filtered to supported image types and
// returns the selected absolute path, size, and a managed AVIF preview URL when
// thumbnail generation succeeds.
const maxDialogReadBytes int64 = 50 * 1024 * 1024

func (s *Service) openSingleImageDialog(title string) (string, os.FileInfo, error) {
	path, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: title,
		Filters: []runtime.FileFilter{
			{DisplayName: "支持的图片 (*.png;*.jpg;*.jpeg;*.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", nil, err
	}
	if path == "" {
		return "", nil, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", nil, err
	}
	return path, info, nil
}

func (s *Service) OpenImageDialog() (SelectFileResponse, error) {
	path, info, err := s.openSingleImageDialog("选择源图片")
	if err != nil || path == "" {
		return SelectFileResponse{}, err
	}
	resp := SelectFileResponse{Path: path, Size: info.Size()}
	if info.Size() > 0 && info.Size() <= maxDialogReadBytes {
		if preview, previewErr := s.registerImportedPreview(path); previewErr == nil {
			resp.ImageID = preview.ID
			resp.PreviewURL = preview.PreviewURL
			resp.PreviewWidth = preview.PreviewWidth
			resp.PreviewHeight = preview.PreviewHeight
		}
	}
	return resp, nil
}

func (s *Service) OpenMaskImageDialog() (SelectFileResponse, error) {
	path, info, err := s.openSingleImageDialog("选择蒙版图片")
	if err != nil || path == "" {
		return SelectFileResponse{}, err
	}
	if info.Size() <= 0 {
		return SelectFileResponse{}, errors.New("蒙版图片为空")
	}
	if info.Size() > maxDialogReadBytes {
		return SelectFileResponse{}, fmt.Errorf("蒙版图片过大，不能超过 %dMB", maxDialogReadBytes/(1024*1024))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return SelectFileResponse{}, err
	}
	return SelectFileResponse{
		Path:     path,
		Size:     info.Size(),
		ImageB64: base64.StdEncoding.EncodeToString(data),
	}, nil
}

func (s *Service) OpenImagesDialog() (SelectFilesResponse, error) {
	paths, err := runtime.OpenMultipleFilesDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择批处理源图片",
		Filters: []runtime.FileFilter{
			{DisplayName: "支持的图片 (*.png;*.jpg;*.jpeg;*.webp)", Pattern: "*.png;*.jpg;*.jpeg;*.webp"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return SelectFilesResponse{}, err
	}
	if len(paths) == 0 {
		return SelectFilesResponse{}, nil
	}
	files := make([]BatchInputImage, 0, len(paths))
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		info, statErr := os.Stat(path)
		if statErr != nil || info.IsDir() {
			continue
		}
		item := BatchInputImage{
			Path: path,
			Name: filepath.Base(path),
			Size: info.Size(),
		}
		if cfg, cfgErr := imageConfig(path); cfgErr == nil {
			item.Width = cfg.Width
			item.Height = cfg.Height
		}
		if info.Size() > 0 && info.Size() <= maxDialogReadBytes {
			if preview, previewErr := s.registerImportedPreview(path); previewErr == nil {
				item.PreviewURL = preview.PreviewURL
				item.PreviewWidth = preview.PreviewWidth
				item.PreviewHeight = preview.PreviewHeight
			}
		}
		files = append(files, item)
	}
	return SelectFilesResponse{Files: files}, nil
}

func (s *Service) ChooseBatchInputDir() (BatchInputDirectory, error) {
	if s.ctx == nil {
		return BatchInputDirectory{}, errors.New("服务未启动")
	}
	chosen, err := runtime.OpenDirectoryDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择批处理输入目录",
	})
	if err != nil {
		return BatchInputDirectory{}, err
	}
	if chosen == "" {
		return BatchInputDirectory{}, nil
	}
	return s.ListBatchInputImages(chosen)
}

func (s *Service) ListBatchInputImages(directory string) (BatchInputDirectory, error) {
	clean := strings.TrimSpace(directory)
	if clean == "" {
		return BatchInputDirectory{}, errors.New("目标目录不能为空")
	}
	root, err := filepath.Abs(clean)
	if err != nil {
		return BatchInputDirectory{}, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return BatchInputDirectory{}, fmt.Errorf("读取目录失败: %w", err)
	}
	if !info.IsDir() {
		return BatchInputDirectory{}, fmt.Errorf("不是目录: %s", root)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return BatchInputDirectory{}, fmt.Errorf("读取目录失败: %w", err)
	}
	images := make([]BatchInputImage, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if _, ok := supportedBatchInputExtensions[ext]; !ok {
			continue
		}
		path := filepath.Join(root, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		item := BatchInputImage{
			Path: path,
			Name: entry.Name(),
			Size: info.Size(),
		}
		if cfg, cfgErr := imageConfig(path); cfgErr == nil {
			item.Width = cfg.Width
			item.Height = cfg.Height
		}
		if info.Size() > 0 && info.Size() <= maxDialogReadBytes {
			if preview, previewErr := s.registerImportedPreview(path); previewErr == nil {
				item.PreviewURL = preview.PreviewURL
				item.PreviewWidth = preview.PreviewWidth
				item.PreviewHeight = preview.PreviewHeight
			}
		}
		images = append(images, item)
	}
	return BatchInputDirectory{
		Directory: root,
		Images:    images,
	}, nil
}

// SaveImageAs prompts the user for a destination and writes the base64 PNG to disk.
func (s *Service) SaveImageAs(imageB64, suggestedName string) (string, error) {
	if suggestedName == "" {
		suggestedName = fmt.Sprintf("image-%d.png", time.Now().Unix())
	}
	dst, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: suggestedName,
		Filters: []runtime.FileFilter{
			{DisplayName: "PNG 图片 (*.png)", Pattern: "*.png"},
		},
	})
	if err != nil || dst == "" {
		return "", err
	}
	return writeBase64PNG(imageB64, dst)
}

// SaveImagePathAs copies an existing managed image to a user-selected path.
// Generated results use this path-first route so the frontend does not have to
// read the full image into JS memory just to save a copy.
func (s *Service) SaveImagePathAs(path, suggestedName string) (string, error) {
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return "", err
	}
	if suggestedName == "" {
		suggestedName = filepath.Base(allowed)
	}
	dst, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "保存图片",
		DefaultFilename: suggestedName,
		Filters: []runtime.FileFilter{
			{DisplayName: "图片文件 (*.png;*.jpg;*.jpeg;*.webp;*.avif)", Pattern: "*.png;*.jpg;*.jpeg;*.webp;*.avif"},
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil || dst == "" {
		return "", err
	}
	data, err := os.ReadFile(allowed)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, secureFileMode); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(dst)
	return abs, nil
}

// SaveImageToDir writes a base64 image into the specified directory without
// opening a save dialog. Used by loop-generation auto-save.
func (s *Service) SaveImageToDir(imageB64, directory, suggestedName string) (string, error) {
	if strings.TrimSpace(imageB64) == "" {
		return "", errors.New("图片数据为空")
	}
	root, err := ensureTargetDirectory(directory)
	if err != nil {
		return "", err
	}
	name := ensureTargetFileName(suggestedName, fmt.Sprintf("image-%d.png", time.Now().Unix()))
	dst, err := uniqueTargetPath(root, name)
	if err != nil {
		return "", err
	}
	return writeBase64PNG(imageB64, dst)
}

// SaveImagePathToDir copies an existing managed image into the specified
// directory without prompting the user.
func (s *Service) SaveImagePathToDir(path, directory, suggestedName string) (string, error) {
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return "", err
	}
	root, err := ensureTargetDirectory(directory)
	if err != nil {
		return "", err
	}
	name := ensureTargetFileName(suggestedName, filepath.Base(allowed))
	dst, err := uniqueTargetPath(root, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(allowed)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dst, data, secureFileMode); err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(dst)
	return abs, nil
}

// GetOutputDir returns the directory where generated images and raw response
// dumps are written —— 用户自定义优先,空时回退到默认。
func (s *Service) GetOutputDir() (string, error) {
	return s.resolvedOutputDir()
}

func ensureTargetDirectory(directory string) (string, error) {
	clean := strings.TrimSpace(directory)
	if clean == "" {
		return "", errors.New("目标目录不能为空")
	}
	abs, err := filepath.Abs(clean)
	if err != nil {
		return "", fmt.Errorf("路径无效:%w", err)
	}
	if err := os.MkdirAll(abs, secureDirMode); err != nil {
		return "", fmt.Errorf("无法创建目标目录 %s: %w", abs, err)
	}
	return abs, nil
}

func ensureTargetFileName(suggestedName, fallback string) string {
	name := strings.TrimSpace(suggestedName)
	if name == "" {
		name = fallback
	}
	name = filepath.Base(name)
	if name == "." || name == string(filepath.Separator) || name == "" {
		return fallback
	}
	return name
}

func uniqueTargetPath(dir, fileName string) (string, error) {
	ext := filepath.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	candidate := filepath.Join(dir, fileName)
	if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
		return candidate, nil
	} else if err != nil {
		return "", err
	}
	for i := 2; i < 10_000; i++ {
		next := filepath.Join(dir, fmt.Sprintf("%s-%d%s", base, i, ext))
		if _, err := os.Stat(next); errors.Is(err, os.ErrNotExist) {
			return next, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("目标目录中同名文件过多:%s", fileName)
}

func uniquePrefixedTargetPath(dir, sourceName, prefix string) (string, error) {
	base := ensureTargetFileName(sourceName, "image.png")
	trimmedPrefix := strings.TrimSpace(prefix)
	if trimmedPrefix == "" {
		return uniqueTargetPath(dir, base)
	}
	return uniqueTargetPath(dir, trimmedPrefix+base)
}

// OpenOutputDir reveals the output directory in the OS file explorer.
// 兜底:用户在第一次生成前就点「打开输出目录」,默认路径还不存在 ——
// `open` / `xdg-open` / `explorer` 拿到不存在的路径都会失败(macOS / Linux
// 表现为完全打不开),所以这里把根目录 + images/log 子目录预建好。
func (s *Service) OpenOutputDir() error {
	dir, err := s.resolvedOutputDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(imagesSubdir(dir), secureDirMode); err != nil {
		return err
	}
	if err := os.MkdirAll(thumbsSubdir(dir), secureDirMode); err != nil {
		return err
	}
	if err := os.MkdirAll(previewsSubdir(dir), secureDirMode); err != nil {
		return err
	}
	if err := os.MkdirAll(logSubdir(dir), secureDirMode); err != nil {
		return err
	}
	return openInExplorer(dir)
}

// OpenExternalURL launches a URL in the default browser. Used for GitHub /
// license / Issues links in the About dialog and Footer.
func (s *Service) OpenExternalURL(rawURL string) error {
	if rawURL == "" {
		return errors.New("url is empty")
	}
	parsed, err := neturl.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return errors.New("invalid external url")
	}
	switch parsed.Scheme {
	case "http", "https":
	default:
		return errors.New("unsupported external url scheme")
	}
	return openInExplorer(rawURL)
}

// OpenFile 在系统默认应用里打开一个本地文件 —— 前端「查看日志」按钮调用,
// 让用户拿系统记事本 / TextEdit / xdg-open 关联程序读 sse-response-*.txt
// 这类原始上游响应。文件不存在就返回错误。
func (s *Service) OpenFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is empty")
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("文件不存在或无法访问:%w", err)
	}
	return openInExplorer(path)
}

// ReadImageAsBase64 loads an image file from disk and returns its bytes as
// standard base64. Used by the frontend to refresh the canvas after a
// rotate/flip/crop operation produced a new file in imports/.
func (s *Service) ReadImageAsBase64(path string) (string, error) {
	allowed, err := s.ensureManagedReadablePath(path, managedImageFile)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(allowed)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// ReadTextFile returns a file's contents as a string. Used to display the raw
// SSE response in the "查看 raw" modal.
func (s *Service) ReadTextFile(path string) (string, error) {
	allowed, err := s.ensureManagedReadablePath(path, managedRawLogFile)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(allowed)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ExportHistoryToFile writes a JSON dump (provided by the frontend) to a
// user-chosen path. Powers the "导出历史" action in settings.
func (s *Service) ExportHistoryToFile(jsonContent string) (string, error) {
	dst, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "导出历史记录",
		DefaultFilename: fmt.Sprintf("image-studio-history-%s.json", time.Now().Format("20060102-150405")),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || dst == "" {
		return "", err
	}
	if err := os.WriteFile(dst, []byte(jsonContent), secureFileMode); err != nil {
		return "", err
	}
	return dst, nil
}

// ExportUpstreamConfigToFile writes an upstream-config JSON dump to a
// user-selected path so profile metadata and API keys can survive upgrades or
// manual reinstalls.
func (s *Service) ExportUpstreamConfigToFile(jsonContent string) (string, error) {
	dst, err := runtime.SaveFileDialog(s.ctx, runtime.SaveDialogOptions{
		Title:           "导出上游配置",
		DefaultFilename: fmt.Sprintf("image-studio-upstream-config-%s.json", time.Now().Format("20060102-150405")),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || dst == "" {
		return "", err
	}
	if err := os.WriteFile(dst, []byte(jsonContent), secureFileMode); err != nil {
		return "", err
	}
	return dst, nil
}

// ImportHistoryFromFile opens a file picker and returns the JSON content as a
// string. The frontend then parses and merges the entries into IndexedDB.
func (s *Service) ImportHistoryFromFile() (string, error) {
	src, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择历史 JSON 文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || src == "" {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ImportUpstreamConfigFromFile opens a JSON file picker and returns the file
// contents so the frontend can merge the imported upstream profiles and
// restore any bundled API keys into the system credential store.
func (s *Service) ImportUpstreamConfigFromFile() (string, error) {
	src, err := runtime.OpenFileDialog(s.ctx, runtime.OpenDialogOptions{
		Title: "选择上游配置 JSON 文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil || src == "" {
		return "", err
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
