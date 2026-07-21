package modules

import "strings"

// Invocation returns the copy-neutral command syntax shown in /help.
func (c Command) Invocation() string {
	invocation := "/" + c.Name
	if parameters := strings.TrimSpace(c.Parameters); parameters != "" {
		invocation += " " + parameters
	}
	return invocation
}

// InvocationSentence returns the command syntax with exactly one logical
// sentence terminator. Variadic syntax ending in "..." is already terminated.
func (c Command) InvocationSentence() string {
	return withTerminalPunctuation(c.Invocation())
}

// ExampleInvocation returns the explicitly registered copyable example.
func (c Command) ExampleInvocation() string {
	return strings.TrimSpace(c.Example)
}

// SummarySentence normalizes a command summary to a sentence without
// duplicating terminal punctuation supplied by the registration.
func (c Command) SummarySentence() string {
	return withTerminalPunctuation(c.Description)
}

// TelegramMenuDescription returns the single-line plain-text description used
// by setMyCommands. Telegram renders the /command separately and does not
// support code blocks in this surface.
func (c Command) TelegramMenuDescription() string {
	var sb strings.Builder
	if parameters := strings.TrimSpace(c.Parameters); parameters != "" {
		sb.WriteString(withTerminalPunctuation(parameters))
		sb.WriteByte(' ')
	}
	sb.WriteString(c.SummarySentence())
	if example := c.ExampleInvocation(); example != "" {
		sb.WriteString(" Eg: ")
		sb.WriteString(example)
	}
	return sb.String()
}

func withTerminalPunctuation(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, ".") || strings.HasSuffix(value, "!") || strings.HasSuffix(value, "?") {
		return value
	}
	return value + "."
}
