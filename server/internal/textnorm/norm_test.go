package textnorm

import "testing"

func TestFold(t *testing.T) {
	cases := map[string]string{
		"Üçyol":            "ucyol",
		"ÜÇYOL":            "ucyol",
		"Subay Evleri̇":    "subay evleri",
		"Halkapınar-Konak": "halkapinar konak",
		"  İSKELE  ":       "iskele",
	}
	for in, want := range cases {
		if got := Fold(in); got != want {
			t.Errorf("Fold(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClean(t *testing.T) {
	if got := Clean("Sefaşehi̇r  Mah."); got != "Sefaşehir Mah." {
		t.Errorf("Clean = %q", got)
	}
}

func TestTitleTR(t *testing.T) {
	if got := TitleTR("ÜÇKUYULAR İSKELE"); got != "Üçkuyular İskele" {
		t.Errorf("got %q", got)
	}
	if got := TitleTR("KIRIKLAR"); got != "Kırıklar" {
		t.Errorf("got %q", got)
	}
}
