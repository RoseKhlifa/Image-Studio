package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRequestImagesAPIWithPartialStreamsPreviews(t *testing.T) {
	partialB64 := base64.StdEncoding.EncodeToString([]byte("partial"))
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	var requestBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"type\":\"image_generation.partial_image\",\"partial_image_index\":0,\"b64_json\":\"%s\"}\n", partialB64)
		fmt.Fprintf(w, "data: {\"type\":\"image_generation.completed\",\"b64_json\":\"%s\"}\n", finalB64)
	}))
	defer srv.Close()

	var partials []PartialImage
	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:         "sk-test",
		Prompt:         "cat",
		BaseURL:        srv.URL,
		APIMode:        APIModeImages,
		PartialImages:  2,
		UserIdentifier: "user-hash-123",
	}, &bytes.Buffer{}, nil, func(partial PartialImage) {
		partials = append(partials, partial)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requestBody), `"stream":true`) {
		t.Fatalf("request body missing stream=true: %s", requestBody)
	}
	if !strings.Contains(string(requestBody), `"background":"auto"`) {
		t.Fatalf("request body missing background=auto: %s", requestBody)
	}
	if !strings.Contains(string(requestBody), `"moderation":"low"`) {
		t.Fatalf("request body missing moderation=low: %s", requestBody)
	}
	if !strings.Contains(string(requestBody), `"user":"user-hash-123"`) {
		t.Fatalf("request body missing user=user-hash-123: %s", requestBody)
	}
	if !strings.Contains(string(requestBody), `"partial_images":2`) {
		t.Fatalf("request body missing partial_images=2: %s", requestBody)
	}
	if res.ImageB64 != finalB64 || res.SourceEvent != "images_api" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(partials) != 1 || partials[0].ImageB64 != partialB64 || partials[0].PartialImageIndex != 0 {
		t.Fatalf("unexpected partials: %+v", partials)
	}
}

func TestRequestImagesAPINewAPICompatSkipsEmptyJSONKeepAlives(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		flusher, _ := w.(http.Flusher)
		_, _ = io.WriteString(w, "{}\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "[]\n\"\"\nnull\n")
		flusher.Flush()
		_, _ = io.WriteString(w, "{\"data\":[]}\n")
		flusher.Flush()
		fmt.Fprintf(w, "{\"data\":[{\"b64_json\":%q,\"revised_prompt\":\"kept alive\"}]}\n", finalB64)
	}))
	defer srv.Close()

	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:             "sk-test",
		Prompt:             "cat",
		BaseURL:            srv.URL,
		APIMode:            APIModeImages,
		ImagesNewAPICompat: true,
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ImageB64 != finalB64 || res.RevisedPrompt != "kept alive" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRequestImagesAPINewAPICompatRejectsOnlyEmptyKeepAlives(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{}\n[]\n\"\"\nnull\n{\"data\":[]}\n")
	}))
	defer srv.Close()

	_, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:             "sk-test",
		Prompt:             "cat",
		BaseURL:            srv.URL,
		APIMode:            APIModeImages,
		ImagesNewAPICompat: true,
	}, &bytes.Buffer{}, nil, nil)
	if !errors.Is(err, ErrNoImageInResponse) {
		t.Fatalf("err = %v, want ErrNoImageInResponse", err)
	}
}

func TestRequestImagesAPINewAPICompatSkipsEmptySSEKeepAlives(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data:\n\ndata:{}\n\ndata: {\"data\":[]}\n\n")
		fmt.Fprintf(w, "data:{\"data\":[{\"b64_json\":%q}]}\n\n", finalB64)
	}))
	defer srv.Close()

	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:             "sk-test",
		Prompt:             "cat",
		BaseURL:            srv.URL,
		APIMode:            APIModeImages,
		ImagesNewAPICompat: true,
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ImageB64 != finalB64 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestRequestImagesAPIDownloadsURLOnlyResponse(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	var imageURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"data":[{"url":%q,"revised_prompt":"cat revised"}]}`, imageURL)
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	imageURL = srv.URL + "/image.png"

	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:       "sk-test",
		Prompt:       "cat",
		BaseURL:      srv.URL,
		APIMode:      APIModeImages,
		ImageModelID: "relay-image-model",
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ImageB64 != base64.StdEncoding.EncodeToString(imageBytes) {
		t.Fatalf("ImageB64 = %q", res.ImageB64)
	}
	if res.RevisedPrompt != "cat revised" {
		t.Fatalf("RevisedPrompt = %q", res.RevisedPrompt)
	}
	if res.SourceEvent != "images_api_url" {
		t.Fatalf("SourceEvent = %q", res.SourceEvent)
	}
}

func TestRequestImagesAPIDownloadsURLOnlySSEResponse(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x01}
	var imageURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/images/generations":
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprintf(w, "data: {\"data\":[{\"url\":%q,\"revised_prompt\":\"cat revised from stream\"}]}\n\n", imageURL)
		case "/image.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(imageBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	imageURL = srv.URL + "/image.png"

	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:       "sk-test",
		Prompt:       "cat",
		BaseURL:      srv.URL,
		APIMode:      APIModeImages,
		ImageModelID: "relay-image-model",
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.ImageB64 != base64.StdEncoding.EncodeToString(imageBytes) {
		t.Fatalf("ImageB64 = %q", res.ImageB64)
	}
	if res.RevisedPrompt != "cat revised from stream" {
		t.Fatalf("RevisedPrompt = %q", res.RevisedPrompt)
	}
	if res.SourceEvent != "images_api_url" {
		t.Fatalf("SourceEvent = %q", res.SourceEvent)
	}
}

func TestRequestImagesAPIGeminiModelUsesNonStreamingCompatEndpoint(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	var requestBody []byte
	var requestPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		requestBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, finalB64)
	}))
	defer srv.Close()

	res, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:       "sk-test",
		Prompt:       "cat",
		BaseURL:      srv.URL + "/v1beta/openai",
		APIMode:      APIModeImages,
		ImageModelID: "gemini-3.1-flash-image",
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if requestPath != "/v1beta/openai/images/generations" {
		t.Fatalf("request path = %q", requestPath)
	}
	if strings.Contains(string(requestBody), `"stream"`) {
		t.Fatalf("gemini request should omit stream: %s", requestBody)
	}
	if !strings.Contains(string(requestBody), `"response_format":"b64_json"`) {
		t.Fatalf("gemini request should request b64_json: %s", requestBody)
	}
	if res.ImageB64 != finalB64 {
		t.Fatalf("ImageB64 = %q", res.ImageB64)
	}
}

func TestBuildGoogleInteractionPayloadMatchesOfficialNanoBanana2Fixture(t *testing.T) {
	raw, err := buildGoogleInteractionPayload(Options{
		Prompt:       "draw a cat",
		ImageModelID: "gemini-3.1-flash-image",
	}, nil, "gemini-3.1-flash-image", "2048x1152", "png")
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["model"] != "gemini-3.1-flash-image" || payload["input"] != "draw a cat" {
		t.Fatalf("unexpected interaction payload: %s", raw)
	}
	if payload["store"] != false {
		t.Fatalf("store = %v, want false", payload["store"])
	}
	format, ok := payload["response_format"].(map[string]any)
	if !ok {
		t.Fatalf("response_format = %T", payload["response_format"])
	}
	if format["type"] != "image" || format["delivery"] != "inline" || format["mime_type"] != "image/png" {
		t.Fatalf("response_format = %#v", format)
	}
	if format["aspect_ratio"] != "16:9" || format["image_size"] != "2K" {
		t.Fatalf("response_format size fields = %#v", format)
	}
}

func TestExtractGoogleInteractionImageFromModelOutputStepFixture(t *testing.T) {
	imageBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	wantB64 := base64.StdEncoding.EncodeToString(imageBytes)
	raw := []byte(fmt.Sprintf(`{
		"object":"interaction",
		"status":"completed",
		"steps":[{"type":"model_output","content":[{"type":"image","data":%q,"mime_type":"image/png"}]}]
	}`, wantB64))
	image, err := extractGoogleInteractionImage(raw, http.StatusOK)
	if err != nil {
		t.Fatal(err)
	}
	result, err := imageResultFromGoogleInteraction(image)
	if err != nil {
		t.Fatal(err)
	}
	if result.ImageB64 != wantB64 || result.SourceEvent != "google_interactions" {
		t.Fatalf("result = %#v", result)
	}
}

func TestBuildGoogleInteractionPayloadRejectsMaskInsteadOfSilentlyIgnoringIt(t *testing.T) {
	_, err := buildGoogleInteractionPayload(Options{
		Prompt:  "edit cat",
		MaskB64: "bWFzaw==",
	}, nil, "gemini-3.1-flash-image", "1024x1024", "png")
	if err == nil || !strings.Contains(err.Error(), "不支持 OpenAI mask") {
		t.Fatalf("err = %v", err)
	}
}

func TestRequestImagesAPIWithRetriesRetriesWhenOnlyPartialPreviewArrives(t *testing.T) {
	partialB64 := base64.StdEncoding.EncodeToString([]byte("partial"))
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if hits == 1 {
			fmt.Fprintf(w, "data: {\"type\":\"image_generation.partial_image\",\"partial_image_index\":0,\"b64_json\":\"%s\"}\n", partialB64)
			fmt.Fprintln(w, `data: {"type":"response.completed","response":{"status":"completed"}}`)
			return
		}
		fmt.Fprintf(w, "data: {\"type\":\"image_generation.completed\",\"b64_json\":\"%s\"}\n", finalB64)
	}))
	defer srv.Close()

	original := RetryBackoffSeconds
	RetryBackoffSeconds = 0
	t.Cleanup(func() { RetryBackoffSeconds = original })

	var partials []PartialImage
	res, _, err := RequestAndExtractWithRetriesAndPartial(
		context.Background(),
		nil,
		Options{
			APIKey:        "sk-test",
			Prompt:        "cat",
			BaseURL:       srv.URL,
			APIMode:       APIModeImages,
			PartialImages: 2,
		},
		t.TempDir(),
		"20260518-200004",
		nil,
		nil,
		func(partial PartialImage) {
			partials = append(partials, partial)
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Fatalf("hits = %d, want 2", hits)
	}
	if res.ImageB64 != finalB64 || res.SourceEvent != "images_api" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if len(partials) != 1 || partials[0].ImageB64 != partialB64 {
		t.Fatalf("unexpected partials: %+v", partials)
	}
}

func TestBuildEditsMultipartSetsMaskMimeType(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, testPNGBytes(t, 2, 1), 0o644); err != nil {
		t.Fatal(err)
	}

	buf, contentType, err := buildEditsMultipart(
		[]string{src},
		base64.StdEncoding.EncodeToString(testAlphaMaskPNGBytes(t, 2, 1)),
		"edit this",
		"gpt-image-2",
		"1024x1024",
		"auto",
		"png",
		"auto",
		100,
		"auto",
		"low",
		"user-hash-123",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(buf, params["boundary"])
	foundMask := false
	foundBackground := false
	foundInputFidelity := false
	foundModeration := false
	foundUser := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.FormName() == "mask" {
			foundMask = true
			if got := part.Header.Get("Content-Type"); got != "image/png" {
				t.Fatalf("mask content-type = %q, want image/png", got)
			}
		}
		if part.FormName() == "moderation" {
			foundModeration = true
		}
		if part.FormName() == "background" {
			foundBackground = true
		}
		if part.FormName() == "user" {
			foundUser = true
		}
		if part.FormName() == "input_fidelity" {
			foundInputFidelity = true
		}
		_, _ = io.Copy(io.Discard, part)
	}
	if !foundMask {
		t.Fatal("expected mask part in multipart body")
	}
	if !foundBackground {
		t.Fatal("expected background field in multipart body")
	}
	if foundInputFidelity {
		t.Fatal("gpt-image-2 multipart body should omit input_fidelity")
	}
	if !foundModeration {
		t.Fatal("expected moderation field in multipart body")
	}
	if !foundUser {
		t.Fatal("expected user field in multipart body")
	}
}

func TestBuildEditsMultipartUsesArrayFieldForEveryMultiImageSource(t *testing.T) {
	dir := t.TempDir()
	paths := []string{
		filepath.Join(dir, "first.png"),
		filepath.Join(dir, "second.png"),
	}
	for _, path := range paths {
		if err := os.WriteFile(path, fakePNG, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	buf, contentType, err := buildEditsMultipart(
		paths,
		"",
		"edit this",
		"gpt-image-2",
		"1024x1024",
		"auto",
		"png",
		"auto",
		100,
		"auto",
		"low",
		"",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(buf, params["boundary"])
	var imageFields []string
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.FileName() != "" {
			imageFields = append(imageFields, part.FormName())
		}
		_, _ = io.Copy(io.Discard, part)
	}
	if got, want := strings.Join(imageFields, ","), "image[],image[]"; got != want {
		t.Fatalf("image multipart fields = %q, want %q", got, want)
	}
}

func TestBuildEditsMultipartCanonicalizesJPEGSourceAndMaskToSameSizePNG(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.jpg")
	if err := os.WriteFile(src, testJPEGBytes(t, 3, 2), 0o644); err != nil {
		t.Fatal(err)
	}
	jpegMask := base64.StdEncoding.EncodeToString(testJPEGBytes(t, 1, 1))

	buf, contentType, err := buildEditsMultipart(
		[]string{src},
		jpegMask,
		"edit this",
		"gpt-image-2",
		"1024x1024",
		"auto",
		"png",
		"auto",
		100,
		"auto",
		"low",
		"",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatal(err)
	}
	reader := multipart.NewReader(buf, params["boundary"])
	foundSource := false
	foundMask := false
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if part.FormName() == "image" || part.FormName() == "mask" {
			if got := part.Header.Get("Content-Type"); got != "image/png" {
				t.Fatalf("%s content-type = %q, want image/png", part.FormName(), got)
			}
			wantName := "source.png"
			if part.FormName() == "mask" {
				wantName = "mask.png"
				foundMask = true
			} else {
				foundSource = true
			}
			if got := part.FileName(); got != wantName {
				t.Fatalf("%s filename = %q, want %q", part.FormName(), got, wantName)
			}
			img, err := png.Decode(part)
			if err != nil {
				t.Fatalf("decode %s PNG: %v", part.FormName(), err)
			}
			if got := img.Bounds().Size(); got != image.Pt(3, 2) {
				t.Fatalf("%s size = %v, want 3x2", part.FormName(), got)
			}
			continue
		}
		_, _ = io.Copy(io.Discard, part)
	}
	if !foundSource || !foundMask {
		t.Fatalf("canonical pair missing: source=%v mask=%v", foundSource, foundMask)
	}
}

func TestRequestImagesAPIRejectsMaskOutsideEditBeforeNetwork(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:  "sk-test",
		BaseURL: srv.URL,
		Prompt:  "cat",
		Mode:    ModeGenerate,
		MaskB64: base64.StdEncoding.EncodeToString(testAlphaMaskPNGBytes(t, 1, 1)),
	}, &bytes.Buffer{}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "图生图") {
		t.Fatalf("err = %v, want edit-mode validation", err)
	}
	if hits != 0 {
		t.Fatalf("server received %d requests, want 0", hits)
	}
}

func TestBuildEditsMultipartOmitsMaskWhenEmpty(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, fakePNG, 0o644); err != nil {
		t.Fatal(err)
	}

	buf, _, err := buildEditsMultipart(
		[]string{src},
		"",
		"edit this",
		"gpt-image-2",
		"1024x1024",
		"auto",
		"png",
		"auto",
		100,
		"auto",
		"low",
		"",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `name="mask"`) {
		t.Fatal("multipart body should omit mask part when mask is empty")
	}
}

func TestBuildEditsMultipartIncludesOutputCompressionForWebP(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, fakePNG, 0o644); err != nil {
		t.Fatal(err)
	}

	buf, _, err := buildEditsMultipart(
		[]string{src},
		"",
		"edit this",
		"gpt-image-2",
		"1024x1024",
		"auto",
		"webp",
		"opaque",
		42,
		"auto",
		"low",
		"",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `name="output_compression"`) {
		t.Fatal("multipart body should include output_compression for webp")
	}
}

func TestBuildEditsMultipartIncludesInputFidelityForSupportedModels(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "source.png")
	if err := os.WriteFile(src, fakePNG, 0o644); err != nil {
		t.Fatal(err)
	}

	buf, _, err := buildEditsMultipart(
		[]string{src},
		"",
		"edit this",
		"gpt-image-1.5",
		"1024x1024",
		"auto",
		"png",
		"auto",
		100,
		"high",
		"low",
		"",
		"",
		0,
		RequestPolicyOpenAI,
		DefaultPartialImages,
		false,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `name="input_fidelity"`) {
		t.Fatal("multipart body should include input_fidelity for supported models")
	}
}

func TestRequestImagesAPISendsDalle3Style(t *testing.T) {
	var requestBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":[{"b64_json":"ZmluYWw="}]}`)
	}))
	defer srv.Close()

	_, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey:       "sk-test",
		Prompt:       "cat",
		BaseURL:      srv.URL,
		APIMode:      APIModeImages,
		ImageModelID: "dall-e-3",
		ImageStyle:   "natural",
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requestBody), `"style":"natural"`) {
		t.Fatalf("request body missing style=natural: %s", requestBody)
	}
}

func TestRequestImagesAPIExplicitGoogleProviderUsesInteractionsOnCustomBase(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString(fakePNG)
	var requestPath string
	var googleKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		googleKey = r.Header.Get("X-Goog-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status":"completed","output_image":{"type":"image","data":%q,"mime_type":"image/png"}}`, finalB64)
	}))
	defer srv.Close()

	result, err := RequestImagesAPIWithPartial(context.Background(), Options{
		APIKey: "google-key", Provider: ProviderGoogle, BaseURL: srv.URL,
		Prompt: "cat", ImageModelID: "gemini-image-model", APIMode: APIModeResponses,
	}, &bytes.Buffer{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if requestPath != "/v1beta/interactions" || googleKey != "google-key" {
		t.Fatalf("path=%q google key=%q", requestPath, googleKey)
	}
	if result.ImageB64 != finalB64 || result.SourceEvent != "google_interactions" {
		t.Fatalf("result=%+v", result)
	}
}

func TestRequestImagesAPIGrokProviderUsesNativeJSONContracts(t *testing.T) {
	finalB64 := base64.StdEncoding.EncodeToString([]byte("final"))
	dir := t.TempDir()
	source := filepath.Join(dir, "source.png")
	if err := os.WriteFile(source, fakePNG, 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		mode     Mode
		paths    []string
		wantPath string
	}{
		{name: "generation", mode: ModeGenerate, wantPath: "/v1/images/generations"},
		{name: "edit", mode: ModeEdit, paths: []string{source}, wantPath: "/v1/images/edits"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestBody []byte
			var requestPath, contentType, authorization string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestPath = r.URL.Path
				contentType = r.Header.Get("Content-Type")
				authorization = r.Header.Get("Authorization")
				requestBody, _ = io.ReadAll(r.Body)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{"data":[{"b64_json":%q}]}`, finalB64)
			}))
			defer srv.Close()

			_, err := RequestImagesAPIWithPartial(context.Background(), Options{
				APIKey: "xai-key", Provider: ProviderGrok, BaseURL: srv.URL,
				Prompt: "cat", ImageModelID: "grok-imagine-image", Mode: tt.mode,
				ImagePaths: tt.paths, Size: "2048x1152", APIMode: APIModeResponses,
			}, &bytes.Buffer{}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if requestPath != tt.wantPath || contentType != "application/json" || authorization != "Bearer xai-key" {
				t.Fatalf("path=%q content-type=%q auth=%q", requestPath, contentType, authorization)
			}
			var payload map[string]any
			if err := json.Unmarshal(requestBody, &payload); err != nil {
				t.Fatal(err)
			}
			if payload["aspect_ratio"] != "16:9" || payload["resolution"] != "2k" || payload["response_format"] != "b64_json" {
				t.Fatalf("payload=%s", requestBody)
			}
			_, hasImage := payload["image"]
			if hasImage != (tt.mode == ModeEdit) {
				t.Fatalf("image presence=%v payload=%s", hasImage, requestBody)
			}
		})
	}
}
