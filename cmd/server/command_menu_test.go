package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/go-telegram/bot/models"

	"github.com/tiennm99/miti99bot/internal/modules"
	"github.com/tiennm99/miti99bot/internal/modules/stock"
	moduleutil "github.com/tiennm99/miti99bot/internal/modules/util"
	"github.com/tiennm99/miti99bot/internal/storage"
	"github.com/tiennm99/miti99bot/internal/testutil"
)

func TestBotCommandMenu_UsesLoadedPublicCommandsInModuleOrder(t *testing.T) {
	reg := &modules.Registry{
		Modules: []modules.Module{
			{
				Name: "beta",
				Commands: []modules.Command{
					{Name: "beta_public", Description: "Beta public", Parameters: "<value>", Visibility: modules.VisibilityPublic},
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
		{Command: "beta_public", Description: "<value>. Beta public."},
		{Command: "alpha_public", Description: "Alpha public."},
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

func TestCommandDiscovery_AllPublicCommandsHaveSafeMetadata(t *testing.T) {
	reg, err := modules.Build(nil, factories(), storage.NewMemoryProvider(), modules.BuildOptions{})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	expectedParameters := map[string]string{
		"coin_price":           "<coin>",
		"coin_topup":           "<usd_amount>",
		"coin_buy":             "<coin> <usd_to_spend>",
		"coin_sell":            "<coin> <usd_to_receive>",
		"gold_topup":           "<vnd_amount>",
		"gold_buy":             "<luong>",
		"gold_sell":            "<luong>",
		"lol":                  "[date]",
		"loldle":               "[champion]",
		"random":               "<options(comma-separated)>",
		"stats":                "[users | user <username> | cmd <command_name>]",
		"stock_price":          "<ticker>",
		"stock_topup":          "<vnd_amount>",
		"stock_buy":            "<quantity> <ticker>",
		"stock_sell":           "<quantity> <ticker>",
		"stock_cash_dividend":  "<vnd_per_share> <ticker>",
		"stock_share_dividend": "<ratio(owned:new)> <ticker>",
		"stock_dividend":       "<vnd_per_share> <ratio(owned:new)> <ticker>",
		"trongtruonghop":       "[target...]",
		"tth":                  "[target...]",
		"wheelofnames":         "<options(comma-separated)>",
		"wordle":               "[word]",
	}

	menu := botCommandMenu(reg)
	if len(menu) != len(reg.PublicCommands()) {
		t.Fatalf("menu commands = %d, public commands = %d", len(menu), len(reg.PublicCommands()))
	}
	for _, command := range reg.PublicCommands() {
		if got := command.Parameters; got != expectedParameters[command.Name] {
			t.Errorf("/%s parameters = %q, want %q", command.Name, got, expectedParameters[command.Name])
		}
		description := command.TelegramMenuDescription()
		if strings.Contains(description, "Eg:") {
			t.Errorf("/%s native menu description contains an example: %q", command.Name, description)
		}
		if strings.ContainsAny(description, "\r\n") {
			t.Errorf("/%s native menu description is multiline: %q", command.Name, description)
		}
		if utf8.RuneCountInString(description) > telegramCommandDescriptionMaxRunesForTest {
			t.Errorf("/%s native menu description exceeds Telegram limit: %d", command.Name, utf8.RuneCountInString(description))
		}
	}

	help := moduleutil.RenderHelp(reg)
	if utf8.RuneCountInString(help) > telegramMessageMaxRunesForTest {
		t.Fatalf("/help source is %d characters, exceeds conservative Telegram limit %d", utf8.RuneCountInString(help), telegramMessageMaxRunesForTest)
	}
}

const (
	telegramCommandDescriptionMaxRunesForTest = 256
	telegramMessageMaxRunesForTest            = 4096
)

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
	if len(cmds) != 1 || cmds[0].Command != "demo" || cmds[0].Description != "Demo command." {
		t.Fatalf("commands payload = %+v, want demo command", cmds)
	}
}
