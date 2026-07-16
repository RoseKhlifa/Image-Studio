package ui

import (
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"image-studio/gio-client/internal/desktopstate"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

const validWorkflowNodeManifestJSON = `{
  "format": "image-studio-workflow-node",
  "schemaVersion": 1,
  "id": "com.example.cinematic-generate",
  "version": "1.0.0",
  "displayName": "电影感生成",
  "description": "预配置电影感模型和参数",
  "category": "自定义/生成",
  "operator": "generate",
  "defaults": {
    "mode": "generate",
    "quality": "high",
    "size": "1536x1024",
    "image_model": "gpt-image-2",
    "batch_count": "1"
  }
}`

func mustParseWorkflowNodeManifest(t *testing.T, data string) parsedWorkflowNodeManifest {
	t.Helper()
	parsed, err := parseWorkflowNodeManifest([]byte(data))
	if err != nil {
		t.Fatalf("parse workflow node manifest: %v", err)
	}
	return parsed
}

func TestExampleWorkflowNodeManifestIsValid(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "workflow-nodes", "cinematic-generate.json"))
	if err != nil {
		t.Fatalf("read example workflow node manifest: %v", err)
	}
	parsed, err := parseWorkflowNodeManifest(data)
	if err != nil {
		t.Fatalf("parse example workflow node manifest: %v", err)
	}
	if parsed.Manifest.ID != "com.example.cinematic-generate" {
		t.Fatalf("example node id=%q", parsed.Manifest.ID)
	}
}

func TestParseWorkflowNodeManifestBuildsTrustedVersionedTemplate(t *testing.T) {
	parsed := mustParseWorkflowNodeManifest(t, validWorkflowNodeManifestJSON)
	template := parsed.Template
	if template.TypeID != "com.example.cinematic-generate" || template.TypeVersion != "1.0.0" {
		t.Fatalf("template identity=%q@%q", template.TypeID, template.TypeVersion)
	}
	if template.Kind != workflowNodeGenerate || template.Title != "电影感生成" || template.Category != "自定义/生成" {
		t.Fatalf("template metadata=%+v", template)
	}
	if template.Properties[workflowPropertyQuality] != "high" || template.Properties[workflowPropertySize] != "1536x1024" {
		t.Fatalf("template defaults=%v", template.Properties)
	}
	if len(template.Inputs) != 2 || len(template.Outputs) != 2 {
		t.Fatalf("trusted generate ports inputs=%v outputs=%v", template.Inputs, template.Outputs)
	}
}

func TestParseWorkflowNodeManifestRejectsUnsafeAndMalformedDefinitions(t *testing.T) {
	tests := map[string]string{
		"unknown operator": strings.Replace(validWorkflowNodeManifestJSON, `"operator": "generate"`, `"operator": "shell"`, 1),
		"unknown property": strings.Replace(validWorkflowNodeManifestJSON, `"batch_count": "1"`, `"batch_count": "1", "command": "whoami"`, 1),
		"invalid id":       strings.Replace(validWorkflowNodeManifestJSON, "com.example.cinematic-generate", "../Cinematic", 1),
		"invalid version":  strings.Replace(validWorkflowNodeManifestJSON, `"version": "1.0.0"`, `"version": "1.0"`, 1),
		"invalid default":  strings.Replace(validWorkflowNodeManifestJSON, `"batch_count": "1"`, `"batch_count": "99"`, 1),
		"unknown field":    strings.Replace(validWorkflowNodeManifestJSON, `"schemaVersion": 1,`, `"schemaVersion": 1, "execute": "native",`, 1),
		"trailing json":    validWorkflowNodeManifestJSON + `{}`,
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := parseWorkflowNodeManifest([]byte(data)); err == nil {
				t.Fatal("invalid manifest was accepted")
			}
		})
	}
	if _, err := parseWorkflowNodeManifest([]byte(strings.Repeat("x", maxWorkflowNodeManifestBytes+1))); err == nil {
		t.Fatal("oversized manifest was accepted")
	}
	if validWorkflowNodeVersion("1.0.0-01") {
		t.Fatal("semver numeric prerelease with leading zero was accepted")
	}
	if compareWorkflowNodeVersions("100000000000000000000.0.0", "9.0.0") <= 0 {
		t.Fatal("large semantic versions were compared with integer overflow")
	}
}

func TestWorkflowNodeManifestInstallIsCanonicalImmutableAndDiscoverable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "workflow-nodes")
	parsed := mustParseWorkflowNodeManifest(t, validWorkflowNodeManifestJSON)
	path, installed, err := installWorkflowNodeManifest(dir, parsed)
	if err != nil {
		t.Fatalf("install workflow node: %v", err)
	}
	if !installed || filepath.Dir(path) != dir || filepath.Base(path) != "com.example.cinematic-generate@1.0.0.json" {
		t.Fatalf("installed=%t path=%q", installed, path)
	}
	if _, installed, err := installWorkflowNodeManifest(dir, parsed); err != nil || installed {
		t.Fatalf("idempotent install installed=%t err=%v", installed, err)
	}
	changed := parsed
	changed.Manifest.DisplayName = "被篡改的同版本"
	if _, _, err := installWorkflowNodeManifest(dir, changed); err == nil {
		t.Fatal("same node version was overwritten")
	}

	newer := parsed
	newer.Manifest.Version = "1.10.0"
	newer.Template.TypeVersion = newer.Manifest.Version
	if _, _, err := installWorkflowNodeManifest(dir, newer); err != nil {
		t.Fatalf("install newer workflow node: %v", err)
	}
	older := parsed
	older.Manifest.Version = "1.2.0"
	older.Template.TypeVersion = older.Manifest.Version
	if _, _, err := installWorkflowNodeManifest(dir, older); err != nil {
		t.Fatalf("install older workflow node: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken.json"), []byte(`{"format":"wrong"}`), 0o600); err != nil {
		t.Fatalf("write broken manifest: %v", err)
	}
	templates, warnings := loadWorkflowNodeTemplates(dir)
	if len(templates) != 1 || templates[0].TypeVersion != "1.10.0" {
		t.Fatalf("templates=%+v", templates)
	}
	if len(warnings) != 1 {
		t.Fatalf("warnings=%v want one invalid-file warning", warnings)
	}
}

func TestCustomWorkflowNodeAddDuplicatePersistAndExecuteWithoutInstalledManifest(t *testing.T) {
	parsed := mustParseWorkflowNodeManifest(t, validWorkflowNodeManifestJSON)
	catalog := append(workflowNodeCatalog(), parsed.Template)
	graph, customID, err := addWorkflowNodeInstanceFromCatalog(defaultWorkflowGraph(), parsed.Template.TypeID, catalog)
	if err != nil {
		t.Fatalf("add custom node: %v", err)
	}
	if customID != parsed.Template.TypeID {
		t.Fatalf("custom node id=%q", customID)
	}
	custom, _ := graph.node(customID)
	if custom.Properties[workflowPropertyImageModel] != "gpt-image-2" {
		t.Fatalf("custom defaults=%v", custom.Properties)
	}
	duplicated, duplicateID, err := duplicateWorkflowNode(graph, customID)
	if err != nil {
		t.Fatalf("duplicate custom node: %v", err)
	}
	duplicate, ok := duplicated.node(duplicateID)
	if !ok || duplicate.TypeID != custom.TypeID || duplicate.TypeVersion != custom.TypeVersion || duplicate.Kind != custom.Kind {
		t.Fatalf("duplicate=%+v ok=%t", duplicate, ok)
	}

	executable := defaultWorkflowGraph()
	for index := range executable.Nodes {
		if executable.Nodes[index].ID != "generate" {
			continue
		}
		position := executable.Nodes[index].Position
		custom.ID = "generate"
		custom.Position = position
		executable.Nodes[index] = custom
	}
	document := desktopWorkflowGraph(executable)
	restored := workflowGraphFromDesktop(document)
	restoredGenerate, ok := restored.node("generate")
	if !ok || restoredGenerate.TypeID != parsed.Manifest.ID || restoredGenerate.TypeVersion != parsed.Manifest.Version || restoredGenerate.Category != parsed.Manifest.Category {
		t.Fatalf("restored custom generate=%+v ok=%t", restoredGenerate, ok)
	}
	data, err := marshalWorkflowDocument("Custom", desktopWorkspaceDraftForTest(), executable)
	if err != nil {
		t.Fatalf("marshal custom workflow: %v", err)
	}
	_, imported, err := parseWorkflowDocument(data)
	if err != nil {
		t.Fatalf("parse custom workflow without catalog: %v", err)
	}
	plan, err := buildWorkflowExecutionPlan(imported, "", false)
	if err != nil {
		t.Fatalf("build custom execution plan: %v", err)
	}
	if plan.Generate.TypeID != parsed.Manifest.ID || plan.Generate.Kind != workflowNodeGenerate {
		t.Fatalf("custom execution plan generate=%+v", plan.Generate)
	}
}

func desktopWorkspaceDraftForTest() desktopstate.WorkspaceDraft {
	return desktopstate.WorkspaceDraft{
		Mode:         "generate",
		Size:         "1536x1024",
		Quality:      "high",
		OutputFormat: "png",
	}
}

func TestWorkflowLibraryRoutesCustomCatalogAddAndKeepsManifestDefaults(t *testing.T) {
	isolateGioStableDataRoot(t)
	parsed := mustParseWorkflowNodeManifest(t, validWorkflowNodeManifestJSON)
	first := New()
	if _, _, err := installWorkflowNodeManifest(first.workflowNodeManifestDir, parsed); err != nil {
		t.Fatalf("install node for restart discovery: %v", err)
	}
	app := New()
	app.experienceMode = experienceModeWorkflow
	if len(app.workflowCustomNodeTemplates) != 1 || app.workflowCustomNodeTemplates[0].TypeID != parsed.Manifest.ID {
		t.Fatalf("restart catalog=%+v", app.workflowCustomNodeTemplates)
	}
	workspaceID := app.activeWorkspaceID
	app.workflowAddNodeButton(parsed.Manifest.ID).Click()
	gtx := layout.Context{
		Ops:         new(op.Ops),
		Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
		Constraints: layout.Exact(image.Pt(340, 760)),
	}
	app.layoutWorkflowLibrary(gtx, snapshot{}, desktopThemeSpec(desktopStyleMacOS, desktopColorModeLight))
	node, ok := app.workflowGraph(workspaceID).node(parsed.Manifest.ID)
	if !ok {
		t.Fatal("custom catalog button did not add node")
	}
	if node.TypeID != parsed.Manifest.ID || node.Properties[workflowPropertyImageModel] != "gpt-image-2" || node.Properties[workflowPropertySize] != "1536x1024" {
		t.Fatalf("added custom node=%+v", node)
	}
}
