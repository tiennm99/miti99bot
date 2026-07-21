package modules

import (
	"context"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func okHandler(_ context.Context, _ *bot.Bot, _ *models.Update) error { return nil }

func TestValidateCommand_RejectsBadNames(t *testing.T) {
	cases := map[string]string{
		"empty":      "",
		"uppercase":  "Ping",
		"hyphen":     "do-thing",
		"too long":   "abcdefghijklmnopqrstuvwxyzabcdefg", // 33 chars
		"with slash": "/ping",
		"unicode":    "пинг",
	}
	for label, name := range cases {
		t.Run(label, func(t *testing.T) {
			err := validateCommand(Command{Name: name, Visibility: VisibilityPublic, Description: "d", Handler: okHandler})
			if err == nil {
				t.Errorf("name %q: expected error", name)
			}
		})
	}
}

func TestValidateCommand_RejectsInvalidPresentationMetadata(t *testing.T) {
	tests := []struct {
		name    string
		command Command
	}{
		{
			name:    "blank description",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: "   ", Handler: okHandler},
		},
		{
			name:    "multiline description",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: "one\ntwo", Handler: okHandler},
		},
		{
			name:    "multiline parameters",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: "d", Parameters: "<a>\n<b>", Handler: okHandler},
		},
		{
			name:    "multiline example",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: "d", Example: "/ok one\n/ok two", Handler: okHandler},
		},
		{
			name:    "example invokes another command",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: "d", Example: "/other", Handler: okHandler},
		},
		{
			name:    "native description too long",
			command: Command{Name: "ok", Visibility: VisibilityPublic, Description: strings.Repeat("x", 250), Handler: okHandler},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateCommand(tc.command); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestValidateCommand_AcceptsLegalNames(t *testing.T) {
	for _, name := range []string{"ping", "do_it", "a", "abc123", "x_1_y"} {
		if err := validateCommand(Command{Name: name, Visibility: VisibilityPublic, Description: "d", Handler: okHandler}); err != nil {
			t.Errorf("name %q: unexpected error %v", name, err)
		}
	}
}

func TestValidateCommand_RequiresDescriptionAndHandler(t *testing.T) {
	if err := validateCommand(Command{Name: "ok", Visibility: VisibilityPublic, Description: "", Handler: okHandler}); err == nil {
		t.Error("expected error for empty description")
	}
	if err := validateCommand(Command{Name: "ok", Visibility: VisibilityPublic, Description: "d", Handler: nil}); err == nil {
		t.Error("expected error for nil handler")
	}
}

func TestValidateCron_RequiresNameAndHandler(t *testing.T) {
	if err := validateCron(Cron{Name: "", Handler: func(_ context.Context, _ Deps) error { return nil }}); err == nil {
		t.Error("expected error for empty name")
	}
	if err := validateCron(Cron{Name: "x", Handler: nil}); err == nil {
		t.Error("expected error for nil handler")
	}
}
