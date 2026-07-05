//go:build windows

package backend

import (
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	sOK                         uintptr = 0x00000000
	sFalse                      uintptr = 0x00000001
	eNoInterface                uintptr = 0x80004002
	dragDropSDrop               uintptr = 0x00040100
	dragDropSCancel             uintptr = 0x00040101
	dragDropSUseDefaultCursors  uintptr = 0x00040102
	dropEffectCopy              uintptr = 0x00000001
	mouseKeyStateLeftButtonDown uintptr = 0x00000001
)

var (
	ole32                  = windows.NewLazySystemDLL("ole32.dll")
	shell32                = windows.NewLazySystemDLL("shell32.dll")
	procOleInitialize      = ole32.NewProc("OleInitialize")
	procOleUninitialize    = ole32.NewProc("OleUninitialize")
	procDoDragDrop         = ole32.NewProc("DoDragDrop")
	procSHCreateDataObject = shell32.NewProc("SHCreateDataObject")
	procILCreateFromPathW  = shell32.NewProc("ILCreateFromPathW")
	procILFree             = shell32.NewProc("ILFree")

	iidIUnknown = windows.GUID{Data1: 0x00000000, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidIDataObj = windows.GUID{Data1: 0x0000010e, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidDropSrc  = windows.GUID{Data1: 0x00000121, Data2: 0x0000, Data3: 0x0000, Data4: [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
)

type dropSourceVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
	queryContinue  uintptr
	giveFeedback   uintptr
}

type dropSource struct {
	lpVtbl *dropSourceVtbl
	refs   uint32
}

var nativeDropSourceVtbl = dropSourceVtbl{
	queryInterface: windows.NewCallback(dropSourceQueryInterface),
	addRef:         windows.NewCallback(dropSourceAddRef),
	release:        windows.NewCallback(dropSourceRelease),
	queryContinue:  windows.NewCallback(dropSourceQueryContinueDrag),
	giveFeedback:   windows.NewCallback(dropSourceGiveFeedback),
}

func beginNativeFileDrag(path string) error {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return fmt.Errorf("drag file path is empty")
	}
	pathPtr, err := windows.UTF16PtrFromString(cleanPath)
	if err != nil {
		return err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hr, _, _ := procOleInitialize.Call(0)
	if failedHRESULT(hr) {
		return fmt.Errorf("OleInitialize failed: 0x%08x", uint32(hr))
	}
	if hr == sOK || hr == sFalse {
		defer procOleUninitialize.Call()
	}

	pidl, _, _ := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(pathPtr)))
	if pidl == 0 {
		return fmt.Errorf("ILCreateFromPathW failed")
	}
	defer procILFree.Call(pidl)

	apidl := pidl
	var dataObject uintptr
	hr, _, _ = procSHCreateDataObject.Call(
		0,
		1,
		uintptr(unsafe.Pointer(&apidl)),
		0,
		uintptr(unsafe.Pointer(&iidIDataObj)),
		uintptr(unsafe.Pointer(&dataObject)),
	)
	if failedHRESULT(hr) || dataObject == 0 {
		return fmt.Errorf("SHCreateDataObject failed: 0x%08x", uint32(hr))
	}
	defer releaseIUnknown(dataObject)

	source := &dropSource{lpVtbl: &nativeDropSourceVtbl, refs: 1}
	var effect uintptr
	hr, _, _ = procDoDragDrop.Call(
		dataObject,
		uintptr(unsafe.Pointer(source)),
		dropEffectCopy,
		uintptr(unsafe.Pointer(&effect)),
	)
	runtime.KeepAlive(source)
	if hr == dragDropSDrop || hr == dragDropSCancel {
		return nil
	}
	if failedHRESULT(hr) {
		return fmt.Errorf("DoDragDrop failed: 0x%08x", uint32(hr))
	}
	return nil
}

func dropSourceQueryInterface(this uintptr, riid uintptr, ppv uintptr) uintptr {
	if ppv == 0 {
		return eNoInterface
	}
	requested := (*windows.GUID)(unsafe.Pointer(riid))
	if *requested == iidIUnknown || *requested == iidDropSrc {
		*(*uintptr)(unsafe.Pointer(ppv)) = this
		dropSourceAddRef(this)
		return sOK
	}
	*(*uintptr)(unsafe.Pointer(ppv)) = 0
	return eNoInterface
}

func dropSourceAddRef(this uintptr) uintptr {
	source := (*dropSource)(unsafe.Pointer(this))
	return uintptr(atomic.AddUint32(&source.refs, 1))
}

func dropSourceRelease(this uintptr) uintptr {
	source := (*dropSource)(unsafe.Pointer(this))
	next := atomic.AddUint32(&source.refs, ^uint32(0))
	return uintptr(next)
}

func dropSourceQueryContinueDrag(_ uintptr, escapePressed uintptr, keyState uintptr) uintptr {
	if escapePressed != 0 {
		return dragDropSCancel
	}
	if keyState&mouseKeyStateLeftButtonDown == 0 {
		return dragDropSDrop
	}
	return sOK
}

func dropSourceGiveFeedback(_ uintptr, _ uintptr) uintptr {
	return dragDropSUseDefaultCursors
}

type iUnknownVtbl struct {
	queryInterface uintptr
	addRef         uintptr
	release        uintptr
}

func releaseIUnknown(ptr uintptr) {
	if ptr == 0 {
		return
	}
	vtbl := *(**iUnknownVtbl)(unsafe.Pointer(ptr))
	syscall.SyscallN(vtbl.release, ptr)
}

func failedHRESULT(hr uintptr) bool {
	return uint32(hr)&0x80000000 != 0
}
