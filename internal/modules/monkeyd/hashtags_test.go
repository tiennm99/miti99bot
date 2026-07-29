package monkeyd

import "testing"

func TestHashtag(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"Cổ Đại", "CổĐại"},
		{"Gia Đình", "GiaĐình"},
		{"Trọng Sinh", "TrọngSinh"},
		{"Nữ Cường", "NữCường"},
		{"Vả Mặt", "VảMặt"},
		// Single word passes through untouched.
		{"Ngôn", "Ngôn"},
		// A lower-case label is capitalised per word.
		{"ngôn tình", "NgônTình"},
		// Punctuation is removed but still ends the word.
		{"sci-fi", "SciFi"},
		{"Đông/Tây", "ĐôngTây"},
		{"Truyện (Chữ)", "TruyệnChữ"},
		// Collapsed whitespace and surrounding space.
		{"  Cổ   Đại  ", "CổĐại"},
		// Digits are kept.
		{"3D", "3D"},
		{"18+ Tuổi", "18Tuổi"},
		// Nothing usable: Telegram will not linkify a digits-only hashtag.
		{"18+", ""},
		{"", ""},
		{"---", ""},
		{"+", ""},
	}
	for _, tt := range tests {
		if got := hashtag(tt.tag); got != tt.want {
			t.Errorf("hashtag(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

func TestHashtagLine(t *testing.T) {
	got := hashtagLine([]string{"Cổ Đại", "Gia Đình"})
	want := "#MonkeyD #CổĐại #GiaĐình"
	if got != want {
		t.Errorf("hashtagLine() = %q, want %q", got, want)
	}
}

func TestHashtagLineWithoutTags(t *testing.T) {
	if got := hashtagLine(nil); got != sourceHashtag {
		t.Errorf("hashtagLine(nil) = %q, want %q", got, sourceHashtag)
	}
}

// Labels differing only in punctuation collapse to one hashtag, and an unusable
// label is dropped rather than emitted as a bare "#".
func TestHashtagLineDropsUnusableAndDuplicates(t *testing.T) {
	got := hashtagLine([]string{"Cổ Đại", "Cổ-Đại", "18+", "Gia Đình"})
	want := "#MonkeyD #CổĐại #GiaĐình"
	if got != want {
		t.Errorf("hashtagLine() = %q, want %q", got, want)
	}
}

func TestTagsMessage(t *testing.T) {
	got := tagsMessage([]string{"Cổ Đại", "Gia Đình"}, "https://monkeydd.com/truong-an-gwem.html")
	want := "#MonkeyD #CổĐại #GiaĐình\n\nhttps://monkeydd.com/truong-an-gwem.html"
	if got != want {
		t.Errorf("tagsMessage() =\n%q\nwant\n%q", got, want)
	}
}

// The block is what makes the line copyable in one tap, and its contents must be
// escaped so a label containing markup cannot break out of it.
func TestMonospaceBlockEscapes(t *testing.T) {
	got := monospaceBlock("#MonkeyD #A<b>&")
	want := "<pre>#MonkeyD #A&lt;b&gt;&amp;</pre>"
	if got != want {
		t.Errorf("monospaceBlock() = %q, want %q", got, want)
	}
}
