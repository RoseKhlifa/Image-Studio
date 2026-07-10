package backend

import "strings"

// --- UI-facing types -------------------------------------------------------

// GenerateOptions is the request shape sent by the frontend.
// Fields mirror client.Options but with friendlier names for TS.
type GenerateOptions struct {
	APIKey string `json:"apiKey"`
	Mode   string `json:"mode"` // "generate" | "edit"
	// RequestedJobID allows the frontend to pre-bind event listeners before
	// dispatching the request, avoiding a race where a very fast result event
	// arrives before the UI has attached `result:<jobId>` handlers.
	RequestedJobID string `json:"requestedJobId"`
	Prompt         string `json:"prompt"`
	Size           string `json:"size"`
	Quality        string `json:"quality"`
	// OutputFormat:"png" | "jpeg" | "webp"。空时回退到 client.OutputFormat 默认("png")。
	OutputFormat string `json:"outputFormat"`

	// Multi-reference: zero or more source images for edit mode. Each is a
	// path on disk (frontend writes imports / generated PNGs to disk so we
	// can avoid pushing large base64 across the JSON bridge).
	ImagePaths []string `json:"imagePaths"`

	// Deprecated single-image path, kept for backward compat with older
	// frontend builds. Folded into ImagePaths when present.
	ImagePath string `json:"imagePath"`

	MaskB64            string `json:"maskB64"`        // optional imported or painted edit mask
	Seed               int64  `json:"seed"`           // 0 = random
	NegativePrompt     string `json:"negativePrompt"` // optional
	Background         string `json:"background"`     // "auto" | "opaque" | "transparent"
	OutputCompression  int    `json:"outputCompression"`
	InputFidelity      string `json:"inputFidelity"` // "auto" | "low" | "high"
	ImageStyle         string `json:"imageStyle"`    // "default" | "vivid" | "natural"
	Moderation         string `json:"moderation"`    // "low" | "auto"
	UserIdentifier     string `json:"userIdentifier"`
	BaseURL            string `json:"baseURL"`      // overrides the default upstream URL
	TextModelID        string `json:"textModelID"`  // overrides the default text model
	ImageModelID       string `json:"imageModelID"` // overrides the default image model
	ReasoningEffort    string `json:"reasoningEffort"`
	APIMode            string `json:"apiMode"` // "responses" (default) | "images"
	ResponsesTransport string `json:"responsesTransport,omitempty"`
	RequestPolicy      string `json:"requestPolicy"` // "openai" (default) | "compat"
	// ImagesNewAPICompat 开启后仅影响 Images API 请求:
	// 强制使用 b64_json,并关闭 stream/partial_images,用于兼容部分 NewAPI 中转。
	ImagesNewAPICompat bool   `json:"imagesNewAPICompat,omitempty"`
	ProxyMode          string `json:"proxyMode"` // "none" | "system" (default) | "custom"
	ProxyURL           string `json:"proxyURL"`  // http(s) proxy URL when ProxyMode == "custom"
	// NoPromptRevision is kept for backward compatibility; Responses API
	// requests now always ask the text model to keep the prompt verbatim.
	NoPromptRevision bool `json:"noPromptRevision"`
	// ConcurrencyLimit is enforced per APIMode. 0 means unlimited.
	ConcurrencyLimit int `json:"concurrencyLimit"`
	// PartialImages controls Responses API stream preview count. 0 keeps the app default.
	PartialImages int `json:"partialImages"`
	// DisablePreview forces partial_images = 0 for this request, regardless of
	// the default partial-image setting.
	DisablePreview bool `json:"disablePreview,omitempty"`
	// AutoRetryEnabled controls whether retryable upstream failures should be
	// re-issued automatically. Default true.
	AutoRetryEnabled bool `json:"autoRetryEnabled"`
	// AutoRetryCount controls how many extra retry attempts are made after the
	// first request. 0 falls back to the app default.
	AutoRetryCount int `json:"autoRetryCount"`
	// FallbackProfile is an optional backup upstream used only after the main
	// upstream exhausted its own automatic retries and still failed.
	FallbackProfile *FallbackProfileOptions `json:"fallbackProfile,omitempty"`
}

type FallbackProfileOptions struct {
	BaseURL            string `json:"baseURL"`
	APIKey             string `json:"apiKey"`
	TextModelID        string `json:"textModelID"`
	ImageModelID       string `json:"imageModelID"`
	ReasoningEffort    string `json:"reasoningEffort"`
	APIMode            string `json:"apiMode"`
	ResponsesTransport string `json:"responsesTransport,omitempty"`
	RequestPolicy      string `json:"requestPolicy"`
	ImagesNewAPICompat bool   `json:"imagesNewAPICompat,omitempty"`
}

// PromptOptimizeOptions is the request shape for one-click prompt revision.
type PromptOptimizeOptions struct {
	APIKey      string   `json:"apiKey"`
	Prompt      string   `json:"prompt"`
	Mode        string   `json:"mode"`
	BaseURL     string   `json:"baseURL"`
	TextModelID string   `json:"textModelID"`
	ProxyMode   string   `json:"proxyMode"`
	ProxyURL    string   `json:"proxyURL"`
	ImagePaths  []string `json:"imagePaths"`
	ImagePath   string   `json:"imagePath"`
}

// ProbeUpstreamOptions is used by the UI's connection-test button. Validation
// and the actual /v1/models request are intentionally host-side so browser and
// WebView quirks do not decide whether a channel is alive.
type ProbeUpstreamOptions struct {
	APIKey             string `json:"apiKey"`
	BaseURL            string `json:"baseURL"`
	ProxyMode          string `json:"proxyMode"`
	ProxyURL           string `json:"proxyURL"`
	APIMode            string `json:"apiMode,omitempty"`
	ResponsesTransport string `json:"responsesTransport,omitempty"`
}

type ProbeUpstreamResult struct {
	ModelCount              int                       `json:"modelCount"`
	Models                  []UpstreamModelDescriptor `json:"models,omitempty"`
	ResponsesTransport      string                    `json:"responsesTransport,omitempty"`
	ResponsesTransportOK    bool                      `json:"responsesTransportOK,omitempty"`
	ResponsesTransportError string                    `json:"responsesTransportError,omitempty"`
}

type UpstreamModelDescriptor struct {
	ID          string `json:"id"`
	Object      string `json:"object,omitempty"`
	OwnedBy     string `json:"ownedBy,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
}

// CodexAPIConfig is loaded from the local Codex desktop config/auth files so
// the frontend can import the same upstream into Image Studio.
type CodexAPIConfig struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"baseURL"`
	APIKey   string `json:"apiKey"`
	WireAPI  string `json:"wireAPI"`
}

type PromptImportBilingualText struct {
	Zh string `json:"zh,omitempty"`
	En string `json:"en,omitempty"`
}

type PromptImportPayload struct {
	Prompt         PromptImportBilingualText  `json:"prompt"`
	NegativePrompt *PromptImportBilingualText `json:"negative_prompt,omitempty"`
	AspectRatio    string                     `json:"aspect_ratio,omitempty"`
	ResolvedSize   string                     `json:"resolvedSize,omitempty"`
}

type PromptImportActivation struct {
	Tokens       []string `json:"tokens,omitempty"`
	InvalidCount int      `json:"invalidCount,omitempty"`
}

func (o PromptOptimizeOptions) collectPaths() []string {
	paths := make([]string, 0, len(o.ImagePaths)+1)
	for _, p := range o.ImagePaths {
		if strings.TrimSpace(p) != "" {
			paths = append(paths, p)
		}
	}
	if strings.TrimSpace(o.ImagePath) != "" {
		paths = append(paths, o.ImagePath)
	}
	return paths
}

// JobStarted is the response to Generate/Edit.
type JobStarted struct {
	JobID string `json:"jobId"`
}

// ProgressPayload is emitted as `progress:<jobId>` events.
type ProgressPayload struct {
	Stage   string `json:"stage"`
	Elapsed int    `json:"elapsed"`
	Bytes   int64  `json:"bytes"`
}

// ResultPayload is emitted as `result:<jobId>`.
type ResultPayload struct {
	ImageB64      string `json:"imageB64,omitempty"`
	RevisedPrompt string `json:"revisedPrompt"`
	SourceEvent   string `json:"sourceEvent"`
	ImageID       string `json:"imageId,omitempty"`
	SavedPath     string `json:"savedPath"` // absolute path in user config dir
	ThumbPath     string `json:"thumbPath,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	FullURL       string `json:"fullUrl,omitempty"`
	PreviewWidth  int    `json:"previewWidth,omitempty"`
	PreviewHeight int    `json:"previewHeight,omitempty"`
	RawPath       string `json:"rawPath"` // raw SSE dump location
	Mode          string `json:"mode"`
	Prompt        string `json:"prompt"`
}

// PreviewPayload is emitted as `preview:<jobId>` while Responses API streams
// intermediate image-generation previews.
type PreviewPayload struct {
	ImageB64          string `json:"imageB64,omitempty"` // legacy/browser fallback only; Wails preview events use PreviewURL.
	ImageID           string `json:"imageId,omitempty"`
	PreviewURL        string `json:"previewUrl,omitempty"`
	PreviewWidth      int    `json:"previewWidth,omitempty"`
	PreviewHeight     int    `json:"previewHeight,omitempty"`
	RevisedPrompt     string `json:"revisedPrompt,omitempty"`
	PartialImageIndex int    `json:"partialImageIndex"`
	Mode              string `json:"mode"`
	Prompt            string `json:"prompt"`
}

// ErrorPayload is emitted as `error:<jobId>` when a run fails.
// RawPath 指向 sse-response-*.txt / images-response-*.json,前端用「查看日志」
// 按钮调 OpenFile 把它在系统默认应用里打开。请求还没真正发出去就失败时(例如
// 参数校验阶段)RawPath 为空。
type ErrorPayload struct {
	Message string `json:"message"`
	RawPath string `json:"rawPath,omitempty"`
}

// SelectFileResponse is returned by OpenImageDialog. New Wails builds return a
// managed previewUrl; ImageB64 remains only as a legacy/browser fallback when
// preview generation is unavailable. Files over 50MB omit both preview forms.
type SelectFileResponse struct {
	Path          string `json:"path"`
	Size          int64  `json:"size"`
	ImageB64      string `json:"imageB64,omitempty"`
	ImageID       string `json:"imageId,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	PreviewWidth  int    `json:"previewWidth,omitempty"`
	PreviewHeight int    `json:"previewHeight,omitempty"`
}

type BatchInputImage struct {
	Path          string `json:"path"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	PreviewWidth  int    `json:"previewWidth,omitempty"`
	PreviewHeight int    `json:"previewHeight,omitempty"`
}

type BatchInputDirectory struct {
	Directory string            `json:"directory"`
	Images    []BatchInputImage `json:"images"`
}

type SelectFilesResponse struct {
	Files []BatchInputImage `json:"files"`
}

// ImportedImage describes a freshly imported (drag-dropped or pasted) image.
type ImportedImage struct {
	Path          string `json:"path"`
	ImageB64      string `json:"imageB64,omitempty"`
	ImageID       string `json:"imageId,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	PreviewWidth  int    `json:"previewWidth,omitempty"`
	PreviewHeight int    `json:"previewHeight,omitempty"`
}

type MediaAssetRef struct {
	ImageID       string `json:"imageId,omitempty"`
	SavedPath     string `json:"savedPath,omitempty"`
	ThumbPath     string `json:"thumbPath,omitempty"`
	PreviewURL    string `json:"previewUrl,omitempty"`
	FullURL       string `json:"fullUrl,omitempty"`
	PreviewWidth  int    `json:"previewWidth,omitempty"`
	PreviewHeight int    `json:"previewHeight,omitempty"`
}
