package client

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var fakePNG = []byte("\x89PNG\r\n\x1a\nfake")

func mustDecodePayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("payload not valid JSON: %v\n%s", err, raw)
	}
	return v
}

func TestBuildPayloadUsesSizeAndQuality(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:  "生成海报",
		Size:    "1536x1024",
		Quality: "high",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v := mustDecodePayload(t, raw)

	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["size"] != "1536x1024" {
		t.Errorf("size = %v, want 1536x1024", tool["size"])
	}
	if tool["quality"] != "high" {
		t.Errorf("quality = %v, want high", tool["quality"])
	}
	if tool["background"] != "auto" {
		t.Errorf("background = %v, want auto", tool["background"])
	}
	if tool["moderation"] != "low" {
		t.Errorf("moderation = %v, want low", tool["moderation"])
	}
	if _, ok := tool["model"]; ok {
		t.Errorf("Responses image_generation tool unexpectedly contains a second model: %v", tool["model"])
	}
	if tool["action"] != "generate" {
		t.Errorf("action = %v, want generate", tool["action"])
	}
	if v["stream"] != true {
		t.Errorf("stream = %v, want true", v["stream"])
	}
	reasoning := v["reasoning"].(map[string]any)
	if reasoning["effort"] != DefaultReasoningEffort {
		t.Errorf("reasoning.effort = %v, want %s", reasoning["effort"], DefaultReasoningEffort)
	}
	if tool["partial_images"] != float64(DefaultPartialImages) {
		t.Errorf("partial_images = %v, want %d", tool["partial_images"], DefaultPartialImages)
	}

	input := v["input"].([]any)[0].(map[string]any)
	content := input["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("generate-mode content len = %d, want 1", len(content))
	}
	first := content[0].(map[string]any)
	if first["type"] != "input_text" || first["text"] != "生成海报" {
		t.Errorf("input_text = %v", first)
	}
}

func TestBuildPayloadAllowsModerationAuto(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:     "生成海报",
		Moderation: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["moderation"] != "auto" {
		t.Fatalf("moderation = %v, want auto", tool["moderation"])
	}
}

func TestBuildPayloadAllowsCustomReasoningEffort(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:          "生成海报",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	reasoning := v["reasoning"].(map[string]any)
	if reasoning["effort"] != "high" {
		t.Fatalf("reasoning.effort = %v, want high", reasoning["effort"])
	}
}

func TestBuildPayloadIncludesSafetyIdentifier(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:         "生成海报",
		UserIdentifier: "  user-hash-123  ",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	if v["safety_identifier"] != "user-hash-123" {
		t.Fatalf("safety_identifier = %v, want user-hash-123", v["safety_identifier"])
	}
}

func TestBuildPayloadIncludesOutputCompressionForJPEG(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:            "生成海报",
		OutputFormat:      "jpeg",
		OutputCompression: 55,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["output_compression"] != float64(55) {
		t.Fatalf("output_compression = %v, want 55", tool["output_compression"])
	}
}

func TestBuildPayloadIncludesInputFidelityForSupportedEditModels(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:        "生成海报",
		ImageModelID:  "gpt-image-1.5",
		ImageDataURL:  "data:image/png;base64,abc123",
		InputFidelity: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["input_fidelity"] != "high" {
		t.Fatalf("input_fidelity = %v, want high", tool["input_fidelity"])
	}
}

func TestBuildPayloadOmitsModerationForUnsupportedModel(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:       "生成海报",
		ImageModelID: "dall-e-3",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if _, ok := tool["moderation"]; ok {
		t.Fatalf("moderation should be omitted for dall-e-3, got %v", tool["moderation"])
	}
	if _, ok := tool["background"]; ok {
		t.Fatalf("background should be omitted for dall-e-3, got %v", tool["background"])
	}
	if _, ok := tool["output_compression"]; ok {
		t.Fatalf("output_compression should be omitted for dall-e-3, got %v", tool["output_compression"])
	}
	if _, ok := tool["input_fidelity"]; ok {
		t.Fatalf("input_fidelity should be omitted for dall-e-3, got %v", tool["input_fidelity"])
	}
}

func TestBuildPayloadNormalizesPartialImages(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want float64
	}{
		{name: "zero uses default", in: 0, want: float64(DefaultPartialImages)},
		{name: "negative uses default", in: -2, want: float64(DefaultPartialImages)},
		{name: "keeps explicit", in: 2, want: 2},
		{name: "clamps max", in: 9, want: 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := BuildPayload(Options{Prompt: "x", PartialImages: tt.in})
			if err != nil {
				t.Fatal(err)
			}
			v := mustDecodePayload(t, raw)
			tool := v["tools"].([]any)[0].(map[string]any)
			if tool["partial_images"] != tt.want {
				t.Fatalf("partial_images = %v, want %v", tool["partial_images"], tt.want)
			}
		})
	}
}

func TestBuildPayloadDisablePreviewForcesZeroPartialImages(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:         "生成海报",
		DisablePreview: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["partial_images"] != float64(0) {
		t.Fatalf("partial_images = %v, want 0", tool["partial_images"])
	}
}

func TestBuildPayloadEditModeAppendsInputImage(t *testing.T) {
	imageURL := "data:image/png;base64,abc123"
	raw, err := BuildPayload(Options{
		Prompt:       "把这张图片改成金色科技风",
		Size:         "1024x1024",
		Quality:      "auto",
		ImageDataURL: imageURL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v := mustDecodePayload(t, raw)

	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["action"] != "edit" {
		t.Errorf("action = %v, want edit", tool["action"])
	}
	content := v["input"].([]any)[0].(map[string]any)["content"].([]any)
	if len(content) != 2 {
		t.Fatalf("edit-mode content len = %d, want 2", len(content))
	}
	first := content[0].(map[string]any)
	if first["type"] != "input_text" || first["text"] != "把这张图片改成金色科技风" {
		t.Errorf("input_text = %v", first)
	}
	second := content[1].(map[string]any)
	if second["type"] != "input_image" || second["image_url"] != imageURL {
		t.Errorf("input_image = %v", second)
	}
}

func TestBuildPayloadEmptyPromptError(t *testing.T) {
	_, err := BuildPayload(Options{Prompt: "  "})
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
}

func TestBuildPayloadAlwaysKeepsPromptVerbatim(t *testing.T) {
	// Responses payload 顶层始终带 instructions,禁止文本模型改写用户 prompt。
	b, err := BuildPayload(Options{
		Prompt:  "a tiny red dot",
		Size:    "1024x1024",
		Quality: "auto",
	})
	if err != nil {
		t.Fatal(err)
	}
	var p map[string]any
	if err := json.Unmarshal(b, &p); err != nil {
		t.Fatal(err)
	}
	instr, ok := p["instructions"].(string)
	if !ok || instr == "" {
		t.Errorf("expected non-empty instructions, got %v", p["instructions"])
	}
	if !strings.Contains(instr, "VERBATIM") {
		t.Errorf("instructions missing VERBATIM directive: %s", instr)
	}
}

func TestRepairSizeForOpenAIOptionsSnapsToNearestLegalSize(t *testing.T) {
	repaired := repairSizeForOpenAIOptions(Options{
		Prompt: "cat",
		Size:   "872x2048",
	})
	if repaired == nil {
		t.Fatal("expected repaired options")
	}
	if repaired.Size != "880x2048" {
		t.Fatalf("repaired size = %q, want 880x2048", repaired.Size)
	}
}

func TestBuildPayloadMultiImageReferences(t *testing.T) {
	urls := []string{
		"data:image/png;base64,AAA",
		"data:image/png;base64,BBB",
		"data:image/jpeg;base64,CCC",
	}
	raw, err := BuildPayload(Options{
		Prompt:        "combine these references",
		ImageDataURLs: urls,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	if tool["action"] != "edit" {
		t.Errorf("action = %v, want edit when references provided", tool["action"])
	}
	content := v["input"].([]any)[0].(map[string]any)["content"].([]any)
	if len(content) != 1+len(urls) {
		t.Fatalf("content len = %d, want %d (1 text + %d images)", len(content), 1+len(urls), len(urls))
	}
	for i, url := range urls {
		block := content[1+i].(map[string]any)
		if block["type"] != "input_image" {
			t.Errorf("content[%d].type = %v, want input_image", 1+i, block["type"])
		}
		if block["image_url"] != url {
			t.Errorf("content[%d].image_url = %v, want %s", 1+i, block["image_url"], url)
		}
	}
}

func TestBuildPayloadLegacySingleURLAndMultiCoexist(t *testing.T) {
	raw, err := BuildPayload(Options{
		Prompt:        "mix",
		ImageDataURLs: []string{"data:image/png;base64,AAA"},
		ImageDataURL:  "data:image/png;base64,BBB",
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	content := v["input"].([]any)[0].(map[string]any)["content"].([]any)
	if len(content) != 3 {
		t.Fatalf("expected 1 text + 2 images, got %d blocks", len(content))
	}
}

func TestBuildPayloadOmitsMaskWhenEmpty(t *testing.T) {
	raw, _ := BuildPayload(Options{Prompt: "x"})
	if strings.Contains(string(raw), `"mask"`) {
		t.Errorf("payload should not contain mask field when MaskB64 is empty:\n%s", raw)
	}
}

func TestBuildPayloadIncludesMaskWhenSet(t *testing.T) {
	maskB64 := base64.StdEncoding.EncodeToString(testAlphaMaskPNGBytes(t, 2, 1))
	sourceB64 := base64.StdEncoding.EncodeToString(testPNGBytes(t, 2, 1))
	raw, err := BuildPayload(Options{
		Prompt:        "x",
		Mode:          ModeEdit,
		ImageDataURLs: []string{"data:image/png;base64," + sourceB64},
		MaskB64:       maskB64,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	mask, ok := tool["input_image_mask"].(map[string]any)
	if !ok {
		t.Fatalf("expected input_image_mask object, got %T", tool["input_image_mask"])
	}
	want := "data:image/png;base64," + maskB64
	if mask["image_url"] != want {
		t.Errorf("input_image_mask.image_url = %v, want %s", mask["image_url"], want)
	}
}

func TestBuildPayloadDetectsMaskMimeTypeFromBase64(t *testing.T) {
	jpegMask := base64.StdEncoding.EncodeToString(testJPEGBytes(t, 1, 1))
	sourceB64 := base64.StdEncoding.EncodeToString(testPNGBytes(t, 1, 1))
	raw, err := BuildPayload(Options{
		Prompt:        "x",
		Mode:          ModeEdit,
		ImageDataURLs: []string{"data:image/png;base64," + sourceB64},
		MaskB64:       jpegMask,
	})
	if err != nil {
		t.Fatal(err)
	}
	v := mustDecodePayload(t, raw)
	tool := v["tools"].([]any)[0].(map[string]any)
	mask := tool["input_image_mask"].(map[string]any)
	want := "data:image/jpeg;base64," + jpegMask
	if mask["image_url"] != want {
		t.Fatalf("input_image_mask.image_url = %v, want %s", mask["image_url"], want)
	}
}

func TestBuildPayloadRejectsMaskWithoutEditSource(t *testing.T) {
	maskB64 := base64.StdEncoding.EncodeToString(testAlphaMaskPNGBytes(t, 1, 1))
	if _, err := BuildPayload(Options{Prompt: "x", Mode: ModeGenerate, MaskB64: maskB64}); err == nil || !strings.Contains(err.Error(), "图生图") {
		t.Fatalf("err = %v, want edit-mode validation", err)
	}
	if _, err := BuildPayload(Options{Prompt: "x", Mode: ModeEdit, MaskB64: maskB64}); err == nil || !strings.Contains(err.Error(), "源图") {
		t.Fatalf("err = %v, want source validation", err)
	}
}

func TestBuildPayloadRejectsInvalidMaskImage(t *testing.T) {
	sourceB64 := base64.StdEncoding.EncodeToString(testPNGBytes(t, 1, 1))
	_, err := BuildPayload(Options{
		Prompt:        "x",
		Mode:          ModeEdit,
		ImageDataURLs: []string{"data:image/png;base64," + sourceB64},
		MaskB64:       base64.StdEncoding.EncodeToString([]byte("not-an-image")),
	})
	if err == nil || !strings.Contains(err.Error(), "不是支持的") {
		t.Fatalf("err = %v, want invalid-image validation", err)
	}
}

func TestImageFileToDataURLEncodesPNG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, fakePNG, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ImageFileToDataURL(src)
	if err != nil {
		t.Fatal(err)
	}
	want := "data:image/png;base64," + base64.StdEncoding.EncodeToString(fakePNG)
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestImageFileToDataURLUnsupportedExt(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "x.bmp")
	_ = os.WriteFile(src, fakePNG, 0o644)
	_, err := ImageFileToDataURL(src)
	if err == nil {
		t.Fatal("expected error for unsupported extension")
	}
	if !strings.Contains(err.Error(), "不支持的图片格式") {
		t.Errorf("error message = %q, want 不支持的图片格式", err)
	}
}

func TestImageDataURLFromBase64DefaultsPNG(t *testing.T) {
	got := imageDataURLFromBase64("AAAA", "")
	if got != "data:image/png;base64,AAAA" {
		t.Fatalf("got %q, want png data URL", got)
	}
}

func TestImageFileToDataURLMissingFile(t *testing.T) {
	_, err := ImageFileToDataURL(filepath.Join(t.TempDir(), "nope.png"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestSlugify(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Hello World", "hello-world"},
		{"  多 个   空 格  ", "多-个-空-格"},
		{"中文 Mix 123", "中文-mix-123"},
		{"", "image"},
		{"!!!", "image"},
	}
	for _, c := range cases {
		if got := Slugify(c.in, ""); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizePath(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{` "E:\foo.png" `, `E:\foo.png`, false},
		{`'/tmp/x.jpg'`, `/tmp/x.jpg`, false},
		{`  `, "", true},
	}
	for _, c := range cases {
		got, err := NormalizePath(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("NormalizePath(%q) wanted error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("NormalizePath(%q) err = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("NormalizePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{500, "500 B"},
		{2048, "2.0 KB"},
		{int64(5 * 1024 * 1024), "5.0 MB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
