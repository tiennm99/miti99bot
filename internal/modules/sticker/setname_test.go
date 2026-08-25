package sticker

import (
	"strings"
	"testing"
)

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		slug string
		ok   bool
	}{
		{"mypack", true},
		{"my_pack_2", true},
		{"abc", true},
		{strings.Repeat("a", maxSlugLen), true},
		{"ab", false},                              // too short
		{strings.Repeat("a", maxSlugLen+1), false}, // too long
		{"1pack", false},                           // leading digit
		{"My_Pack", false},                         // uppercase
		{"my__pack", false},                        // consecutive underscores
		{"mypack_", false},                         // trailing underscore
		{"my-pack", false},                         // hyphen
		{"", false},
	}
	for _, tc := range cases {
		err := validateSlug(tc.slug)
		if tc.ok && err != nil {
			t.Errorf("validateSlug(%q) = %v, want nil", tc.slug, err)
		}
		if !tc.ok && err == nil {
			t.Errorf("validateSlug(%q) = nil, want an error", tc.slug)
		}
	}
}

func TestMakeSetName(t *testing.T) {
	got, err := makeSetName("mypack", "miti99bot")
	if err != nil {
		t.Fatalf("makeSetName: %v", err)
	}
	if want := "mypack_by_miti99bot"; got != want {
		t.Errorf("makeSetName = %q, want %q", got, want)
	}
}

// The set name has a hard 64-char ceiling and the slug is the only part the
// user controls, so the refusal has to name the budget they actually have.
func TestMakeSetName_TooLongReportsBudget(t *testing.T) {
	username := strings.Repeat("b", 30)
	slug := strings.Repeat("a", maxSlugLen)
	_, err := makeSetName(slug, username)
	if err == nil {
		t.Fatalf("makeSetName(%d-char slug, %d-char username) succeeded; want a refusal", len(slug), len(username))
	}
	// 64 - len("_by_" + username) = 30
	if !strings.Contains(err.Error(), "30") {
		t.Errorf("refusal %q does not state the remaining budget", err)
	}
}

func TestOwnsSet(t *testing.T) {
	pack := Pack{Slug: "mypack", Name: "mypack_by_bot"}
	cases := []struct {
		setName string
		want    bool
	}{
		{"mypack_by_bot", true},
		{"MyPack_By_Bot", true}, // Telegram echoes the creation casing
		{"otherpack_by_bot", false},
		{"mypack_by_otherbot", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := ownsSet(pack, tc.setName); got != tc.want {
			t.Errorf("ownsSet(%q) = %v, want %v", tc.setName, got, tc.want)
		}
	}
	if ownsSet(Pack{}, "anything") {
		t.Error("ownsSet with an empty stored name = true, want false")
	}
}

// Renaming the bot in BotFather leaves existing set names untouched. Ownership
// compares the stored name for exactly this reason: deriving it from the live
// username would make every user's own pack refuse as "not yours".
func TestOwnsSet_SurvivesBotRename(t *testing.T) {
	pack := Pack{Slug: "mypack", Name: "mypack_by_oldbot"}
	if !ownsSet(pack, "mypack_by_oldbot") {
		t.Error("pack stopped resolving after the bot was renamed")
	}
	// The new username only ever builds *new* names.
	fresh, err := makeSetName("newpack", "newbot")
	if err != nil || fresh != "newpack_by_newbot" {
		t.Errorf("makeSetName after rename = (%q, %v)", fresh, err)
	}
}
