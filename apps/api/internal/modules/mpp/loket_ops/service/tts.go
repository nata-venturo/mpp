package service

import (
	"strconv"
	"strings"
	"unicode"
)

// digitWords spell a queue number one digit at a time — "014" is read
// "nol satu empat", never "empat belas". A citizen matching a spoken
// number against a printed one needs the digits, not the value.
var digitWords = [...]string{
	"nol", "satu", "dua", "tiga", "empat",
	"lima", "enam", "tujuh", "delapan", "sembilan",
}

// SpeakNomor turns a printed queue number into speakable Indonesian:
//
//	"A-014" → "A - nol satu empat"
//
// Letters are left as letters (a TTS voice reads "A" correctly), digits
// become words, and separators become a spoken pause.
func SpeakNomor(nomor string) string {
	parts := make([]string, 0, len(nomor))

	for _, r := range nomor {
		switch {
		case unicode.IsDigit(r):
			parts = append(parts, digitWords[r-'0'])
		case r == '-' || r == '/' || r == '.':
			parts = append(parts, "-")
		case r == ' ':
			// Already a boundary; nothing to say.
		default:
			parts = append(parts, string(r))
		}
	}

	return strings.Join(parts, " ")
}

// SpeakLoket turns a counter label into speakable Indonesian:
//
//	"Loket 3"  → "loket tiga"
//	"L-A3"     → "loket a tiga"
//
// Counter numbers ARE read as values (loket tiga, loket dua belas) —
// unlike queue numbers, they are quantities, not identifiers to match
// digit by digit.
func SpeakLoket(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "loket"
	}

	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		return r == ' ' || r == '-' || r == '_'
	})

	spoken := make([]string, 0, len(fields)+1)
	spoken = append(spoken, "loket")

	for _, field := range fields {
		lower := strings.ToLower(field)
		if lower == "loket" {
			continue
		}

		if n, err := strconv.Atoi(field); err == nil {
			spoken = append(spoken, SpeakNumber(n))
			continue
		}

		// Mixed token like "A3": split the letters from the number.
		letters := strings.TrimRightFunc(field, unicode.IsDigit)
		digits := strings.TrimPrefix(field, letters)
		if letters != "" {
			spoken = append(spoken, strings.ToLower(letters))
		}
		if n, err := strconv.Atoi(digits); err == nil {
			spoken = append(spoken, SpeakNumber(n))
		}
	}

	return strings.Join(spoken, " ")
}

// SpeakNumber renders a small cardinal number in Indonesian. Counters
// never run into the thousands, so the table stops at 999 and falls back
// to digit-by-digit above that rather than growing a full grammar.
func SpeakNumber(n int) string {
	switch {
	case n < 0:
		return strconv.Itoa(n)
	case n < 10:
		return digitWords[n]
	case n == 10:
		return "sepuluh"
	case n == 11:
		return "sebelas"
	case n < 20:
		return digitWords[n-10] + " belas"
	case n < 100:
		tens := digitWords[n/10] + " puluh"
		if n%10 == 0 {
			return tens
		}
		return tens + " " + digitWords[n%10]
	case n < 200:
		if n == 100 {
			return "seratus"
		}
		return "seratus " + SpeakNumber(n-100)
	case n < 1000:
		hundreds := digitWords[n/100] + " ratus"
		if n%100 == 0 {
			return hundreds
		}
		return hundreds + " " + SpeakNumber(n%100)
	default:
		return SpeakNomor(strconv.Itoa(n))
	}
}

// BuildTTSText fills the configured announcement template (FR-CFG-03).
// Default template: "Nomor antrian {nomor}, silakan menuju {loket}".
func BuildTTSText(template, nomor, loketName string) string {
	out := strings.ReplaceAll(template, "{nomor}", SpeakNomor(nomor))
	return strings.ReplaceAll(out, "{loket}", SpeakLoket(loketName))
}
