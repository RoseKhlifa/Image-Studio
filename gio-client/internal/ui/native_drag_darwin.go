//go:build darwin && !ios

package ui

/*
#cgo CFLAGS: -x objective-c -fobjc-arc -mmacosx-version-min=10.13
#cgo LDFLAGS: -framework Cocoa -framework Foundation -mmacosx-version-min=10.13

#import <Cocoa/Cocoa.h>
#import <Foundation/Foundation.h>
#import <stdlib.h>
#import <string.h>

@interface GioClientDraggingSource : NSObject <NSDraggingSource>
@end

@implementation GioClientDraggingSource
- (NSDragOperation)draggingSession:(NSDraggingSession *)session sourceOperationMaskForDraggingContext:(NSDraggingContext)context {
	return NSDragOperationCopy;
}
- (BOOL)ignoreModifierKeysForDraggingSession:(NSDraggingSession *)session {
	return YES;
}
@end

static char *gio_client_make_drag_error(const char *message) {
	if (message == NULL) {
		return NULL;
	}
	size_t len = strlen(message);
	char *buf = (char *)malloc(len + 1);
	if (buf == NULL) {
		return NULL;
	}
	memcpy(buf, message, len + 1);
	return buf;
}

static int gio_client_begin_file_drag(uintptr_t viewPtr, const char *path, char **errOut) {
	@autoreleasepool {
		if (viewPtr == 0) {
			*errOut = gio_client_make_drag_error("empty AppKit view");
			return 1;
		}
		NSView *view = (__bridge NSView *)((void *)viewPtr);
		if (view == nil) {
			*errOut = gio_client_make_drag_error("invalid AppKit view");
			return 1;
		}
		if (path == NULL || path[0] == '\0') {
			*errOut = gio_client_make_drag_error("empty drag path");
			return 1;
		}
		NSString *filePath = [NSString stringWithUTF8String:path];
		if (filePath == nil) {
			*errOut = gio_client_make_drag_error("invalid drag path");
			return 1;
		}
		BOOL isDir = NO;
		if (![[NSFileManager defaultManager] fileExistsAtPath:filePath isDirectory:&isDir] || isDir) {
			*errOut = gio_client_make_drag_error("drag file does not exist");
			return 1;
		}

		NSWindow *window = view.window;
		if (window == nil) {
			*errOut = gio_client_make_drag_error("AppKit view has no window");
			return 1;
		}

		NSImage *icon = [[NSWorkspace sharedWorkspace] iconForFile:filePath];
		if (icon == nil) {
			icon = [NSImage imageNamed:NSImageNameMultipleDocuments];
		}
		if (icon == nil) {
			*errOut = gio_client_make_drag_error("failed to build drag icon");
			return 1;
		}
		[icon setSize:NSMakeSize(96, 96)];

		NSDraggingItem *dragItem = [[NSDraggingItem alloc] initWithPasteboardWriter:[NSURL fileURLWithPath:filePath]];
		NSPoint mouse = [view convertPoint:[window mouseLocationOutsideOfEventStream] fromView:nil];
		NSRect rect = NSMakeRect(mouse.x - 48, mouse.y - 48, 96, 96);
		[dragItem setDraggingFrame:rect contents:icon];

		NSEvent *event = [NSApp currentEvent];
		if (event == nil) {
			event = [NSEvent mouseEventWithType:NSEventTypeLeftMouseDragged
				location:[window mouseLocationOutsideOfEventStream]
				modifierFlags:0
				timestamp:[NSDate timeIntervalSinceReferenceDate]
				windowNumber:[window windowNumber]
				context:nil
				eventNumber:0
				clickCount:1
				pressure:1.0];
		}
		if (event == nil) {
			*errOut = gio_client_make_drag_error("failed to construct drag event");
			return 1;
		}
		GioClientDraggingSource *source = [GioClientDraggingSource new];
		[view beginDraggingSessionWithItems:@[dragItem] event:event source:source];
		return 0;
	}
}

static void gio_client_free_drag_error(char *value) {
	if (value != NULL) {
		free(value);
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

func beginNativeFileDragDarwin(view uintptr, path string) error {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	var errPtr *C.char
	code := C.gio_client_begin_file_drag(C.uintptr_t(view), cPath, &errPtr)
	defer func() {
		if errPtr != nil {
			C.gio_client_free_drag_error(errPtr)
		}
	}()
	if code != 0 {
		if errPtr != nil {
			return errors.New(C.GoString(errPtr))
		}
		return errors.New("native drag failed")
	}
	return nil
}
