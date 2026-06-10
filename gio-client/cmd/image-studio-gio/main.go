//go:build windows || linux

package main

import (
	"fmt"
	"log"
	"os"

	"image-studio/gio-client/internal/promptipc"
	"image-studio/gio-client/internal/ui"

	"gioui.org/app"
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
	appUI.StartBackgroundAppUpdateCheck()
	server, alreadyRunning, err := promptipc.TryStart(func(msg promptipc.Message) {
		switch msg.Type {
		case promptipc.MessageTypeRaise:
			appUI.RaiseWindow()
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
			log.Fatal(err)
		}
		os.Exit(0)
	}()
	app.Main()
}
