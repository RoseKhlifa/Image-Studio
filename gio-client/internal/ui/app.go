package ui

import (
	"context"
	"image"
	"strconv"
	"strings"
	"sync"
	"time"

	gioCompat "image-studio/gio-client/internal/compat"
	"image-studio/gio-client/internal/kernel"
	sharedCompat "image-studio/shared/compat"

	"gioui.org/app"
	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/yuanhua/image-gptcodex/pkg/promptimport"
)

type resultState struct {
	Image         image.Image
	SavedPath     string
	RawPath       string
	RevisedPrompt string
	SourceEvent   string
	Item          sharedCompat.HistoryItem
	HasItem       bool
	Rev           int
}

type snapshot struct {
	Running                   bool
	ProcessingImageTransform  bool
	Status                    string
	Logs                      []string
	RenderBackend             string
	RenderFrameTime           time.Duration
	RenderFPS                 float64
	RenderActive              bool
	TodayHistoryCount         int
	History                   []sharedCompat.HistoryItem
	BatchResults              []sharedCompat.HistoryItem
	BatchTotal                int
	Profiles                  []sharedCompat.UpstreamProfile
	ActiveProfileID           string
	SettingsSelectedProfileID string
	SelectedHistoryID         string
	PromptHistory             []string
	PromptTemplates           []sharedCompat.PromptTemplate
	Presets                   []sharedCompat.Preset
	OptimizingPrompt          bool
	TestingUpstream           bool
	SyncingCodexConfig        bool
	LastProbeSummary          string
	LastProbeModels           []kernel.UpstreamModelDescriptor
	ActivePromptGroup         historyPromptGroup
	ActiveResultDetail        sharedCompat.HistoryItem
	HistoryTimelineOpen       bool
	Fullscreen                bool
	LastErrorMessage          string
	LastRunAvailable          bool
	LastLowFPSSnapshotPath    string
	RawResponseModalPath      string
	RawResponseModalText      string
	RawResponseModalError     string
	ResultGridOpen            bool
	Compare                   resultState
	CompareSplit              float32
	Result                    resultState
	SavePromptVisible         bool
	PromptImportVisible       bool
	PromptImportLoading       bool
	PromptImportToken         string
	PromptImportPayload       *promptimport.ImportPayload
	PromptImportResolvedSize  string
	PromptImportRegisterOpen  bool
	PromptImportRegisterBusy  bool
	PromptImportRegisterNote  string
}

type cachedImage struct {
	Image   image.Image
	Op      paint.ImageOp
	Failed  bool
	Loading bool
}

type historyItemDisplay struct {
	ShortPrompt      string
	MetaBadges       []string
	StatusMetaBadges []string
	Clock            string
	ClockPrecise     string
	RailMetaText     string
	MetaText         string
}

type historyItemDisplayCache struct {
	rev   int
	items map[string]historyItemDisplay
}

type promptSuggestionsCache struct {
	historyRev       int
	promptHistoryRev int
	items            []string
}

type historyGroupLookupCache struct {
	rev    int
	groups []historyPromptGroup
	index  map[string]int
}

type workspaceState struct {
	ID                  string
	Name                string
	Prompt              string
	NegativePrompt      string
	Mode                string
	Size                string
	Quality             string
	OutputFormat        string
	Background          string
	OutputCompression   string
	InputFidelity       string
	ImageStyle          string
	Moderation          string
	UserIdentifier      string
	PartialImages       string
	StyleTag            string
	SeedText            string
	BatchCount          int
	LoopEnabled         bool
	LoopTotalCount      int
	LoopConcurrency     int
	LoopAutoSave        bool
	LoopAutoSaveDir     string
	LoopLivePreview     bool
	BatchMode           bool
	BatchInputDir       string
	BatchOutputDir      string
	BatchRetryOnFail    bool
	BatchAutoAspect     string
	SourcePathsText     string
	ResultSavedPath     string
	ResultRawPath       string
	ResultRevisedPrompt string
	ResultSourceEvent   string
	ResultItem          sharedCompat.HistoryItem
	ResultHasItem       bool
	SelectedHistoryID   string
	BatchResultIDs      []string
	ResultGridOpen      bool
	CompareHistoryID    string
	CompareSplit        float32
}

type App struct {
	th     *material.Theme
	runner kernel.Runner

	controlsList        widget.List
	logList             widget.List
	historyList         widget.List
	historyTimelineList widget.List
	promptGroupList     widget.List
	promptHelperList    widget.List
	settingsProfileList widget.List
	settingsList        widget.List
	workspaceList       widget.List

	apiKeyInput               widget.Editor
	baseURLInput              widget.Editor
	textModelInput            widget.Editor
	imageModelInput           widget.Editor
	profileNameInput          widget.Editor
	concurrencyLimitInput     widget.Editor
	promptInput               widget.Editor
	sourcePathsInput          widget.Editor
	outputDirInput            widget.Editor
	seedInput                 widget.Editor
	negativePromptInput       widget.Editor
	partialImagesInput        widget.Editor
	outputCompressionInput    widget.Editor
	proxyURLInput             widget.Editor
	userIdentifierInput       widget.Editor
	savePromptPathInput       widget.Editor
	promptTemplateLabelInput  widget.Editor
	promptTemplateTextInput   widget.Editor
	presetNameInput           widget.Editor
	loopTotalCountInput       widget.Editor
	loopConcurrencyInput      widget.Editor
	loopAutoSaveDirInput      widget.Editor
	batchInputDirInput        widget.Editor
	batchOutputDirInput       widget.Editor
	upstreamQuickImportInput  widget.Editor
	rawResponseViewerInput    widget.Editor
	historyQueryInput         widget.Editor
	historyTimelineQueryInput widget.Editor
	workspaceNameInput        widget.Editor

	mode                 string
	api                  string
	size                 string
	quality              string
	format               string
	policy               string
	responsesTransport   string
	reasoningEffort      string
	fallbackProfileID    string
	proxy                string
	background           string
	inputFidelity        string
	imageStyle           string
	moderation           string
	styleTag             string
	protectStreamPreview bool
	autoRetryEnabled     bool
	autoRetryCount       int
	loopEnabled          bool
	loopTotalCount       int
	loopConcurrency      int
	loopAutoSave         bool
	loopAutoSaveDir      string
	loopLivePreview      bool
	batchMode            bool
	batchInputDir        string
	batchOutputDir       string
	batchRetryOnFail     bool
	batchAutoAspect      string
	themeMode            string
	fontScale            float64
	reducedEffects       bool
	imagesNewAPICompat   bool
	batchCount           int

	modeButtons                              []widget.Clickable
	apiButtons                               []widget.Clickable
	sizeButtons                              []widget.Clickable
	aspectButtons                            []widget.Clickable
	styleButtons                             []widget.Clickable
	clearStyleButton                         widget.Clickable
	randomSeedButton                         widget.Clickable
	clearSeedButton                          widget.Clickable
	batchCountButtons                        []widget.Clickable
	resolutionButtons                        []widget.Clickable
	qualityButtons                           []widget.Clickable
	formatButtons                            []widget.Clickable
	policyButtons                            []widget.Clickable
	responsesTransportButtons                []widget.Clickable
	reasoningEffortButtons                   []widget.Clickable
	proxyButtons                             []widget.Clickable
	backgroundButtons                        []widget.Clickable
	inputFidelityButtons                     []widget.Clickable
	imageStyleButtons                        []widget.Clickable
	moderationButtons                        []widget.Clickable
	composeSourceModeButtons                 []widget.Clickable
	partialPreviewButtons                    []widget.Clickable
	protectStreamPreviewButtons              []widget.Clickable
	historyModeButtons                       []widget.Clickable
	historyDateButtons                       []widget.Clickable
	historyTimelineModeButtons               []widget.Clickable
	historyTimelineDateButtons               []widget.Clickable
	runButton                                widget.Clickable
	cancelButton                             widget.Clickable
	retryLastRunButton                       widget.Clickable
	openRawResponseButton                    widget.Clickable
	openLogsRawResponseButton                widget.Clickable
	dismissErrorButton                       widget.Clickable
	closeRawResponseButton                   widget.Clickable
	copyRawResponseButton                    widget.Clickable
	clearLogButton                           widget.Clickable
	saveAsButton                             widget.Clickable
	latestResultButton                       widget.Clickable
	currentGroupButton                       widget.Clickable
	closeCompareButton                       widget.Clickable
	closeResultGridButton                    widget.Clickable
	rotateLeftButton                         widget.Clickable
	rotateRightButton                        widget.Clickable
	flipHorizontalButton                     widget.Clickable
	flipVerticalButton                       widget.Clickable
	clearCurrentButton                       widget.Clickable
	clearSourcesButton                       widget.Clickable
	addSourceFilesButton                     widget.Clickable
	addSourceStripButton                     widget.Clickable
	chooseBatchInputDirButton                widget.Clickable
	chooseBatchFilesButton                   widget.Clickable
	chooseBatchOutputDirButton               widget.Clickable
	toggleLoopButton                         widget.Clickable
	chooseLoopAutoSaveDirButton              widget.Clickable
	emptyStateImportButton                   widget.Clickable
	promptHelperButton                       widget.Clickable
	promptHelperTemplatesButton              widget.Clickable
	promptHelperPresetsButton                widget.Clickable
	promptHelperHistoryButton                widget.Clickable
	openPromptTemplateManagerButton          widget.Clickable
	openPresetManagerFromPromptButton        widget.Clickable
	newPromptTemplateButton                  widget.Clickable
	savePromptTemplateButton                 widget.Clickable
	deletePromptTemplateButton               widget.Clickable
	promptTemplateListButtons                map[string]*widget.Clickable
	openPresetManagerButton                  widget.Clickable
	saveCurrentPresetButton                  widget.Clickable
	closePresetManagerButton                 widget.Clickable
	newPresetButton                          widget.Clickable
	savePresetButton                         widget.Clickable
	overwritePresetButton                    widget.Clickable
	applyPresetButton                        widget.Clickable
	deletePresetButton                       widget.Clickable
	presetListButtons                        map[string]*widget.Clickable
	openCustomAspectRatioManagerButton       widget.Clickable
	addCustomAspectRatioButton               widget.Clickable
	deleteCustomAspectRatioButton            widget.Clickable
	customAspectRatioListButtons             map[string]*widget.Clickable
	closePromptHelperButton                  widget.Clickable
	optimizePromptButton                     widget.Clickable
	testUpstreamButton                       widget.Clickable
	settingsTestUpstreamButton               widget.Clickable
	loadUpstreamModelsButton                 widget.Clickable
	settingsImagesCompatButton               widget.Clickable
	syncCodexConfigButton                    widget.Clickable
	exportUpstreamConfigsButton              widget.Clickable
	importUpstreamConfigsButton              widget.Clickable
	openQuickImportUpstreamConfigsButton     widget.Clickable
	confirmQuickImportUpstreamConfigsButton  widget.Clickable
	closeQuickImportUpstreamConfigsButton    widget.Clickable
	historyTimelineModePickerButton          widget.Clickable
	historyTimelineDatePickerButton          widget.Clickable
	toggleAPIKeyMaskButton                   widget.Clickable
	upstreamConfigButton                     widget.Clickable
	settingsHelpButton                       widget.Clickable
	closeSettingsHelpButton                  widget.Clickable
	saveSettingsButton                       widget.Clickable
	closeGeneralSettingsButton               widget.Clickable
	copyGeneralPerformanceDiagnosticsButton  widget.Clickable
	previewCompletionSoundButton             widget.Clickable
	chooseCompletionSoundButton              widget.Clickable
	resetCompletionSoundButton               widget.Clickable
	requestCompletionNotificationButton      widget.Clickable
	generalRuntimePickerButton               widget.Clickable
	openGeneralUpstreamButton                widget.Clickable
	openGeneralOutputButton                  widget.Clickable
	chooseGeneralOutputButton                widget.Clickable
	chooseGeneralLoopAutoSaveDirButton       widget.Clickable
	useGeneralLoopOutputDirButton            widget.Clickable
	chooseGeneralBatchInputButton            widget.Clickable
	chooseGeneralBatchFilesButton            widget.Clickable
	chooseGeneralBatchOutputButton           widget.Clickable
	resetGeneralOutputButton                 widget.Clickable
	triggerGeneralHistoryMediaBackfillButton widget.Clickable
	openGeneralHistoryTimelineButton         widget.Clickable
	exportGeneralHistoryButton               widget.Clickable
	importGeneralHistoryButton               widget.Clickable
	openGeneralAboutButton                   widget.Clickable
	openGeneralUpdateButton                  widget.Clickable
	openGeneralDiagnosticsDirButton          widget.Clickable
	openGeneralLastLowFPSSnapshotButton      widget.Clickable
	clearGeneralAPIKeyButton                 widget.Clickable
	clearGeneralHistoryButton                widget.Clickable
	pruneGeneralHistoryButtons               []widget.Clickable
	openGeneralRepoButton                    widget.Clickable
	openGeneralFeedbackButton                widget.Clickable
	dismissAppUpdateButton                   widget.Clickable
	ignoreAppUpdateButton                    widget.Clickable
	openAppUpdateButton                      widget.Clickable
	closeAboutButton                         widget.Clickable
	openAboutUpdateButton                    widget.Clickable
	openAboutRepoButton                      widget.Clickable
	openAboutFeedbackButton                  widget.Clickable
	openAboutLicenseButton                   widget.Clickable
	themeButtons                             []widget.Clickable
	generalThemeButtons                      []widget.Clickable
	generalRuntimeButtons                    []widget.Clickable
	generalFontScaleButtons                  []widget.Clickable
	generalPerformanceButtons                []widget.Clickable
	generalSavePromptButtons                 []widget.Clickable
	generalCompletionSoundButtons            []widget.Clickable
	generalCompletionSoundModeButtons        []widget.Clickable
	generalCompletionNotificationButtons     []widget.Clickable
	generalCleanupPreviewCacheButtons        []widget.Clickable
	generalProtectStreamPreviewButtons       []widget.Clickable
	generalAutoRetryButtons                  []widget.Clickable
	generalAutoRetryCountButtons             []widget.Clickable
	generalLoopButtons                       []widget.Clickable
	generalLoopAutoSaveButtons               []widget.Clickable
	generalLoopPreviewButtons                []widget.Clickable
	generalBatchButtons                      []widget.Clickable
	generalBatchRetryButtons                 []widget.Clickable
	generalBatchAutoAspectButtons            []widget.Clickable
	generalBatchAutoAspectResolutionButtons  []widget.Clickable
	composeBatchRetryButtons                 []widget.Clickable
	composeBatchAutoAspectButtons            []widget.Clickable
	composeBatchAutoAspectResolutionButtons  []widget.Clickable
	composeLoopButtons                       []widget.Clickable
	composeLoopCountButtons                  []widget.Clickable
	composeLoopConcurrencyButtons            []widget.Clickable
	composeLoopAutoSaveButtons               []widget.Clickable
	composeLoopPreviewButtons                []widget.Clickable
	generalProxyButtons                      []widget.Clickable
	generalKeepLogsButtons                   []widget.Clickable
	headerAddWorkspaceButton                 widget.Clickable
	headerQuoteButton                        widget.Clickable
	githubButton                             widget.Clickable
	headerStarButton                         widget.Clickable
	settingsButton                           widget.Clickable
	fullscreenButton                         widget.Clickable
	resultDetailButton                       widget.Clickable
	footerOutputButton                       widget.Clickable
	footerGithubButton                       widget.Clickable
	footerFeedbackButton                     widget.Clickable
	addWorkspaceButton                       widget.Clickable
	workspaceRenameSaveButton                widget.Clickable
	workspaceRenameCancelButton              widget.Clickable
	closeSettingsButton                      widget.Clickable
	createProfileButton                      widget.Clickable
	createImagesProfileButton                widget.Clickable
	duplicateProfileButton                   widget.Clickable
	deleteProfileButton                      widget.Clickable
	settingsActivateProfileButton            widget.Clickable
	closeResultDetailButton                  widget.Clickable
	resultDetailSaveAsButton                 widget.Clickable
	resultDetailUseSourceButton              widget.Clickable
	resultDetailUsePromptButton              widget.Clickable
	resultDetailUseRevisedButton             widget.Clickable
	resultDetailOpenPathButton               widget.Clickable
	resultDetailCopyPromptButton             widget.Clickable
	resultDetailCopyRevisedButton            widget.Clickable
	resultDetailCopyPathButton               widget.Clickable
	resultDetailDeleteButton                 widget.Clickable
	composeToggleButton                      widget.Clickable
	advancedToggleButton                     widget.Clickable
	copyPerformanceDiagnosticsButton         widget.Clickable
	profilePickerButton                      widget.Clickable
	manageUpstreamButton                     widget.Clickable
	historyCollapseButton                    widget.Clickable
	closePromptGroupButton                   widget.Clickable
	openHistoryTimelineButton                widget.Clickable
	openHistoryTimelineMoreButton            widget.Clickable
	closeHistoryTimelineButton               widget.Clickable
	savePromptSaveButton                     widget.Clickable
	savePromptSkipButton                     widget.Clickable
	savePromptNeverAsk                       widget.Bool
	promptImportConfirmButton                widget.Clickable
	promptImportCloseButton                  widget.Clickable
	promptImportRegisterNowButton            widget.Clickable
	promptImportRegisterLaterButton          widget.Clickable

	mu                           sync.Mutex
	running                      bool
	cancel                       context.CancelFunc
	status                       string
	logs                         []string
	logsRev                      int
	logsSnapshotRev              int
	logsSnapshotCache            []string
	history                      []sharedCompat.HistoryItem
	profiles                     []sharedCompat.UpstreamProfile
	promptHistory                []string
	promptTemplates              []sharedCompat.PromptTemplate
	promptHistoryRev             int
	presets                      []sharedCompat.Preset
	customAspectRatios           []sharedCompat.CustomAspectRatio
	historyThumbBackfillInFlight map[string]struct{}
	activeProfileID              string
	selectedHistoryID            string
	optimizingPrompt             bool
	testingUpstream              bool
	syncingCodexConfig           bool
	processingImageTransform     bool
	lastProbeSummary             string
	lastProbeModels              []kernel.UpstreamModelDescriptor
	fullscreen                   bool
	activeResultDetail           sharedCompat.HistoryItem
	result                       resultState
	compare                      resultState
	imageOp                      paint.ImageOp
	imageOpRev                   int
	compareImageOp               paint.ImageOp
	compareImageOpRev            int
	canvasDisplayScale           float32
	imageCache                   map[string]cachedImage
	imageLoadWaiters             map[string]chan struct{}
	checkerboard                 checkerboardCache
	snapshotCache                snapshot
	snapshotReady                bool
	historyRev                   int
	batchResultsRev              int
	batchResultsKey              string
	batchResultsSnapshot         []sharedCompat.HistoryItem
	historyTodayRev              int
	historyTodayDay              string
	historyTodayCount            int
	historyPanelCache            historyPanelCache
	historyTimelineCache         historyTimelineCache
	historyGroupLookup           historyGroupLookupCache
	promptSuggestionsCache       promptSuggestionsCache
	historyItemDisplayCache      historyItemDisplayCache
	sourcePathParseCache         map[string][]string
	composeSummaryCacheKey       string
	composeSummaryCache          string
	advancedSummaryCacheKey      string
	advancedSummaryCache         string
	promptLabelCacheKey          string
	promptLabelCacheItems        []promptHelperItem
	presetLabelCacheKey          string
	presetLabelCacheItems        []promptHelperItem
	promptTextMetricsKey         string
	promptTextMetricsTrimmed     string
	promptTextMetricsLen         int
	renderBackend                string
	frameRawIntervalEMA          time.Duration
	frameRawFPS                  float64
	frameIntervalEMA             time.Duration
	frameFPS                     float64
	layoutShellEMA               time.Duration
	layoutControlsEMA            time.Duration
	layoutSubmitDockEMA          time.Duration
	layoutActionsEMA             time.Duration
	layoutPromptCardEMA          time.Duration
	layoutComposeCardEMA         time.Duration
	layoutAdvancedCardEMA        time.Duration
	layoutCanvasEMA              time.Duration
	layoutCanvasToolbarEMA       time.Duration
	layoutResultSurfaceEMA       time.Duration
	layoutCanvasStatusEMA        time.Duration
	layoutHistoryRailEMA         time.Duration
	layoutUpstreamCardEMA        time.Duration
	layoutHistorySummaryEMA      time.Duration
	layoutLatestHistoryEMA       time.Duration
	layoutHistoryResultsEMA      time.Duration
	layoutTimelineModalEMA       time.Duration
	layoutPeaks                  [layoutTimingCount]time.Duration
	frameLastAt                  time.Time
	renderActive                 bool
	lastRenderActivityAt         time.Time
	lastFrameSize                image.Point
	lowFPSLastLoggedAt           time.Time
	lowFPSStreak                 int
	lastLowFPSDiagnosticsPath    string
	lastHistoryThumbPrewarmAt    time.Time
	lastHistoryThumbPrewarmMs    time.Duration
	lastHistoryThumbPrewarmLoad  int
	lastHistoryThumbPrewarmFail  int
	lowFPSSnapshotInFlight       bool
	invalidateQueued             bool
	lastRunConfig                kernel.Config
	lastRunBatchCount            int
	lastRunValid                 bool
	lastErrorMessage             string
	rawResponseModalPath         string
	rawResponseModalText         string
	rawResponseModalError        string
	batchResultIDs               []string
	resultGridOpen               bool
	compareSplitSlider           widget.Float
	compareSplitDrag             gesture.Drag

	savePromptVisible                bool
	savePromptSuppressed             bool
	completionSound                  sharedCompat.CompletionSoundSettings
	completionNotification           sharedCompat.CompletionNotificationSettings
	completionNotificationPermission systemNotificationPermissionState
	keepLogs                         bool
	cleanupPreviewCacheOnExit        bool
	windowFocused                    bool
	kernelRuntimeMode                string
	savePromptSourcePath             string
	promptImportOpen                 bool
	promptImportLoading              bool
	promptImportToken                string
	promptImportPayload              *promptimport.ImportPayload
	promptImportResolvedSize         string
	promptImportQueue                []string
	promptImportRegisterOpen         bool
	promptImportRegisterBusy         bool
	promptImportRegisterNote         string
	composeOpen                      bool
	advancedOpen                     bool
	profilePickerOpen                bool
	historyRailCollapsed             bool
	historyModeFilter                string
	historyDateFilter                string
	historyTimelineOpen              bool
	historyTimelineModeFilter        string
	historyTimelineDateFilter        string
	historyTimelineModePickerOpen    bool
	historyTimelineDatePickerOpen    bool
	profileButtons                   map[string]*widget.Clickable
	settingsProfileButtons           map[string]*widget.Clickable
	historyButtons                   map[string]*widget.Clickable
	promptButtons                    map[string]*widget.Clickable
	sourceButtons                    map[string]*widget.Clickable
	historyActionButtons             map[string]*widget.Clickable
	expandedPromptGroups             map[string]bool
	promptHelperOpen                 bool
	promptHelperTab                  string
	promptTemplateManagerOpen        bool
	selectedPromptTemplateID         string
	presetManagerOpen                bool
	selectedPresetID                 string
	customAspectRatioManagerOpen     bool
	customAspectWidthInput           widget.Editor
	customAspectHeightInput          widget.Editor
	selectedCustomAspectRatioID      string
	activePromptGroup                historyPromptGroup
	generalSettingsOpen              bool
	generalRuntimePickerOpen         bool
	aboutModalOpen                   bool
	appUpdateChecking                bool
	appUpdateModalOpen               bool
	settingsModalOpen                bool
	settingsHelpOpen                 bool
	upstreamQuickImportOpen          bool
	settingsSelectedProfileID        string
	apiKeyVisible                    bool
	appUpdateInfo                    *appUpdateInfo
	ignoredReleaseTag                string
	workspaces                       []workspaceState
	activeWorkspaceID                string
	workspaceButtons                 map[string]*widget.Clickable
	closeWorkspaceButtons            map[string]*widget.Clickable
	workspaceRenameID                string
	workspaceLastClickID             string
	workspaceLastClickAt             time.Time
	headerQuoteIndex                 int

	invalidate func()
	window     *app.Window
}

func New() *App {
	cfg := kernel.DefaultConfig()
	compatState, compatPath, compatErr := gioCompat.LoadState()
	if compatErr == nil {
		cfg = gioCompat.ConfigFromState(cfg, compatState)
	}
	themeMode := normalizeThemeMode(compatState.Settings.Theme)
	fontScale := normalizeFontScale(compatState.Settings.FontScale)
	fluent = themePalette(resolveThemeMode(themeMode))
	th := material.NewTheme()
	collection := bundledFontCollection()
	if len(collection) > 0 {
		th.Shaper = text.NewShaper(text.WithCollection(collection))
	} else {
		th.Shaper = text.NewShaper()
	}
	th.Face = uiSansTypeface
	th.Palette = material.Palette{
		Bg:         fluent.bg,
		Fg:         fluent.text,
		ContrastBg: fluent.accent,
		ContrastFg: fluent.white,
	}
	th.TextSize = unit.Sp(float32(14) * float32(fontScale))
	a := &App{
		th:                                      th,
		runner:                                  kernel.Runner{},
		mode:                                    string(cfg.Mode),
		api:                                     string(cfg.APIMode),
		size:                                    cfg.Size,
		quality:                                 cfg.Quality,
		format:                                  cfg.OutputFormat,
		policy:                                  string(cfg.RequestPolicy),
		responsesTransport:                      normalizeProfileResponsesTransport(string(cfg.ResponsesTransport)),
		reasoningEffort:                         normalizeReasoningEffort(cfg.ReasoningEffort),
		proxy:                                   cfg.ProxyMode,
		background:                              cfg.Background,
		inputFidelity:                           cfg.InputFidelity,
		imageStyle:                              cfg.ImageStyle,
		moderation:                              cfg.Moderation,
		styleTag:                                "",
		protectStreamPreview:                    cfg.ProtectStreamPreview,
		autoRetryEnabled:                        cfg.AutoRetryEnabled,
		autoRetryCount:                          normalizeAutoRetryCount(cfg.AutoRetryCount),
		loopEnabled:                             false,
		loopTotalCount:                          10,
		loopConcurrency:                         2,
		loopAutoSave:                            false,
		loopAutoSaveDir:                         "",
		loopLivePreview:                         true,
		batchMode:                               false,
		batchInputDir:                           "",
		batchOutputDir:                          "",
		batchRetryOnFail:                        false,
		batchAutoAspect:                         "",
		themeMode:                               themeMode,
		fontScale:                               fontScale,
		reducedEffects:                          compatState.Settings.ReducedEffects,
		imagesNewAPICompat:                      cfg.ImagesNewAPICompat,
		kernelRuntimeMode:                       normalizeKernelRuntimeMode(compatState.Settings.KernelRuntimeMode),
		completionSound:                         normaliseCompletionSoundSettings(compatState.Settings.CompletionSound),
		completionNotification:                  normaliseCompletionNotificationSettings(compatState.Settings.CompletionNotification),
		completionNotificationPermission:        readSystemNotificationPermission(),
		cleanupPreviewCacheOnExit:               compatState.Settings.CleanupPreviewCacheOnExit,
		windowFocused:                           true,
		ignoredReleaseTag:                       strings.TrimSpace(compatState.Settings.IgnoredReleaseTag),
		batchCount:                              1,
		themeButtons:                            make([]widget.Clickable, 3),
		generalThemeButtons:                     make([]widget.Clickable, 3),
		generalRuntimeButtons:                   make([]widget.Clickable, 3),
		generalFontScaleButtons:                 make([]widget.Clickable, 3),
		generalPerformanceButtons:               make([]widget.Clickable, 2),
		generalSavePromptButtons:                make([]widget.Clickable, 2),
		generalCompletionSoundButtons:           make([]widget.Clickable, 2),
		generalCompletionSoundModeButtons:       make([]widget.Clickable, 2),
		generalCompletionNotificationButtons:    make([]widget.Clickable, 2),
		generalCleanupPreviewCacheButtons:       make([]widget.Clickable, 2),
		generalProtectStreamPreviewButtons:      make([]widget.Clickable, 2),
		generalAutoRetryButtons:                 make([]widget.Clickable, 2),
		generalAutoRetryCountButtons:            make([]widget.Clickable, 5),
		generalLoopButtons:                      make([]widget.Clickable, 2),
		generalLoopAutoSaveButtons:              make([]widget.Clickable, 2),
		generalLoopPreviewButtons:               make([]widget.Clickable, 2),
		generalBatchButtons:                     make([]widget.Clickable, 2),
		generalBatchRetryButtons:                make([]widget.Clickable, 2),
		generalBatchAutoAspectButtons:           make([]widget.Clickable, 2),
		generalBatchAutoAspectResolutionButtons: make([]widget.Clickable, 5),
		composeBatchRetryButtons:                make([]widget.Clickable, 2),
		composeBatchAutoAspectButtons:           make([]widget.Clickable, 2),
		composeBatchAutoAspectResolutionButtons: make([]widget.Clickable, 5),
		composeLoopButtons:                      make([]widget.Clickable, 2),
		composeLoopCountButtons:                 make([]widget.Clickable, 5),
		composeLoopConcurrencyButtons:           make([]widget.Clickable, 4),
		composeLoopAutoSaveButtons:              make([]widget.Clickable, 2),
		composeLoopPreviewButtons:               make([]widget.Clickable, 2),
		generalProxyButtons:                     make([]widget.Clickable, len(proxyChoices)),
		generalKeepLogsButtons:                  make([]widget.Clickable, 2),
		pruneGeneralHistoryButtons:              make([]widget.Clickable, 2),
		modeButtons:                             make([]widget.Clickable, len(modeChoices)),
		apiButtons:                              make([]widget.Clickable, len(apiChoices)),
		sizeButtons:                             make([]widget.Clickable, len(sizeChoices)),
		aspectButtons:                           make([]widget.Clickable, len(aspectChoices)+len(compatState.Settings.CustomAspectRatios)+24),
		styleButtons:                            make([]widget.Clickable, len(styleChoices)),
		batchCountButtons:                       make([]widget.Clickable, len(batchCountChoices)),
		resolutionButtons:                       make([]widget.Clickable, len(resolutionChoices)),
		qualityButtons:                          make([]widget.Clickable, len(qualityChoices)),
		formatButtons:                           make([]widget.Clickable, len(formatChoices)),
		policyButtons:                           make([]widget.Clickable, len(policyChoices)),
		responsesTransportButtons:               make([]widget.Clickable, len(responsesTransportChoices)),
		reasoningEffortButtons:                  make([]widget.Clickable, len(reasoningEffortChoices)),
		proxyButtons:                            make([]widget.Clickable, len(proxyChoices)),
		backgroundButtons:                       make([]widget.Clickable, len(backgroundChoices)),
		inputFidelityButtons:                    make([]widget.Clickable, len(inputFidelityChoices)),
		imageStyleButtons:                       make([]widget.Clickable, len(imageStyleChoices)),
		moderationButtons:                       make([]widget.Clickable, len(moderationChoices)),
		composeSourceModeButtons:                make([]widget.Clickable, 2),
		partialPreviewButtons:                   make([]widget.Clickable, len(partialPreviewChoices)),
		protectStreamPreviewButtons:             make([]widget.Clickable, 2),
		historyModeButtons:                      make([]widget.Clickable, 3),
		historyDateButtons:                      make([]widget.Clickable, 3),
		historyTimelineModeButtons:              make([]widget.Clickable, 3),
		historyTimelineDateButtons:              make([]widget.Clickable, 3),
		status:                                  "Gio 原生客户端就绪",
		logs:                                    []string{"独立 Gio 高性能测试客户端已启动。"},
		logsRev:                                 1,
		logsSnapshotRev:                         -1,
		history:                                 append([]sharedCompat.HistoryItem(nil), compatState.History...),
		profiles:                                append([]sharedCompat.UpstreamProfile(nil), compatState.Profiles...),
		promptHistory:                           append([]string(nil), compatState.Settings.PromptHistory...),
		promptTemplates:                         append([]sharedCompat.PromptTemplate(nil), compatState.Settings.PromptTemplates...),
		promptHistoryRev:                        1,
		presets:                                 append([]sharedCompat.Preset(nil), compatState.Settings.Presets...),
		customAspectRatios:                      append([]sharedCompat.CustomAspectRatio(nil), compatState.Settings.CustomAspectRatios...),
		savePromptSuppressed:                    gioCompat.SavePromptSuppressed(compatState),
		keepLogs:                                compatState.Settings.KeepLogs,
		promptImportResolvedSize:                "auto",
		imageCache:                              map[string]cachedImage{},
		historyRev:                              1,
		composeOpen:                             false,
		advancedOpen:                            false,
		profilePickerOpen:                       false,
		historyRailCollapsed:                    false,
		historyModeFilter:                       "all",
		historyDateFilter:                       "all",
		historyTimelineModeFilter:               "all",
		historyTimelineDateFilter:               "all",
		profileButtons:                          map[string]*widget.Clickable{},
		settingsProfileButtons:                  map[string]*widget.Clickable{},
		historyButtons:                          map[string]*widget.Clickable{},
		promptButtons:                           map[string]*widget.Clickable{},
		promptTemplateListButtons:               map[string]*widget.Clickable{},
		presetListButtons:                       map[string]*widget.Clickable{},
		customAspectRatioListButtons:            map[string]*widget.Clickable{},
		sourceButtons:                           map[string]*widget.Clickable{},
		historyActionButtons:                    map[string]*widget.Clickable{},
		workspaceButtons:                        map[string]*widget.Clickable{},
		closeWorkspaceButtons:                   map[string]*widget.Clickable{},
		expandedPromptGroups:                    map[string]bool{},
		promptHelperOpen:                        false,
		promptHelperTab:                         "templates",
		headerQuoteIndex:                        initialHeaderQuoteIndex(time.Now()),
	}
	if profile, ok := gioCompat.ActiveProfile(compatState); ok {
		a.activeProfileID = profile.ID
		a.profileNameInput.SetText(strings.TrimSpace(profile.Name))
		a.fallbackProfileID = strings.TrimSpace(profile.FallbackProfileID)
		if profile.ConcurrencyLimit > 0 {
			a.concurrencyLimitInput.SetText(strconv.Itoa(profile.ConcurrencyLimit))
		}
	}
	a.savePromptNeverAsk.Value = a.savePromptSuppressed
	if compatPath != "" {
		a.appendLogLocked("兼容状态文件: " + compatPath)
	}
	if compatErr != nil {
		a.appendLogLocked("兼容状态读取失败: " + compatErr.Error())
	}
	a.controlsList.List.Axis = layout.Vertical
	a.logList.List.Axis = layout.Vertical
	a.historyList.List.Axis = layout.Vertical
	a.historyTimelineList.List.Axis = layout.Vertical
	a.promptGroupList.List.Axis = layout.Vertical
	a.promptHelperList.List.Axis = layout.Vertical
	a.settingsProfileList.List.Axis = layout.Vertical
	a.settingsList.List.Axis = layout.Vertical
	a.workspaceList.List.Axis = layout.Horizontal
	a.compareSplitSlider.Value = 0.5
	a.configureEditors(cfg)
	a.historyQueryInput.SingleLine = true
	a.historyTimelineQueryInput.SingleLine = true
	a.runStartupHistoryThumbPrewarm()
	a.startHistoryPreviewWarmup()
	if latest, ok := newestHistoryItem(a.history); ok {
		a.prefillControlsFromHistoryItem(latest)
		if err := a.loadHistoryPreview(latest, false); err != nil && !isMissingPreview(err) {
			a.appendLogLocked("载入最近历史失败: " + err.Error())
		}
	}
	a.initWorkspaces()
	a.scheduleHistoryThumbPrewarm(historyThumbPrewarmDelay)
	a.scheduleHistoryThumbBackfill(historyBackfillStartupDelay)
	a.schedulePromptImportRegistrationCheck()
	return a
}

func (a *App) configureEditors(cfg kernel.Config) {
	singleLine := []*widget.Editor{
		&a.apiKeyInput,
		&a.baseURLInput,
		&a.textModelInput,
		&a.imageModelInput,
		&a.profileNameInput,
		&a.concurrencyLimitInput,
		&a.outputDirInput,
		&a.seedInput,
		&a.partialImagesInput,
		&a.outputCompressionInput,
		&a.proxyURLInput,
		&a.userIdentifierInput,
		&a.savePromptPathInput,
		&a.promptTemplateLabelInput,
		&a.presetNameInput,
		&a.loopTotalCountInput,
		&a.loopConcurrencyInput,
		&a.loopAutoSaveDirInput,
		&a.batchInputDirInput,
		&a.batchOutputDirInput,
		&a.historyQueryInput,
		&a.historyTimelineQueryInput,
		&a.workspaceNameInput,
	}
	for _, editor := range singleLine {
		editor.SingleLine = true
	}
	a.apiKeyInput.Mask = '*'
	a.workspaceNameInput.Submit = true
	a.rawResponseViewerInput.ReadOnly = true
	a.seedInput.Filter = "0123456789"
	a.partialImagesInput.Filter = "0123456789"
	a.outputCompressionInput.Filter = "0123456789"
	a.concurrencyLimitInput.Filter = "0123456789"
	a.loopTotalCountInput.Filter = "0123456789"
	a.loopConcurrencyInput.Filter = "0123456789"
	a.apiKeyInput.SetText(cfg.APIKey)
	a.baseURLInput.SetText(cfg.BaseURL)
	a.textModelInput.SetText(cfg.TextModelID)
	a.imageModelInput.SetText(cfg.ImageModelID)
	a.outputDirInput.SetText(cfg.OutputDir)
	a.partialImagesInput.SetText(strconv.Itoa(cfg.PartialImages))
	a.outputCompressionInput.SetText(strconv.Itoa(cfg.OutputCompression))
	a.proxyURLInput.SetText(cfg.ProxyURL)
	a.userIdentifierInput.SetText(cfg.UserIdentifier)
	a.syncLoopInputsFromState()
	a.promptInput.SetText("")
}

func (a *App) applyRuntimeConfig(cfg kernel.Config) {
	if strings.TrimSpace(cfg.APIKey) != "" || strings.TrimSpace(a.apiKeyInput.Text()) != "" {
		a.apiKeyInput.SetText(cfg.APIKey)
	}
	a.baseURLInput.SetText(cfg.BaseURL)
	a.textModelInput.SetText(cfg.TextModelID)
	a.imageModelInput.SetText(cfg.ImageModelID)
	a.proxy = cfg.ProxyMode
	a.background = cfg.Background
	a.inputFidelity = cfg.InputFidelity
	a.imageStyle = cfg.ImageStyle
	a.moderation = cfg.Moderation
	a.responsesTransport = normalizeProfileResponsesTransport(string(cfg.ResponsesTransport))
	a.reasoningEffort = normalizeReasoningEffort(cfg.ReasoningEffort)
	a.fallbackProfileID = strings.TrimSpace(cfg.FallbackProfileID)
	a.completionSound = normaliseCompletionSoundSettings(&cfg.CompletionSound)
	a.protectStreamPreview = cfg.ProtectStreamPreview
	a.autoRetryEnabled = cfg.AutoRetryEnabled
	a.autoRetryCount = normalizeAutoRetryCount(cfg.AutoRetryCount)
	a.proxyURLInput.SetText(cfg.ProxyURL)
	a.outputDirInput.SetText(cfg.OutputDir)
	a.partialImagesInput.SetText(strconv.Itoa(cfg.PartialImages))
	a.outputCompressionInput.SetText(strconv.Itoa(cfg.OutputCompression))
	a.userIdentifierInput.SetText(cfg.UserIdentifier)
	a.imagesNewAPICompat = cfg.ImagesNewAPICompat
	if strings.TrimSpace(cfg.OutputFormat) != "" {
		a.format = cfg.OutputFormat
	}
	if strings.TrimSpace(string(cfg.APIMode)) != "" {
		a.api = string(cfg.APIMode)
	}
	if strings.TrimSpace(string(cfg.RequestPolicy)) != "" {
		a.policy = string(cfg.RequestPolicy)
	}
}

func (a *App) Run(w *app.Window) error {
	a.window = w
	a.invalidate = w.Invalidate
	var ops op.Ops
	for {
		switch e := w.Event().(type) {
		case app.ConfigEvent:
			a.mu.Lock()
			a.windowFocused = e.Config.Focused
			a.mu.Unlock()
		case app.DestroyEvent:
			a.saveCurrentConfig()
			a.cancelRun()
			a.cleanupRuntimeArtifactsOnExit()
			return e.Err
		case app.FrameEvent:
			a.recordRenderFrame(e.Now, e.Size)
			gtx := app.NewContext(&ops, e)
			a.layout(gtx)
			e.Frame(gtx.Ops)
		}
	}
}
