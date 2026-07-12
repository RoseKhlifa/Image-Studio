// SPDX-License-Identifier: Unlicense OR MIT

//go:build darwin && !ios

package app

import "testing"

func TestFullscreenAuxiliaryCollectionBehaviorMask(t *testing.T) {
	const want = uint64(1<<1 | 1<<8)
	if got := fullscreenAuxiliaryCollectionBehaviorBits(); got != want {
		t.Fatalf("collection behavior mask=%#x want %#x", got, want)
	}
}
