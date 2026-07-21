package modules

import "testing"

func TestCommandPresentation(t *testing.T) {
	tests := []struct {
		name       string
		command    Command
		invocation string
		example    string
		menu       string
	}{
		{
			name: "parameters and explicit example",
			command: Command{
				Name:        "stock_buy",
				Parameters:  "<quantity> <ticker>",
				Description: "Buy VN stock at market price",
				Example:     "/stock_buy 100 TCB",
			},
			invocation: "/stock_buy <quantity> <ticker>",
			example:    "/stock_buy 100 TCB",
			menu:       "<quantity> <ticker>. Buy VN stock at market price. Eg: /stock_buy 100 TCB",
		},
		{
			name:       "no parameters omits example",
			command:    Command{Name: "ping", Description: "Health check!"},
			invocation: "/ping",
			example:    "",
			menu:       "Health check!",
		},
		{
			name: "variadic parameters keep ellipsis",
			command: Command{
				Name:        "random",
				Parameters:  "<options(comma-separated)>",
				Description: "Pick one option",
				Example:     "/random pizza, sushi",
			},
			invocation: "/random <options(comma-separated)>",
			example:    "/random pizza, sushi",
			menu:       "<options(comma-separated)>. Pick one option. Eg: /random pizza, sushi",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.command.Invocation(); got != tc.invocation {
				t.Errorf("Invocation() = %q, want %q", got, tc.invocation)
			}
			if got := tc.command.ExampleInvocation(); got != tc.example {
				t.Errorf("ExampleInvocation() = %q, want %q", got, tc.example)
			}
			if got := tc.command.TelegramMenuDescription(); got != tc.menu {
				t.Errorf("TelegramMenuDescription() = %q, want %q", got, tc.menu)
			}
		})
	}
}
