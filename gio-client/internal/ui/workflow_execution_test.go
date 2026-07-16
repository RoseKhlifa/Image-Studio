package ui

import (
	"path/filepath"
	"strings"
	"testing"

	"image-studio/gio-client/internal/kernel"

	"github.com/yuanhua/image-gptcodex/pkg/client"
)

func mustToggleWorkflowEdge(t *testing.T, graph workflowGraphModel, edge workflowEdgeModel) workflowGraphModel {
	t.Helper()
	next, err := toggleWorkflowConnection(graph, edge)
	if err != nil {
		t.Fatalf("toggle %s: %v", workflowEdgeID(edge), err)
	}
	return next
}

func mustConfigureWorkflowNode(t *testing.T, graph workflowGraphModel, nodeID string, update func(map[string]string)) workflowGraphModel {
	t.Helper()
	node, ok := graph.node(nodeID)
	if !ok {
		t.Fatalf("node %s missing", nodeID)
	}
	properties := cloneWorkflowProperties(node.Properties)
	update(properties)
	return configureWorkflowNode(graph, nodeID, node.Title, properties)
}

func TestBuildWorkflowExecutionPlanSupportsPreviewAndDirectBranches(t *testing.T) {
	graph := defaultWorkflowGraph()
	plan, err := buildWorkflowExecutionPlan(graph, "export", false)
	if err != nil {
		t.Fatalf("build preview plan: %v", err)
	}
	if plan.Direct || plan.Preview == nil || plan.Generate.ID != "generate" || plan.Export.ID != "export" {
		t.Fatalf("preview plan=%+v", plan)
	}

	direct := mustToggleWorkflowEdge(t, graph, workflowEdgeModel{FromNode: "generate", FromPort: "image", ToNode: "export", ToPort: "image"})
	plan, err = buildWorkflowExecutionPlan(direct, "export", false)
	if err != nil {
		t.Fatalf("build direct plan: %v", err)
	}
	if !plan.Direct || plan.Preview != nil || plan.Generate.ID != "generate" {
		t.Fatalf("direct plan=%+v", plan)
	}
}

func TestBuildWorkflowExecutionPlanUsesConnectedDuplicatePrompt(t *testing.T) {
	graph, promptID, err := addWorkflowNodeInstance(defaultWorkflowGraph(), "prompt")
	if err != nil {
		t.Fatalf("add prompt: %v", err)
	}
	graph = mustConfigureWorkflowNode(t, graph, promptID, func(properties map[string]string) {
		properties[workflowPropertyPrompt] = "branch prompt"
	})
	graph = mustToggleWorkflowEdge(t, graph, workflowEdgeModel{FromNode: promptID, FromPort: "text", ToNode: "generate", ToPort: "prompt"})

	plan, err := buildWorkflowExecutionPlan(graph, "export", false)
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if plan.Prompt.ID != promptID {
		t.Fatalf("prompt=%q want %q", plan.Prompt.ID, promptID)
	}
	cfg, _ := (&App{}).applyWorkflowExecutionPlan(kernel.DefaultConfig(), plan)
	if cfg.Prompt != "branch prompt" {
		t.Fatalf("compiled prompt=%q", cfg.Prompt)
	}
}

func TestBuildWorkflowExecutionPlanRequiresSelectedOutputForMultipleBranches(t *testing.T) {
	graph, exportID, err := addWorkflowNodeInstance(defaultWorkflowGraph(), "export")
	if err != nil {
		t.Fatalf("add export: %v", err)
	}
	graph = mustToggleWorkflowEdge(t, graph, workflowEdgeModel{FromNode: "generate", FromPort: "image", ToNode: exportID, ToPort: "image"})
	if _, err := buildWorkflowExecutionPlan(graph, "", false); err == nil || !strings.Contains(err.Error(), "选中") {
		t.Fatalf("multiple outputs error=%v", err)
	}
	plan, err := buildWorkflowExecutionPlan(graph, exportID, false)
	if err != nil {
		t.Fatalf("build selected output: %v", err)
	}
	if plan.Export.ID != exportID || !plan.Direct {
		t.Fatalf("selected plan=%+v", plan)
	}
	disconnected := mustToggleWorkflowEdge(t, graph, workflowEdgeModel{FromNode: "generate", FromPort: "image", ToNode: exportID, ToPort: "image"})
	if _, err := buildWorkflowExecutionPlan(disconnected, exportID, false); err == nil || !strings.Contains(err.Error(), "缺少图像输入") {
		t.Fatalf("disconnected selected output error=%v", err)
	}
}

func TestApplyWorkflowExecutionPlanUsesOnlyNodeProperties(t *testing.T) {
	graph := defaultWorkflowGraph()
	outputDir := filepath.Join(t.TempDir(), "branch-output")
	graph = mustConfigureWorkflowNode(t, graph, "prompt", func(properties map[string]string) {
		properties[workflowPropertyPrompt] = "node prompt"
		properties[workflowPropertyNegative] = "noise"
		properties[workflowPropertyStyleTag] = "editorial"
	})
	graph = mustConfigureWorkflowNode(t, graph, "source", func(properties map[string]string) {
		properties[workflowPropertySourcePaths] = "/tmp/one.png\n/tmp/two.png"
	})
	graph = mustConfigureWorkflowNode(t, graph, "generate", func(properties map[string]string) {
		properties[workflowPropertyMode] = string(client.ModeEdit)
		properties[workflowPropertyQuality] = "high"
		properties[workflowPropertySize] = "1536x1024"
		properties[workflowPropertyImageModel] = "branch-model"
		properties[workflowPropertyBatchCount] = "6"
	})
	graph = mustConfigureWorkflowNode(t, graph, "preview", func(properties map[string]string) {
		properties[workflowPropertyPartialImages] = "3"
	})
	graph = mustConfigureWorkflowNode(t, graph, "export", func(properties map[string]string) {
		properties[workflowPropertyOutputFormat] = "webp"
		properties[workflowPropertyOutputDir] = outputDir
	})
	plan, err := buildWorkflowExecutionPlan(graph, "export", true)
	if err != nil {
		t.Fatalf("build configured plan: %v", err)
	}
	base := kernel.DefaultConfig()
	base.Prompt = "global prompt"
	base.ImageModelID = "global-model"
	cfg, total := (&App{}).applyWorkflowExecutionPlan(base, plan)
	if cfg.Prompt != "node prompt" || cfg.NegativePrompt != "noise" || cfg.StyleTag != "editorial" {
		t.Fatalf("compiled prompt settings=%+v", cfg)
	}
	if cfg.Mode != client.ModeEdit || cfg.Quality != "high" || cfg.Size != "1536x1024" || cfg.ImageModelID != "branch-model" || total != 6 {
		t.Fatalf("compiled generation settings=%+v total=%d", cfg, total)
	}
	if len(cfg.SourcePaths) != 2 || cfg.PartialImages != 3 || cfg.OutputFormat != "webp" || cfg.OutputDir != outputDir {
		t.Fatalf("compiled io settings=%+v", cfg)
	}
}

func TestBuildWorkflowExecutionPlanRejectsMissingOrInvalidNodeProperties(t *testing.T) {
	graph := defaultWorkflowGraph()
	generate, _ := graph.node("generate")
	delete(generate.Properties, workflowPropertyImageModel)
	graph = setWorkflowNodeProperties(graph, generate.ID, generate.Properties)
	if _, err := buildWorkflowExecutionPlan(graph, "export", false); err == nil || !strings.Contains(err.Error(), "图像模型") {
		t.Fatalf("missing property error=%v", err)
	}

	graph = defaultWorkflowGraph()
	graph = mustConfigureWorkflowNode(t, graph, "generate", func(properties map[string]string) {
		properties[workflowPropertyBatchCount] = "99"
	})
	if _, err := buildWorkflowExecutionPlan(graph, "export", false); err == nil || !strings.Contains(err.Error(), "生成张数") {
		t.Fatalf("invalid property error=%v", err)
	}
}
