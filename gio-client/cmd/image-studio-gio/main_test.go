package main

import (
	"net/url"
	"testing"

	"image-studio/gio-client/internal/promptipc"
)

func TestPromptImportMessageFromURLInvalidToken(t *testing.T) {
	u, err := url.Parse("image-studio://import?token=invalid")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	msg := promptImportMessageFromURL(u)
	if msg.Type != promptipc.MessageTypeInvalid {
		t.Fatalf("message type=%q want %q", msg.Type, promptipc.MessageTypeInvalid)
	}
}

func TestPromptImportMessageFromURLValidToken(t *testing.T) {
	u, err := url.Parse("image-studio://import?token=AB12cd34")
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	msg := promptImportMessageFromURL(u)
	if msg.Type != promptipc.MessageTypeToken {
		t.Fatalf("message type=%q want %q", msg.Type, promptipc.MessageTypeToken)
	}
	if msg.Token != "AB12cd34" {
		t.Fatalf("message token=%q want %q", msg.Token, "AB12cd34")
	}
}

func TestPromptImportMessageFromURLNil(t *testing.T) {
	msg := promptImportMessageFromURL(nil)
	if msg.Type != "" {
		t.Fatalf("message type=%q want empty", msg.Type)
	}
}
