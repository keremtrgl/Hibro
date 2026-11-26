// Package textnorm cleans up and folds Turkish text for display and search.
package textnorm

import (
	"strings"
	"unicode"
)

// Clean fixes common encoding artefacts in municipal data, such as the
// "i + combining dot above" sequence (i̇) that some feeds emit instead of
// a plain "i", and collapses repeated whitespace.
func Clean(s string) string {
	s = strings.ReplaceAll(s, "i̇", "i")
	s = strings.ReplaceAll(s, "İ", "İ")
	return strings.Join(strings.Fields(s), " ")
}

var fold = strings.NewReplacer(
	"ı", "i", "İ", "i", "I", "i",
	"ş", "s", "Ş", "s",
	"ğ", "g", "Ğ", "g",
	"ü", "u", "Ü", "u",
	"ö", "o", "Ö", "o",
	"ç", "c", "Ç", "c",
	"â", "a", "î", "i", "û", "u",
	"̇", "",
)

// Fold lower-cases s and strips Turkish diacritics so "Üçyol", "ucyol"
// and "UÇYOL" all compare equal. Punctuation becomes spaces.
func Fold(s string) string {
	s = fold.Replace(s)
	s = strings.ToLower(s)
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// acronyms stay upper case when title-casing.
var acronyms = map[string]bool{
	"SGK": true, "AVM": true, "PTT": true, "TED": true, "KYK": true, "TOKİ": true, "SSK": true,
	"OSB": true, "TEM": true, "D100": true, "D-100": true, "ODTÜ": true, "İTÜ": true, "GOP": true,
	"TCDD": true, "İZBAN": true, "ESHOT": true, "TRT": true, "KTÜ": true, "MTA": true,
}

// TitleTR converts an ALL CAPS Turkish string to Title Case, respecting
// the dotted/dotless i rules ("ÜÇKUYULAR İSKELE" -> "Üçkuyular İskele",
// "OTOGAR-SEKAPARK" -> "Otogar-Sekapark"). Known acronyms stay upper case.
func TitleTR(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if acronyms[w] {
			continue
		}
		rs := []rune(w)
		start := true
		for j, r := range rs {
			if !unicode.IsLetter(r) {
				start = r == '-' || r == '/' || r == '(' || r == '.'
				continue
			}
			if start {
				start = false
				continue
			}
			rs[j] = lowerTR(r)
		}
		words[i] = string(rs)
	}
	return strings.Join(words, " ")
}

// IsAllCaps reports whether s has letters and none of them is lower case.
func IsAllCaps(s string) bool {
	hasUpper := false
	for _, r := range s {
		if unicode.IsLower(r) {
			return false
		}
		if unicode.IsUpper(r) {
			hasUpper = true
		}
	}
	return hasUpper
}

// Nice title-cases s only when it is entirely upper case (as some
// municipal feeds publish every name); mixed-case names are kept.
func Nice(s string) string {
	s = Clean(s)
	if IsAllCaps(s) {
		return TitleTR(s)
	}
	return s
}

func lowerTR(r rune) rune {
	switch r {
	case 'I':
		return 'ı'
	case 'İ':
		return 'i'
	}
	return unicode.ToLower(r)
}
