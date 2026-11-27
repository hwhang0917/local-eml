package exporter

import "testing"

func TestCSVTextDefusesFormulas(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "",
		"Hello":                  "Hello",
		"Re: =1+1":               "Re: =1+1",
		`=HYPERLINK("http://x")`: `'=HYPERLINK("http://x")`,
		"+1":                     "'+1",
		"-2+3":                   "'-2+3",
		"@SUM(A1)":               "'@SUM(A1)",
		"\t=1":                   "'\t=1",
	} {
		if got := csvText(in); got != want {
			t.Errorf("csvText(%q) = %q, want %q", in, got, want)
		}
	}
}
