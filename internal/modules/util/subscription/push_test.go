package subscription

import (
	"errors"
	"testing"
)

func TestClassifyTerminal(t *testing.T) {
	chatWide := []string{
		"Forbidden: bot was blocked by the user",
		"Forbidden: user is deactivated",
		"Bad Request: chat not found",
		"Bad Request: group chat was upgraded to a supergroup chat",
	}
	for _, msg := range chatWide {
		if got := ClassifyTerminal(errors.New(msg)); got != TerminalChatWide {
			t.Errorf("ClassifyTerminal(%q) = %v, want TerminalChatWide", msg, got)
		}
	}

	topicOnly := []string{
		"Bad Request: have no rights to send a message",
	}
	for _, msg := range topicOnly {
		if got := ClassifyTerminal(errors.New(msg)); got != TerminalTopicOnly {
			t.Errorf("ClassifyTerminal(%q) = %v, want TerminalTopicOnly", msg, got)
		}
	}

	transients := []string{
		"connection reset by peer",
		"Too Many Requests: retry after 30",
		"context deadline exceeded",
	}
	for _, msg := range transients {
		if got := ClassifyTerminal(errors.New(msg)); got != TerminalNone {
			t.Errorf("ClassifyTerminal(%q) = %v, want TerminalNone (transient)", msg, got)
		}
	}
	if got := ClassifyTerminal(nil); got != TerminalNone {
		t.Errorf("ClassifyTerminal(nil) = %v, want TerminalNone", got)
	}
}
