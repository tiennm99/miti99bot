package loldle

import "testing"

func TestLoadChampions_EmbedIsValid(t *testing.T) {
	cs := loadChampions()
	if n := len(cs); n < 150 || n > 200 {
		t.Errorf("champion count = %d, want ~172", n)
	}
	got := findChampionByExactName(cs, "Aatrox")
	if got == nil {
		t.Fatal("expected Aatrox in embedded list")
	}
	if got.Gender != "Male" || got.Resource != "Manaless" {
		t.Errorf("Aatrox shape unexpected: %+v", got)
	}
}

func TestFindChampion(t *testing.T) {
	pool := []Champion{{ChampionName: "Aatrox"}, {ChampionName: "Ahri"}, {ChampionName: "Kai'Sa"}}
	cases := []struct {
		input string
		want  string // "" means no match
	}{
		{"Aatrox", "Aatrox"},
		{"aatrox", "Aatrox"},
		{"kaisa", "Kai'Sa"},
		{"KAI SA", "Kai'Sa"},
		{"Aat", "Aatrox"}, // unique prefix
		{"A", ""},         // ambiguous prefix
		{"", ""},          // empty
		{"!!!", ""},       // no alphanumerics
		{"zed", ""},       // no match
	}
	for _, tc := range cases {
		got := findChampion(pool, tc.input)
		if tc.want == "" {
			if got != nil {
				t.Errorf("findChampion(%q) = %+v, want nil", tc.input, got)
			}
			continue
		}
		if got == nil || got.ChampionName != tc.want {
			t.Errorf("findChampion(%q) = %+v, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFindChampionMatchReportsAmbiguousPrefix(t *testing.T) {
	pool := []Champion{{ChampionName: "Aatrox"}, {ChampionName: "Ahri"}, {ChampionName: "Kai'Sa"}}
	got, ambiguous := findChampionMatch(pool, "A")
	if got != nil || !ambiguous {
		t.Fatalf("findChampionMatch ambiguous = (%+v, %v), want (nil, true)", got, ambiguous)
	}

	got, ambiguous = findChampionMatch(pool, "zed")
	if got != nil || ambiguous {
		t.Fatalf("findChampionMatch unknown = (%+v, %v), want (nil, false)", got, ambiguous)
	}
}
