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
	if strings.ContainsAny(c.Example, "\r\n") {
		return fmt.Errorf("command %q: example must be single-line", c.Name)
	}
	parameters := strings.TrimSpace(c.Parameters)
	example := strings.TrimSpace(c.Example)
	if c.Visibility == VisibilityPublic {
		if parameters == "" && example != "" {
			return fmt.Errorf("command %q: example requires parameters", c.Name)
		}
		if parameters != "" && example == "" {
			return fmt.Errorf("command %q: example is required when parameters are present", c.Name)
		}
	}
	if example != "" {
		prefix := "/" + c.Name
		if example != prefix && !strings.HasPrefix(example, prefix+" ") {
			return fmt.Errorf("command %q: example must invoke %s", c.Name, prefix)
		}
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
