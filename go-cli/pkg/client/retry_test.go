package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRetryableCloudflare524(t *testing.T) {
	html, err := os.ReadFile(filepath.Join("..", "..", "testdata", "cloudflare_524.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsRetryable(string(html)) {
		t.Errorf("Cloudflare 524 HTML should be retryable")
	}
	if !strings.Contains(DescribeProblem(string(html)), "Cloudflare 524") {
		t.Errorf("DescribeProblem missing Cloudflare 524 marker: %q", DescribeProblem(string(html)))
	}
}

func TestIsRetryableJSON504(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "testdata", "json_504.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !IsRetryable(string(body)) {
		t.Errorf("JSON 504 body should be retryable")
	}
	if !strings.Contains(DescribeProblem(string(body)), "504") {
		t.Errorf("DescribeProblem missing 504 marker: %q", DescribeProblem(string(body)))
	}
}

func TestIsRetryableJSON403(t *testing.T) {
	body := `{"error":{"message":"Upstream request failed","type":"upstream_error","upstreamStatus":403}}`
	if !IsRetryable(body) {
		t.Errorf("JSON upstream 403 body should be retryable")
	}
}

func TestIsRetryableFalseForSuccess(t *testing.T) {
	if IsRetryable(`{"status":200,"output":[]}`) {
		t.Errorf("200 success should not be retryable")
	}
}

func TestDescribeProblemEmpty(t *testing.T) {
	if DescribeProblem("") != "接口返回为空。" {
		t.Errorf("empty body description wrong")
	}
}

func TestDescribeProblemExtractsRefusalTextFromResponsesSSE(t *testing.T) {
	raw := strings.Join([]string{
		`data: {"type":"response.output_item.done","item":{"type":"message","status":"completed","content":[{"type":"output_text","text":"抱歉，这个请求包含成人裸露，我无法生成这类真实照片风格图片。"}]}}`,
		`data: {"type":"response.completed","response":{"status":"completed","output":[{"type":"image_generation_call","status":"failed"}]}}`,
	}, "\n")
	if got := DescribeProblem(raw); got != "抱歉，这个请求包含成人裸露，我无法生成这类真实照片风格图片。" {
		t.Fatalf("DescribeProblem refusal text = %q", got)
	}
}

func TestDescribeProblemSpecialErrorCodesMatchSharedRequestModel(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "moderation blocked",
			raw:  `{"error":{"code":"moderation_blocked","message":"blocked","type":"invalid_request_error"}}`,
			want: "🚫 上游内容审核拦截 · 生成被拒",
		},
		{
			name: "content policy violation",
			raw:  `{"error":{"code":"content_policy_violation","message":"policy","type":"invalid_request_error"}}`,
			want: "🚫 上游内容政策拦截 (content_policy_violation)",
		},
		{
			name: "rate limit exceeded",
			raw:  `{"error":{"code":"rate_limit_exceeded","message":"too many requests","type":"rate_limit_error"}}`,
			want: "⏱ 上游限速 (rate_limit_exceeded)\n\ntoo many requests",
		},
		{
			name: "quota exhausted",
			raw:  `{"error":{"code":"insufficient_quota","message":"quota empty","type":"billing_error"}}`,
			want: "💳 上游账户额度不足\n\nquota empty",
		},
		{
			name: "model not found",
			raw:  `{"error":{"code":"model_not_found","message":"missing model","type":"invalid_request_error"}}`,
			want: "🤷 上游找不到指定模型\n\nmissing model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DescribeProblem(tt.raw); got != tt.want {
				t.Fatalf("DescribeProblem(%s) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}
