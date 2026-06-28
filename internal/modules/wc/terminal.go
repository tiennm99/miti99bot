package wc

import (
	"errors"
	"strings"
)

type terminalKind int

const (
	terminalNone terminalKind = iota
	terminalChatWide
	terminalTopicOnly
)

var chatWideTerminalMarkers = []string{
	"bot was blocked by the user",
	"user is deactivated",
	"bot is not a member",
	"chat not found",
	"group chat was upgraded",
	"chat was deleted",
}

var topicOnlyTerminalMarkers = []string{
	"have no rights to send",
}

func classifyTerminal(err error) terminalKind {
	if err == nil {
		return terminalNone
	}
	msg := err.Error()
	for _, m := range chatWideTerminalMarkers {
		if strings.Contains(msg, m) {
			return terminalChatWide
		}
	}
	for _, m := range topicOnlyTerminalMarkers {
		if strings.Contains(msg, m) {
			return terminalTopicOnly
		}
	}
	return terminalNone
}

var errNilBot = errors.New("wc daily push: deps.Bot is nil (BuildOptions.Bot not wired)")
