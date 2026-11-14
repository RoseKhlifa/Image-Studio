package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/yuanhua/image-gptcodex/internal/fsio"
	"github.com/yuanhua/image-gptcodex/internal/promptui"
	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "发生错误:", err)
		os.Exit(1)
	}
}

func run() error {
	apiKey := flag.String("api-key", "", "GPTCODEX API key (overrides interactive prompt; env GPTCODEX_API_KEY also accepted)")
	mode := flag.String("mode", "", "generate | edit (overrides interactive prompt)")
	image := flag.String("image", "", "source image path (required for edit mode)")
	size := flag.String("size", "", "1024x1024 | 1536x1024 | 1024x1536 | 2048x1152")
	quality := flag.String("quality", "", "auto | low | medium | high | xhigh | max")
	outputFormat := flag.String("output-format", "png", "png | jpeg | webp | gif | svg")
	complexity := flag.String("complexity", "high", "low | medium | high (GPT Image 2.6)")
	frameCount := flag.Int("frame-count", 0, "GIF frame count (2-16; default 9)")
	frameDurationMs := flag.Int("frame-duration-ms", 0, "GIF frame duration in milliseconds (20-2000; default 200)")
	prompt := flag.String("prompt", "", "prompt text (or edit instructions)")
	outDir := flag.String("out-dir", "", "output directory (default: ./images)")
	flag.Parse()

	if envKey := os.Getenv("GPTCODEX_API_KEY"); envKey != "" && *apiKey == "" {
		*apiKey = envKey
	}

	fmt.Println("GPTCODEX 图片生成器")
	fmt.Println()

	p := promptui.NewPrompter()

	var err error
	if strings.TrimSpace(*apiKey) == "" {
		if *apiKey, err = p.APIKey(); err != nil {
			return err
		}
	}

	var resolvedMode client.Mode
	switch *mode {
	case "generate":
		resolvedMode = client.ModeGenerate
	case "edit":
		resolvedMode = client.ModeEdit
	case "":
		if resolvedMode, err = p.Mode(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("--mode 必须是 generate 或 edit")
	}

	var imageDataURL string
	var sourceImagePath string
	if resolvedMode == client.ModeEdit {
		if *image == "" {
			if sourceImagePath, err = p.ImagePath(); err != nil {
				return err
			}
		} else {
			if sourceImagePath, err = client.NormalizePath(*image); err != nil {
				return err
			}
		}
		if imageDataURL, err = client.ImageFileToDataURL(sourceImagePath); err != nil {
			return err
		}
	}

	if *size == "" {
		if *size, err = p.Size(); err != nil {
			return err
		}
	}
	if *quality == "" {
		if *quality, err = p.Quality(); err != nil {
			return err
		}
	}
	if *prompt == "" {
		if *prompt, err = p.PromptText(resolvedMode); err != nil {
			return err
		}
	}

	transport, err := client.PickTransport()
	if err != nil {
		return err
	}

	output := *outDir
	if output == "" {
		output = fsio.DefaultOutputDir()
	}
	if err := fsio.EnsureDir(output); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102-150405")

	fmt.Println()
	actionLabel := "生成图片"
	if resolvedMode == client.ModeEdit {
		actionLabel = "编辑图片"
	}
	fmt.Printf("正在请求%s,比例 %s,质量 %s...\n", actionLabel, *size, *quality)
	if sourceImagePath != "" {
		abs, _ := filepath.Abs(sourceImagePath)
		fmt.Printf("源图片:%s\n", abs)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	opts := client.Options{
		APIKey:          *apiKey,
		Prompt:          *prompt,
		Mode:            resolvedMode,
		Size:            *size,
		Quality:         *quality,
		OutputFormat:    client.NormalizeOutputFormat(*outputFormat),
		Complexity:      *complexity,
		FrameCount:      *frameCount,
		FrameDurationMs: *frameDurationMs,
		ImageDataURL:    imageDataURL,
	}

	logger := func(msg string) {
		fmt.Println(msg)
	}
	progress := func(stage string, elapsed int, bytes int64) {
		fmt.Printf("已等待 %d 秒,状态:%s,已接收 %s...\n", elapsed, stage, client.FormatBytes(bytes))
	}

	result, rawPath, err := client.RequestAndExtractWithRetries(ctx, transport, opts, output, timestamp, logger, progress)
	if err != nil {
		return err
	}

	imageName := fsio.BuildImageName(resolvedMode, *prompt, timestamp, opts.OutputFormat)
	imagePath, err := fsio.SaveImage(result.ImageB64, filepath.Join(output, imageName))
	if err != nil {
		return err
	}
	absRaw, _ := filepath.Abs(rawPath)

	fmt.Printf("图片已保存:%s\n", imagePath)
	fmt.Printf("原始返回已保存:%s\n", absRaw)
	if result.RevisedPrompt != "" {
		fmt.Printf("修订提示词:%s\n", result.RevisedPrompt)
	}
	return nil
}
