package store

import (
	"strings"
	"unicode"
)

// chosungTable maps Hangul choseong index (0..18) to its compatibility-jamo
// codepoint, which is the form users actually type on a Korean keyboard.
var chosungTable = []rune{
	'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ',
	'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ',
}

// ToChosung folds a string into its 초성 (initial-consonant) form so that
// substring matching can implement 초성검색:
//
//   - Precomposed Hangul syllables (U+AC00..U+D7A3) collapse to their leading
//     choseong jamo (compat block, e.g. '한' → 'ㅎ').
//   - Compatibility jamo passes through.
//   - Everything else is lowercased so Latin / digit queries match against the
//     same canonical form without a separate code path.
func ToChosung(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 0xAC00 && r <= 0xD7A3:
			idx := (r - 0xAC00) / (21 * 28)
			b.WriteRune(chosungTable[idx])
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// HasJamo reports whether s contains at least one Hangul compatibility-jamo
// rune (U+3131..U+318E). That's the signal that the user is typing a
// 초성검색 query rather than ordinary text.
func HasJamo(s string) bool {
	for _, r := range s {
		if r >= 0x3131 && r <= 0x318E {
			return true
		}
	}
	return false
}
