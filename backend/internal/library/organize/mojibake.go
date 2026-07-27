package organize

import (
	"unicode"

	"golang.org/x/text/encoding/charmap"
)

// fixCyrillicMojibake repairs tag text that dhowden/tag decoded as ID3v2's
// "ISO-8859-1" encoding byte (a literal byte-for-byte mapping to runes
// U+0000-U+00FF) when the bytes on disk were actually Windows-1251 —
// something old (Russian/Ukrainian) tagging tools wrote while still
// flagging the frame as ISO-8859-1. Left uncorrected, a Cyrillic tag like
// "Азарт" round-trips as garbled Latin-1-supplement letters ("Àçàðò").
//
// Only strings made up entirely of letters in the Latin-1 supplement range
// are considered: genuine ISO-8859-1 text (café, Mötley Crüe, ...) mixes
// those with plain ASCII letters, which this deliberately leaves alone
// rather than risk corrupting it.
func fixCyrillicMojibake(s string) string {
	if s == "" {
		return s
	}

	letters, inRange := 0, 0
	for _, r := range s {
		if !unicode.IsLetter(r) {
			continue
		}
		letters++
		if r >= 0xC0 && r <= 0xFF {
			inRange++
		}
	}
	if letters == 0 || inRange != letters {
		return s
	}

	raw := make([]byte, 0, len(s))
	for _, r := range s {
		if r > 0xFF {
			return s
		}
		raw = append(raw, byte(r))
	}

	decoded, err := charmap.Windows1251.NewDecoder().Bytes(raw)
	if err != nil {
		return s
	}

	fixed := string(decoded)
	for _, r := range fixed {
		if r == ' ' || unicode.Is(unicode.Cyrillic, r) {
			continue
		}
		return s
	}
	return fixed
}
