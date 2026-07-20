package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/stock"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

func TestBotCommandMenu_UsesLoadedPublicCommandsInModuleOrder(t *testing.T) {
	reg := &modules.Registry{
		Modules: []modules.Module{
			{
				Name: "beta",
				Commands: []modules.Command{
					{Name: "beta_public", Description: "Beta public", Visibility: modules.VisibilityPublic},
					{Name: "beta_private", Description: "Beta private", Visibility: modules.VisibilityPrivate},
				},
			},
			{
				Name: "alpha",
				Commands: []modules.Command{
					{Name: "alpha_public", Description: "Alpha public", Visibility: modules.VisibilityPublic},
					{Name: "alpha_protected", Description: "Alpha protected", Visibility: modules.VisibilityProtected},
				},
			},
		},
	}

	got := botCommandMenu(reg)
	want := []models.BotCommand{
		{Command: "beta_public", Description: "Beta public"},
		{Command: "alpha_public", Description: "Alpha public"},
	}
	if len(got) != len(want) {
		t.Fatalf("commands = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("commands[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBotCommandMenu_StockDividendContracts(t *testing.T) {
	mod := stock.New(modules.Deps{Store: storage.NewMemoryProvider().Collection("stock")})
	mod.Name = "stock"
	got := botCommandMenu(&modules.Registry{Modules: []modules.Module{mod}})

	commands := make(map[string]string, len(got))
	for _, command := range got {
		commands[command.Command] = command.Description
		if len(command.Description) > 256 {
			t.Fatalf("description for %s exceeds Telegram limit", command.Command)
		}
	}
	for _, name := range []string{"stock_cash_dividend", "stock_share_dividend", "stock_dividend"} {
		if commands[name] == "" {
			t.Fatalf("stock menu missing %s: %v", name, commands)
		}
	}
	if _, exists := commands["stock_bonus"]; exists {
		t.Fatalf("stock_bonus remains in public menu: %v", commands)
	}
}

func TestRegisterCommandMenu_CallsTelegramSetMyCommands(t *testing.T) {
	reg := &modules.Registry{
		Modules: []modules.Module{{
			Name: "demo",
			Commands: []modules.Command{{
				Name:        "demo",
				Description: "Demo command",
				Visibility:  modules.VisibilityPublic,
			}},
		}},
	}
	rb := testutil.NewRecordingBot(t)

	n, err := registerCommandMenu(context.Background(), rb.Bot, reg)
	if err != nil {
		t.Fatalf("registerCommandMenu: %v", err)
	}
	if n != 1 {
		t.Fatalf("registered count = %d, want 1", n)
	}

	call := rb.LastSent()
	if call.Method != "setMyCommands" {
		t.Fatalf("method = %q, want setMyCommands", call.Method)
	}
	var cmds []models.BotCommand
	if err := json.Unmarshal([]byte(call.Form["commands"]), &cmds); err != nil {
		t.Fatalf("decode commands form field: %v; raw=%q", err, call.Form["commands"])
	}
	if len(cmds) != 1 || cmds[0].Command != "demo" || cmds[0].Description != "Demo command" {
		t.Fatalf("commands payload = %+v, want demo command", cmds)
	}
}
