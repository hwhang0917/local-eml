package store

import "testing"

func TestToChosung(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"한국어", "ㅎㄱㅇ"},
		{"가나다", "ㄱㄴㄷ"},
		{"별표", "ㅂㅍ"},
		{"hwhang", "hwhang"},
		{"Hello 한", "hello ㅎ"},
		{"", ""},
		{"ㅎㄱ", "ㅎㄱ"},
	}
	for _, c := range cases {
		got := ToChosung(c.in)
		if got != c.want {
			t.Errorf("ToChosung(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestHasJamo(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"ㅎㄱ", true},
		{"hello ㅎ", true},
		{"한국어", false},
		{"hello", false},
		{"", false},
	}
	for _, c := range cases {
		got := HasJamo(c.in)
		if got != c.want {
			t.Errorf("HasJamo(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
