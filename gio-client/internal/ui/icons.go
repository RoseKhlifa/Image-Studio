package ui

import (
	"gioui.org/widget"
	mdicons "golang.org/x/exp/shiny/materialdesign/icons"
)

var (
	uiIconAdd            = mustIcon(widget.NewIcon(mdicons.ContentAdd))
	uiIconHistory        = mustIcon(widget.NewIcon(mdicons.ActionHistory))
	uiIconCalendar       = mustIcon(widget.NewIcon(mdicons.ActionDateRange))
	uiIconPanTool        = mustIcon(widget.NewIcon(mdicons.ActionPanTool))
	uiIconCompare        = mustIcon(widget.NewIcon(mdicons.ActionCompareArrows))
	uiIconSearch         = mustIcon(widget.NewIcon(mdicons.ActionSearch))
	uiIconDelete         = mustIcon(widget.NewIcon(mdicons.ActionDelete))
	uiIconCopy           = mustIcon(widget.NewIcon(mdicons.ContentContentCopy))
	uiIconUndo           = mustIcon(widget.NewIcon(mdicons.ContentUndo))
	uiIconRedo           = mustIcon(widget.NewIcon(mdicons.ContentRedo))
	uiIconInfo           = mustIcon(widget.NewIcon(mdicons.ActionInfoOutline))
	uiIconGrid           = mustIcon(widget.NewIcon(mdicons.ActionViewModule))
	uiIconList           = mustIcon(widget.NewIcon(mdicons.ActionViewList))
	uiIconSpark          = mustIcon(widget.NewIcon(mdicons.ActionLightbulbOutline))
	uiIconRefresh        = mustIcon(widget.NewIcon(mdicons.ActionAutorenew))
	uiIconPlay           = mustIcon(widget.NewIcon(mdicons.AVPlayArrow))
	uiIconBuild          = mustIcon(widget.NewIcon(mdicons.ActionBuild))
	uiIconCancel         = mustIcon(widget.NewIcon(mdicons.NavigationCancel))
	uiIconCheck          = mustIcon(widget.NewIcon(mdicons.NavigationCheck))
	uiIconClear          = mustIcon(widget.NewIcon(mdicons.ContentClear))
	uiIconEdit           = mustIcon(widget.NewIcon(mdicons.EditorModeEdit))
	uiIconVisibility     = mustIcon(widget.NewIcon(mdicons.ActionVisibility))
	uiIconVisibilityOff  = mustIcon(widget.NewIcon(mdicons.ActionVisibilityOff))
	uiIconSource         = mustIcon(widget.NewIcon(mdicons.ImageAddAPhoto))
	uiIconClose          = mustIcon(widget.NewIcon(mdicons.NavigationClose))
	uiIconRotateLeft     = mustIcon(widget.NewIcon(mdicons.ImageRotateLeft))
	uiIconRotateRight    = mustIcon(widget.NewIcon(mdicons.ImageRotateRight))
	uiIconCrop           = mustIcon(widget.NewIcon(mdicons.ImageCrop))
	uiIconFlip           = mustIcon(widget.NewIcon(mdicons.ImageFlip))
	uiIconBrush          = mustIcon(widget.NewIcon(mdicons.ImageBrush))
	uiIconAnnotate       = mustIcon(widget.NewIcon(mdicons.ImageCropSquare))
	uiIconLaunch         = mustIcon(widget.NewIcon(mdicons.ActionLaunch))
	uiIconDownload       = mustIcon(widget.NewIcon(mdicons.FileFileDownload))
	uiIconSave           = mustIcon(widget.NewIcon(mdicons.ContentSave))
	uiIconSettings       = mustIcon(widget.NewIcon(mdicons.ActionSettings))
	uiIconStar           = mustIcon(widget.NewIcon(mdicons.ToggleStar))
	uiIconFolder         = mustIcon(widget.NewIcon(mdicons.FileFolder))
	uiIconFeedback       = mustIcon(widget.NewIcon(mdicons.ActionHelpOutline))
	uiIconSystem         = mustIcon(widget.NewIcon(mdicons.DeviceBrightnessAuto))
	uiIconLight          = mustIcon(widget.NewIcon(mdicons.ImageBrightness7))
	uiIconDark           = mustIcon(widget.NewIcon(mdicons.NotificationDoNotDisturb))
	uiIconPhoto          = mustIcon(widget.NewIcon(mdicons.ImagePhoto))
	uiIconMoreHoriz      = mustIcon(widget.NewIcon(mdicons.NavigationMoreHoriz))
	uiIconCollapse       = mustIcon(widget.NewIcon(mdicons.NavigationExpandLess))
	uiIconExpand         = mustIcon(widget.NewIcon(mdicons.NavigationExpandMore))
	uiIconFullscreen     = mustIcon(widget.NewIcon(mdicons.NavigationFullscreen))
	uiIconFullscreenExit = mustIcon(widget.NewIcon(mdicons.NavigationFullscreenExit))
	uiIconChevronLeft    = mustIcon(widget.NewIcon(mdicons.NavigationChevronLeft))
	uiIconChevronRight   = mustIcon(widget.NewIcon(mdicons.NavigationChevronRight))
)

func mustIcon(ic *widget.Icon, err error) *widget.Icon {
	if err != nil {
		panic(err)
	}
	return ic
}
