package util

import (
	"strings"
	"testing"
)

func TestParseEmoji(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"single", []string{"😂"}, []string{"😂"}},
		{"space separated", []string{"😂", "🔥"}, []string{"😂", "🔥"}},
		{"joined in one arg", []string{"😂🔥"}, []string{"😂", "🔥"}},
		{"zwj family stays one", []string{"👩\u200d👩\u200d👧"}, []string{"👩\u200d👩\u200d👧"}},
		{"skin tone binds", []string{"👍🏽"}, []string{"👍🏽"}},
		{"flag is one cluster", []string{"🇻🇳"}, []string{"🇻🇳"}},
		{"two flags", []string{"🇻🇳🇯🇵"}, []string{"🇻🇳", "🇯🇵"}},
		{"keycap stays one", []string{"1️⃣"}, []string{"1️⃣"}},
		{"variation selector binds", []string{"❤️"}, []string{"❤️"}},
		{"star default", []string{"⭐"}, []string{"⭐"}},
		{"empty", nil, nil},
		{"blank", []string{"  "}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEmoji(tc.args)
			if err != nil {
				t.Fatalf("parseEmoji(%q) error: %v", tc.args, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseEmoji(%q) = %q, want %q", tc.args, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("cluster %d = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// A stray word must fail loudly. With no pack argument left on /addsticker,
// there is nothing else an argument could have meant.
func TestParseEmoji_RejectsPlainText(t *testing.T) {
	for _, arg := range []string{"mypack", "hello 😂", "a"} {
		if got, err := parseEmoji([]string{arg}); err == nil {
			t.Errorf("parseEmoji(%q) = %q, want an error", arg, got)
		}
	}
}

func TestParseEmoji_CapsAtTwenty(t *testing.T) {
	twenty := strings.Repeat("😂", maxEmojiPerSticker)
	if got, err := parseEmoji([]string{twenty}); err != nil {
		t.Fatalf("parseEmoji(20 emoji) error: %v (got %d)", err, len(got))
	}
	if _, err := parseEmoji([]string{twenty + "🔥"}); err == nil {
		t.Error("parseEmoji(21 emoji) succeeded, want an error")
	}
}

// Refusals are shown to the user verbatim, so they must be userError.
func TestParseEmoji_RefusalIsUserFacing(t *testing.T) {
	_, err := parseEmoji([]string{"notanemoji"})
	if err == nil {
		t.Fatal("want an error")
	}
	if _, ok := err.(userError); !ok {
		t.Errorf("err is %T, want userError so the handler can echo it", err)
	}
}

// TestParseEmoji_ClusterEdgeCases pins the emoji-clustering rules that a
// hand-rolled segmenter gets wrong. Every case here failed before the
// clustering fix: the first group was refused outright, the second silently
// produced an emoji_list Telegram rejects.
func TestParseEmoji_ClusterEdgeCases(t *testing.T) {
	accepted := []struct {
		name string
		in   string
		want []string
	}{
		// Emoji that fall between the blocks the range table covers.
		{"copyright", "©️", []string{"©️"}},
		{"registered", "®️", []string{"®️"}},
		{"wavy dash", "〰️", []string{"〰️"}},
		{"part alternation", "〽️", []string{"〽️"}},
		{"japanese congratulations", "㊗️", []string{"㊗️"}},
		{"japanese secret", "㊙️", []string{"㊙️"}},
		{"circled m", "Ⓜ️", []string{"Ⓜ️"}},
		{"arrow curving up", "⤴️", []string{"⤴️"}},
		{"arrow curving down", "⤵️", []string{"⤵️"}},

		// A subdivision flag is a base flag plus invisible tag characters.
		{
			"tag sequence flag",
			"\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F",
			[]string{"\U0001F3F4\U000E0067\U000E0062\U000E0065\U000E006E\U000E0067\U000E007F"},
		},

		// A dangling joiner is dropped rather than shipped.
		{"trailing joiner", "\U0001F600\u200d", []string{"\U0001F600"}},

		// The joiner must not swallow a flag's first half.
		{
			"joiner before a flag",
			"\U0001F600\u200d\U0001F1FB\U0001F1F3",
			[]string{"\U0001F600", "\U0001F1FB\U0001F1F3"},
		},

		// Sequences that already worked, kept here so a fix cannot regress them.
		{"family", "\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466", []string{"\U0001F468\u200d\U0001F469\u200d\U0001F467\u200d\U0001F466"}},
		{"skin tone", "\U0001F44D\U0001F3FD", []string{"\U0001F44D\U0001F3FD"}},
		{"keycap", "1️⃣", []string{"1️⃣"}},
		{"flag", "\U0001F1FB\U0001F1F3", []string{"\U0001F1FB\U0001F1F3"}},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEmoji([]string{tc.in})
			if err != nil {
				t.Fatalf("parseEmoji(%+q) refused: %v", tc.in, err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("parseEmoji(%+q) = %+q, want %+q", tc.in, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("parseEmoji(%+q) = %+q, want %+q", tc.in, got, tc.want)
				}
			}
		})
	}

	refused := []struct {
		name string
		in   string
	}{
		// A lone regional indicator is half a flag, and Telegram rejects it.
		{"lone regional indicator", "\U0001F1FB"},
		{"odd regional indicator count", "\U0001F1FB\U0001F1F3\U0001F1FA"},
		{"plain text", "hello"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseEmoji([]string{tc.in})
			if err == nil {
				t.Fatalf("parseEmoji(%+q) = %+q, want refusal", tc.in, got)
			}
		})
	}
}
