package store

import (
	"strings"
	"unicode"
)

// Hangul code points: the precomposed syllable block, how many syllables share
// one leading consonant (21 vowels × 28 finals), and the compatibility-jamo
// block users type on a Korean keyboard.
const (
	hangulSyllableFirst  = 0xAC00
	hangulSyllableLast   = 0xD7A3
	syllablesPerChoseong = 21 * 28
	compatJamoFirst      = 0x3131
	compatJamoLast       = 0x318E
)

// chosungTable maps Hangul choseong index (0..18) to its compatibility-jamo
// codepoint, which is the form users actually type on a Korean keyboard.
var chosungTable = []rune{
	'ㄱ', 'ㄲ', 'ㄴ', 'ㄷ', 'ㄸ', 'ㄹ', 'ㅁ', 'ㅂ', 'ㅃ', 'ㅅ',
	'ㅆ', 'ㅇ', 'ㅈ', 'ㅉ', 'ㅊ', 'ㅋ', 'ㅌ', 'ㅍ', 'ㅎ',
}

// toChosung folds a string into its 초성 (initial-consonant) form so that
// substring matching can implement 초성검색:
//
//   - Precomposed Hangul syllables collapse to their leading choseong jamo
//     (compat block, e.g. '한' → 'ㅎ').
//   - Compatibility jamo passes through.
//   - Everything else is lowercased so Latin / digit queries match against the
//     same canonical form without a separate code path.
func toChosung(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= hangulSyllableFirst && r <= hangulSyllableLast:
			idx := (r - hangulSyllableFirst) / syllablesPerChoseong
			b.WriteRune(chosungTable[idx])
		default:
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// hasJamo reports whether s contains at least one Hangul compatibility-jamo
// rune. That's the signal that the user is typing a 초성검색 query rather than
// ordinary text.
func hasJamo(s string) bool {
	for _, r := range s {
		if r >= compatJamoFirst && r <= compatJamoLast {
			return true
		}
	}
	return false
}
