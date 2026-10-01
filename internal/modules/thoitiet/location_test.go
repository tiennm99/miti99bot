package thoitiet

import "testing"

func TestNormalizeQuery(t *testing.T) {
	cases := map[string]string{
		"":                    "",
		"  Đà   Lạt ":         "da lat",
		"Ðà Lạt":              "da lat", // GeoNames' look-alike Ð (U+00D0)
		"Hồ Chí Minh":         "ho chi minh",
		"Thừa Thiên Huế":      "thua thien hue",
		"Sài Gòn":             "sai gon",
		"TP.HCM":              "tp.hcm",
		"New York":            "new york",
		"Buôn Ma Thuột":       "buon ma thuot",
		"Phan Rang–Tháp Chàm": "phan rang–thap cham",
	}
	for in, want := range cases {
		if got := normalizeQuery(in); got != want {
			t.Errorf("normalizeQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickPlace(t *testing.T) {
	jp := place{Name: "Tokyo", CountryCode: "JP"}
	vn := place{Name: "Huế", CountryCode: "VN"}
	et := place{Name: "Humera", CountryCode: "ET"}

	if got, ok := pickPlace([]place{et, vn}); !ok || got != vn {
		t.Errorf("pickPlace prefers VN: got %+v, %v", got, ok)
	}
	if got, ok := pickPlace([]place{jp, et}); !ok || got != jp {
		t.Errorf("pickPlace falls back to first: got %+v, %v", got, ok)
	}
	if _, ok := pickPlace(nil); ok {
		t.Error("pickPlace(nil) ok = true, want false")
	}
}

func TestDisplayName(t *testing.T) {
	cases := []struct {
		p    place
		want string
	}{
		{hcmPlace, "Thành phố Hồ Chí Minh"},
		{place{Name: "Hà Nội", Admin1: "Hanoi", CountryCode: "VN"}, "Hà Nội"},
		{place{Name: "Vũng Tàu", Admin1: "Thành phố Hồ Chí Minh", CountryCode: "VN"}, "Vũng Tàu, Thành phố Hồ Chí Minh"},
		{place{Name: "Tokyo", Admin1: "Tokyo", CountryCode: "JP", Country: "Nhật Bản"}, "Tokyo, Nhật Bản"},
	}
	for _, c := range cases {
		if got := displayName(c.p); got != c.want {
			t.Errorf("displayName(%+v) = %q, want %q", c.p, got, c.want)
		}
	}
}
