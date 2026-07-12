//go:build windows || (linux && !android) || (darwin && !ios)

package main

import (
	"fmt"
	"log"
	"os"

	"image-studio/gio-client/internal/promptipc"
	"image-studio/gio-client/internal/ui"
	"image-studio/gio-client/internal/windowing"

	"gioui.org/app"
	"gioui.org/io/event"
	"gioui.org/unit"
)

func main() {
	if handled, exitCode, err := runCLICommand(os.Args[1:], os.Stdout, os.Stderr); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCode)
	}
	appUI := ui.New()
	desktopWindows := windowing.NewManager(ui.NewDesktopWindowFactory(appUI), appUI.HandleDesktopWindowError)
	appUI.SetDesktopWindowController(desktopWindows)
	appUI.RestoreDesktopWindows()
	appUI.StartBackgroundAppUpdateCheck()
	server, alreadyRunning, err := promptipc.TryStart(func(msg promptipc.Message) {
		switch msg.Type {
		case promptipc.MessageTypeRaise:
			appUI.RaiseWindow()
		case promptipc.MessageTypeOpenResult:
			appUI.RaiseWindow()
			appUI.OpenResultDetailByIDOrSavedPath(msg.ResultID, msg.SavedPath)
		case promptipc.MessageTypeToken:
			appUI.HandlePromptImportToken(msg.Token)
		case promptipc.MessageTypeInvalid:
			appUI.HandlePromptImportInvalid()
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	defer server.Close()
	initialMessage := promptImportMessageFromArgs(os.Args[1:])
	if alreadyRunning {
		if initialMessage.Type == "" {
			_ = promptipc.SendRaise()
		} else {
			_ = promptipc.Send(initialMessage)
		}
		os.Exit(0)
	}
	switch initialMessage.Type {
	case promptipc.MessageTypeToken:
		appUI.HandlePromptImportToken(initialMessage.Token)
	case promptipc.MessageTypeInvalid:
		appUI.HandlePromptImportInvalid()
	}
	go func() {
		w := new(app.Window)
		w.Option(
			app.Title("Image Studio Gio"),
			app.Size(unit.Dp(1440), unit.Dp(980)),
			app.MinSize(unit.Dp(1040), unit.Dp(720)),
		)
		if err := appUI.Run(w); err != nil {
			log.Printf("main window closed with error: %v", err)
		}
		shutdownDesktopResources(
			server,
			desktopWindows,
			desktopWindowShutdownTimeout,
			desktopWindowShutdownPollInterval,
			log.Printf,
		)
		os.Exit(0)
	}()
	app.Events(func(evt event.Event) bool {
		switch e := evt.(type) {
		case app.URLEvent:
			handlePromptImportURLEvent(appUI, e)
		}
		return true
	})
}

func handlePromptImportURLEvent(appUI *ui.App, evt app.URLEvent) {
	msg := promptImportMessageFromURL(evt.URL)
	switch msg.Type {
	case promptipc.MessageTypeToken:
		appUI.HandlePromptImportToken(msg.Token)
	case promptipc.MessageTypeInvalid:
		appUI.HandlePromptImportInvalid()
	}
}
