// SPDX-License-Identifier: Unlicense OR MIT

package app

import (
	"testing"

	"gioui.org/unit"
)

func TestFullscreenAuxiliaryOption(t *testing.T) {
	var config Config
	FullscreenAuxiliary(true)(unit.Metric{}, &config)
	if !config.FullscreenAuxiliary {
		t.Fatal("FullscreenAuxiliary(true) did not update Config")
	}
	FullscreenAuxiliary(false)(unit.Metric{}, &config)
	if config.FullscreenAuxiliary {
		t.Fatal("FullscreenAuxiliary(false) did not update Config")
	}
}
