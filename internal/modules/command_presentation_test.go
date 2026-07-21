package modules

import "testing"

func TestCommandPresentation(t *testing.T) {
	tests := []struct {
		name       string
		command    Command
		invocation string
		menu       string
	}{
		{
			name: "parameters",
			command: Command{
				Name:        "stock_buy",
				Parameters:  "<quantity> <ticker>",
				Description: "Buy VN stock at market price",
			},
			invocation: "/stock_buy <quantity> <ticker>",
			menu:       "<quantity> <ticker>. Buy VN stock at market price.",
		},
		{
			name:       "no parameters",
			command:    Command{Name: "ping", Description: "Health check!"},
			invocation: "/ping",
			menu:       "Health check!",
		},
		{
			name: "concise parameter name",
			command: Command{
				Name:        "random",
				Parameters:  "<option,...>",
				Description: "Pick one option",
			},
			invocation: "/random <option,...>",
			menu:       "<option,...>. Pick one option.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.command.Invocation(); got != tc.invocation {
				t.Errorf("Invocation() = %q, want %q", got, tc.invocation)
			}
			if got := tc.command.TelegramMenuDescription(); got != tc.menu {
				t.Errorf("TelegramMenuDescription() = %q, want %q", got, tc.menu)
			}
		})
	}
}
