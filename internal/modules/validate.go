package modules

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

var commandNameRe = regexp.MustCompile(`^[a-z0-9_]{1,32}$`)

const telegramCommandDescriptionMaxRunes = 256

func validateCommand(c Command) error {
	if !commandNameRe.MatchString(c.Name) {
		return fmt.Errorf("command name %q must match %s", c.Name, commandNameRe)
	}
	switch c.Visibility {
	case VisibilityPublic, VisibilityProtected, VisibilityPrivate:
	default:
		return fmt.Errorf("command %q: unknown visibility %d", c.Name, c.Visibility)
	}
	if strings.TrimSpace(c.Description) == "" {
		return fmt.Errorf("command %q: description is required", c.Name)
	}
	if strings.ContainsAny(c.Description, "\r\n") {
		return fmt.Errorf("command %q: description must be single-line", c.Name)
	}
	if strings.ContainsAny(c.Parameters, "\r\n") {
		return fmt.Errorf("command %q: parameters must be single-line", c.Name)
	}
	if c.Visibility == VisibilityPublic && utf8.RuneCountInString(c.TelegramMenuDescription()) > telegramCommandDescriptionMaxRunes {
		return fmt.Errorf("command %q: Telegram menu description exceeds %d characters", c.Name, telegramCommandDescriptionMaxRunes)
	}
	if c.Handler == nil {
		return fmt.Errorf("command %q: handler is nil", c.Name)
	}
	return nil
}

func validateCron(c Cron) error {
	if c.Name == "" {
		return fmt.Errorf("cron: name is required")
	}
	if c.Handler == nil {
		return fmt.Errorf("cron %q: handler is nil", c.Name)
	}
	return nil
}

func validateCallback(c Callback) error {
	if strings.TrimSpace(c.Prefix) == "" {
		return fmt.Errorf("callback: prefix is required")
	}
	if len(c.Prefix) > 32 || strings.ContainsAny(c.Prefix, "\r\n\x00") {
		return fmt.Errorf("callback prefix %q is invalid", c.Prefix)
	}
	switch c.Visibility {
	case VisibilityPublic, VisibilityProtected, VisibilityPrivate:
	default:
		return fmt.Errorf("callback prefix %q: unknown visibility %d", c.Prefix, c.Visibility)
	}
	if c.Handler == nil {
		return fmt.Errorf("callback prefix %q: handler is nil", c.Prefix)
	}
	return nil
}
