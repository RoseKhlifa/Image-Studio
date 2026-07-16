package ui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"image-studio/gio-client/internal/desktopstate"

	"gioui.org/widget"
)

const (
	workflowNodeManifestFormat      = "image-studio-workflow-node"
	workflowNodeManifestSchema      = 1
	maxWorkflowNodeManifestBytes    = 256 << 10
	maxInstalledWorkflowNodeFiles   = 512
	workflowNodeManifestDirectory   = "workflow-nodes"
	workflowNodeManifestDescription = 240
)

var workflowNodeVersionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)

type workflowNodeManifest struct {
	Format        string            `json:"format"`
	SchemaVersion int               `json:"schemaVersion"`
	ID            string            `json:"id"`
	Version       string            `json:"version"`
	DisplayName   string            `json:"displayName"`
	Description   string            `json:"description,omitempty"`
	Category      string            `json:"category,omitempty"`
	Operator      string            `json:"operator"`
	Defaults      map[string]string `json:"defaults,omitempty"`
}

type parsedWorkflowNodeManifest struct {
	Manifest workflowNodeManifest
	Template workflowNodeModel
}

func parseWorkflowNodeManifest(data []byte) (parsedWorkflowNodeManifest, error) {
	if len(data) == 0 {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("节点清单为空")
	}
	if len(data) > maxWorkflowNodeManifestBytes {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("节点清单超过 %d KiB 限制", maxWorkflowNodeManifestBytes>>10)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var manifest workflowNodeManifest
	if err := decoder.Decode(&manifest); err != nil {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("解析节点清单: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return parsedWorkflowNodeManifest{}, err
	}
	manifest.Format = strings.TrimSpace(manifest.Format)
	manifest.ID = strings.TrimSpace(manifest.ID)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.DisplayName = strings.TrimSpace(manifest.DisplayName)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Category = strings.TrimSpace(manifest.Category)
	manifest.Operator = strings.TrimSpace(manifest.Operator)
	if manifest.Format != workflowNodeManifestFormat {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("不支持的节点清单格式 %q", manifest.Format)
	}
	if manifest.SchemaVersion != workflowNodeManifestSchema {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("不支持的节点清单版本 %d", manifest.SchemaVersion)
	}
	if err := validateWorkflowNodeTypeID(manifest.ID); err != nil {
		return parsedWorkflowNodeManifest{}, err
	}
	if !validWorkflowNodeVersion(manifest.Version) {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("节点版本 %q 不是有效的语义化版本", manifest.Version)
	}
	if err := validateWorkflowNodeLabel("显示名称", manifest.DisplayName, 64, true); err != nil {
		return parsedWorkflowNodeManifest{}, err
	}
	if err := validateWorkflowNodeLabel("说明", manifest.Description, workflowNodeManifestDescription, false); err != nil {
		return parsedWorkflowNodeManifest{}, err
	}
	if err := validateWorkflowNodeLabel("分类", manifest.Category, 64, false); err != nil {
		return parsedWorkflowNodeManifest{}, err
	}
	kind := workflowNodeKind(manifest.Operator)
	base, ok := workflowNodeTemplateByKind(kind)
	if !ok {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("节点算子 %q 不在受信任目录中", manifest.Operator)
	}
	for key := range manifest.Defaults {
		if !workflowNodePropertyAllowed(kind, key) {
			return parsedWorkflowNodeManifest{}, fmt.Errorf("%s 算子不支持默认属性 %q", manifest.Operator, key)
		}
	}
	base.ID = manifest.ID
	base.TypeID = manifest.ID
	base.TypeVersion = manifest.Version
	base.Kind = kind
	base.Title = manifest.DisplayName
	base.Subtitle = chooseNonEmpty(manifest.Description, base.Subtitle)
	base.Category = chooseNonEmpty(manifest.Category, "自定义/"+workflowNodeOperatorName(kind))
	base.Properties = mergeWorkflowProperties(base.Properties, manifest.Defaults)
	if err := validateWorkflowNodeDefaults(base); err != nil {
		return parsedWorkflowNodeManifest{}, fmt.Errorf("节点默认属性无效: %w", err)
	}
	manifest.Category = base.Category
	manifest.Defaults = cloneWorkflowProperties(manifest.Defaults)
	return parsedWorkflowNodeManifest{Manifest: manifest, Template: base}, nil
}

func validWorkflowNodeVersion(version string) bool {
	if len(version) == 0 || len(version) > 64 {
		return false
	}
	match := workflowNodeVersionPattern.FindStringSubmatch(version)
	if match == nil {
		return false
	}
	if match[4] != "" {
		for _, identifier := range strings.Split(match[4], ".") {
			if len(identifier) > 1 && identifier[0] == '0' && workflowNodeNumericIdentifier(identifier) {
				return false
			}
		}
	}
	return true
}

func requireJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return fmt.Errorf("解析节点清单: %w", err)
	}
	return fmt.Errorf("节点清单只能包含一个 JSON 对象")
}

func validateWorkflowNodeTypeID(typeID string) error {
	if len(typeID) < 5 || len(typeID) > 160 {
		return fmt.Errorf("节点 ID 长度必须为 5 到 160 个字符")
	}
	segments := strings.Split(typeID, ".")
	if len(segments) < 3 {
		return fmt.Errorf("节点 ID %q 必须使用反向域名命名，例如 com.example.my-node", typeID)
	}
	for _, segment := range segments {
		if segment == "" || len(segment) > 63 || segment[0] == '-' || segment[len(segment)-1] == '-' {
			return fmt.Errorf("节点 ID %q 包含无效的命名段", typeID)
		}
		for _, r := range segment {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("节点 ID %q 只能使用小写字母、数字、点和连字符", typeID)
			}
		}
	}
	for _, builtIn := range workflowNodeCatalog() {
		if typeID == workflowNodeTypeID(builtIn) {
			return fmt.Errorf("节点 ID %q 为内置类型保留", typeID)
		}
	}
	return nil
}

func validateWorkflowNodeLabel(name string, value string, limit int, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("节点%s不能为空", name)
		}
		return nil
	}
	if len([]rune(value)) > limit {
		return fmt.Errorf("节点%s不能超过 %d 个字符", name, limit)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("节点%s不能包含控制字符", name)
		}
	}
	return nil
}

func workflowNodePropertyAllowed(kind workflowNodeKind, key string) bool {
	key = strings.TrimSpace(key)
	for _, allowed := range workflowNodePropertyKeys(kind) {
		if key == allowed {
			return true
		}
	}
	return false
}

func workflowNodePropertyKeys(kind workflowNodeKind) []string {
	switch kind {
	case workflowNodePrompt:
		return []string{workflowPropertyPrompt, workflowPropertyNegative, workflowPropertyStyleTag}
	case workflowNodeSource:
		return []string{workflowPropertySourcePaths}
	case workflowNodeGenerate:
		return []string{workflowPropertyMode, workflowPropertyQuality, workflowPropertySize, workflowPropertyImageModel, workflowPropertyBatchCount}
	case workflowNodePreview:
		return []string{workflowPropertyPartialImages}
	case workflowNodeExport:
		return []string{workflowPropertyOutputFormat, workflowPropertyOutputDir}
	default:
		return nil
	}
}

func validateWorkflowNodeDefaults(node workflowNodeModel) error {
	for key := range node.Properties {
		if !workflowNodePropertyAllowed(node.Kind, key) {
			return fmt.Errorf("不支持属性 %q", key)
		}
	}
	switch node.Kind {
	case workflowNodePrompt, workflowNodeSource:
		return nil
	case workflowNodeGenerate:
		if !workflowChoiceContains(modeChoices, strings.TrimSpace(node.Properties[workflowPropertyMode])) {
			return fmt.Errorf("模式无效")
		}
		if !workflowChoiceContains(qualityChoices, strings.TrimSpace(node.Properties[workflowPropertyQuality])) {
			return fmt.Errorf("质量无效")
		}
		if strings.TrimSpace(node.Properties[workflowPropertySize]) == "" {
			return fmt.Errorf("缺少尺寸")
		}
		if strings.TrimSpace(node.Properties[workflowPropertyImageModel]) == "" {
			return fmt.Errorf("缺少图像模型")
		}
		count, err := strconv.Atoi(strings.TrimSpace(node.Properties[workflowPropertyBatchCount]))
		if err != nil || count < 1 || count > 9 {
			return fmt.Errorf("生成张数必须为 1 到 9")
		}
	case workflowNodePreview:
		if !workflowChoiceContains(partialPreviewChoices, strings.TrimSpace(node.Properties[workflowPropertyPartialImages])) {
			return fmt.Errorf("预览帧数无效")
		}
	case workflowNodeExport:
		if !workflowChoiceContains(formatChoices, strings.TrimSpace(node.Properties[workflowPropertyOutputFormat])) {
			return fmt.Errorf("输出格式无效")
		}
		if strings.TrimSpace(node.Properties[workflowPropertyOutputDir]) == "" {
			return fmt.Errorf("缺少输出目录")
		}
	default:
		return fmt.Errorf("未知算子 %q", node.Kind)
	}
	return nil
}

func workflowNodeOperatorName(kind workflowNodeKind) string {
	switch kind {
	case workflowNodePrompt:
		return "提示词"
	case workflowNodeSource:
		return "输入"
	case workflowNodeGenerate:
		return "生成"
	case workflowNodePreview:
		return "预览"
	case workflowNodeExport:
		return "导出"
	default:
		return string(kind)
	}
}

func workflowNodeManifestDir(store *desktopstate.Store) string {
	if store == nil || strings.TrimSpace(store.Path()) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(store.Path()), workflowNodeManifestDirectory)
}

func canonicalWorkflowNodeManifestPath(dir string, manifest workflowNodeManifest) (string, error) {
	if err := validateWorkflowNodeTypeID(manifest.ID); err != nil {
		return "", err
	}
	if !validWorkflowNodeVersion(manifest.Version) {
		return "", fmt.Errorf("节点版本 %q 无效", manifest.Version)
	}
	dir = filepath.Clean(strings.TrimSpace(dir))
	if dir == "" || dir == "." {
		return "", fmt.Errorf("自定义节点目录不可用")
	}
	name := manifest.ID + "@" + manifest.Version + ".json"
	path := filepath.Join(dir, name)
	if filepath.Dir(path) != dir {
		return "", fmt.Errorf("自定义节点路径越界")
	}
	return path, nil
}

func installWorkflowNodeManifest(dir string, parsed parsedWorkflowNodeManifest) (string, bool, error) {
	path, err := canonicalWorkflowNodeManifestPath(dir, parsed.Manifest)
	if err != nil {
		return "", false, err
	}
	canonical, err := json.MarshalIndent(parsed.Manifest, "", "  ")
	if err != nil {
		return "", false, err
	}
	canonical = append(canonical, '\n')
	if existing, readErr := os.ReadFile(path); readErr == nil {
		existingParsed, parseErr := parseWorkflowNodeManifest(existing)
		if parseErr == nil {
			existingCanonical, _ := json.MarshalIndent(existingParsed.Manifest, "", "  ")
			existingCanonical = append(existingCanonical, '\n')
			if bytes.Equal(existingCanonical, canonical) {
				return path, false, nil
			}
		}
		return "", false, fmt.Errorf("节点 %s@%s 已安装且内容不同；同一版本不可覆盖", parsed.Manifest.ID, parsed.Manifest.Version)
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return "", false, readErr
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", false, err
	}
	temp, err := os.CreateTemp(dir, ".workflow-node-*.tmp")
	if err != nil {
		return "", false, err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return "", false, err
	}
	if _, err := temp.Write(canonical); err != nil {
		temp.Close()
		return "", false, err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return "", false, err
	}
	if err := temp.Close(); err != nil {
		return "", false, err
	}
	if err := os.Link(tempName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return "", false, fmt.Errorf("节点 %s@%s 已被并发安装，请重新导入确认", parsed.Manifest.ID, parsed.Manifest.Version)
		}
		return "", false, err
	}
	return path, true, nil
}

func readWorkflowNodeManifestFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxWorkflowNodeManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxWorkflowNodeManifestBytes {
		return nil, fmt.Errorf("节点清单超过 %d KiB 限制", maxWorkflowNodeManifestBytes>>10)
	}
	return data, nil
}

func loadWorkflowNodeTemplates(dir string) ([]workflowNodeModel, []error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, []error{err}
	}
	templates := map[string]workflowNodeModel{}
	warnings := make([]error, 0)
	manifestFiles := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		manifestFiles++
		if manifestFiles > maxInstalledWorkflowNodeFiles {
			warnings = append(warnings, fmt.Errorf("自定义节点目录超过 %d 个清单限制，其余文件已忽略", maxInstalledWorkflowNodeFiles))
			break
		}
		if entry.Type()&os.ModeSymlink != 0 {
			warnings = append(warnings, fmt.Errorf("%s: 忽略符号链接节点清单", entry.Name()))
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, readErr := readWorkflowNodeManifestFile(path)
		if readErr != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", entry.Name(), readErr))
			continue
		}
		parsed, parseErr := parseWorkflowNodeManifest(data)
		if parseErr != nil {
			warnings = append(warnings, fmt.Errorf("%s: %w", entry.Name(), parseErr))
			continue
		}
		current, exists := templates[parsed.Manifest.ID]
		if !exists || compareWorkflowNodeVersions(parsed.Template.TypeVersion, current.TypeVersion) > 0 {
			templates[parsed.Manifest.ID] = parsed.Template
		}
	}
	out := make([]workflowNodeModel, 0, len(templates))
	for _, template := range templates {
		out = append(out, template)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category != out[j].Category {
			return out[i].Category < out[j].Category
		}
		if out[i].Title != out[j].Title {
			return out[i].Title < out[j].Title
		}
		return out[i].TypeID < out[j].TypeID
	})
	return out, warnings
}

func compareWorkflowNodeVersions(left string, right string) int {
	leftMatch := workflowNodeVersionPattern.FindStringSubmatch(left)
	rightMatch := workflowNodeVersionPattern.FindStringSubmatch(right)
	if leftMatch == nil || rightMatch == nil {
		return strings.Compare(left, right)
	}
	for index := 1; index <= 3; index++ {
		if compared := compareWorkflowNodeNumericIdentifiers(leftMatch[index], rightMatch[index]); compared != 0 {
			return compared
		}
	}
	leftPre := leftMatch[4]
	rightPre := rightMatch[4]
	if leftPre == rightPre {
		return 0
	}
	if leftPre == "" {
		return 1
	}
	if rightPre == "" {
		return -1
	}
	return compareWorkflowNodePrerelease(leftPre, rightPre)
}

func compareWorkflowNodePrerelease(left string, right string) int {
	leftParts := strings.Split(left, ".")
	rightParts := strings.Split(right, ".")
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		if leftParts[index] == rightParts[index] {
			continue
		}
		leftNumeric := workflowNodeNumericIdentifier(leftParts[index])
		rightNumeric := workflowNodeNumericIdentifier(rightParts[index])
		switch {
		case leftNumeric && rightNumeric:
			return compareWorkflowNodeNumericIdentifiers(leftParts[index], rightParts[index])
		case leftNumeric:
			return -1
		case rightNumeric:
			return 1
		default:
			return strings.Compare(leftParts[index], rightParts[index])
		}
	}
	if len(leftParts) < len(rightParts) {
		return -1
	}
	if len(leftParts) > len(rightParts) {
		return 1
	}
	return 0
}

func workflowNodeNumericIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func compareWorkflowNodeNumericIdentifiers(left string, right string) int {
	left = strings.TrimLeft(left, "0")
	right = strings.TrimLeft(right, "0")
	if left == "" {
		left = "0"
	}
	if right == "" {
		right = "0"
	}
	if len(left) < len(right) {
		return -1
	}
	if len(left) > len(right) {
		return 1
	}
	return strings.Compare(left, right)
}

func (a *App) workflowNodeCatalog() []workflowNodeModel {
	catalog := workflowNodeCatalog()
	for _, template := range a.workflowCustomNodeTemplates {
		template.Inputs = append([]workflowPortModel(nil), template.Inputs...)
		template.Outputs = append([]workflowPortModel(nil), template.Outputs...)
		template.Properties = cloneWorkflowProperties(template.Properties)
		catalog = append(catalog, template)
	}
	return catalog
}

func (a *App) reloadWorkflowNodeCatalog() []error {
	templates, warnings := loadWorkflowNodeTemplates(a.workflowNodeManifestDir)
	a.workflowCustomNodeTemplates = templates
	a.workflowAddNodeButtons = map[string]*widget.Clickable{}
	return warnings
}

func (a *App) importCustomWorkflowNode() {
	src, err := chooseJSONFile()
	if err != nil {
		a.appendLog("选择节点清单失败: " + err.Error())
		return
	}
	if strings.TrimSpace(src) == "" {
		return
	}
	data, err := readWorkflowNodeManifestFile(src)
	if err != nil {
		a.appendLog("读取节点清单失败: " + err.Error())
		return
	}
	parsed, err := parseWorkflowNodeManifest(data)
	if err != nil {
		a.appendLog("导入自定义节点失败: " + err.Error())
		return
	}
	path, installed, err := installWorkflowNodeManifest(a.workflowNodeManifestDir, parsed)
	if err != nil {
		a.appendLog("安装自定义节点失败: " + err.Error())
		return
	}
	for _, warning := range a.reloadWorkflowNodeCatalog() {
		a.appendLog("自定义节点目录警告: " + warning.Error())
	}
	if installed {
		a.appendLog(fmt.Sprintf("已安装自定义节点: %s %s (%s)", parsed.Manifest.DisplayName, parsed.Manifest.Version, filepath.Base(path)))
	} else {
		a.appendLog(fmt.Sprintf("自定义节点已是最新: %s %s", parsed.Manifest.DisplayName, parsed.Manifest.Version))
	}
	a.invalidateNow()
}
