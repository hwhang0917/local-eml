// Package textextract turns an attachment into plain text, so an AI agent
// connected over MCP can read a PDF or a Word file without downloading it.
// Everything here is pure Go and works offline: the bytes never leave the
// process. It reads text only; layout, images and formulas are dropped.
package textextract

import (
	"bytes"
	"errors"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/korean"
)

// ErrUnsupported means the file type has no text extractor. Images, audio,
// archives and the legacy binary Office formats (.doc, .xls, .ppt) land here.
var ErrUnsupported = errors.New("no text can be read from this file type")

// ErrProtected means the file is encrypted or locked for distribution.
var ErrProtected = errors.New("the file is password-protected or locked")

// MaxInput caps how large an attachment we try to read. Mail servers rarely
// pass more than 25-50 MB, and parsing is done in memory.
const MaxInput = 64 << 20

// format names what Extract recognised, for tests and error messages.
type format string

const (
	fmtText format = "text"
	fmtHTML format = "html"
	fmtDOCX format = "docx"
	fmtXLSX format = "xlsx"
	fmtPPTX format = "pptx"
	fmtHWPX format = "hwpx"
	fmtODF  format = "odf"
	fmtHWP  format = "hwp"
	fmtPDF  format = "pdf"
	fmtNone format = ""
)

var textExts = map[string]bool{
	"txt": true, "text": true, "csv": true, "tsv": true, "md": true, "markdown": true,
	"json": true, "xml": true, "log": true, "ics": true, "vcf": true, "yaml": true,
	"yml": true, "ini": true, "cfg": true, "conf": true,
}

func detect(filename, contentType string) format {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "pdf":
		return fmtPDF
	case "docx", "docm", "dotx":
		return fmtDOCX
	case "xlsx", "xlsm", "xltx":
		return fmtXLSX
	case "pptx", "pptm", "ppsx":
		return fmtPPTX
	case "hwpx":
		return fmtHWPX
	case "hwp":
		return fmtHWP
	case "odt", "ods", "odp":
		return fmtODF
	case "html", "htm":
		return fmtHTML
	}
	if textExts[ext] {
		return fmtText
	}
	mt, _, _ := mime.ParseMediaType(contentType)
	mt = strings.ToLower(mt)
	switch {
	case mt == "application/pdf":
		return fmtPDF
	case mt == "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
		return fmtDOCX
	case mt == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
		return fmtXLSX
	case mt == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
		return fmtPPTX
	case mt == "application/vnd.hancom.hwpx":
		return fmtHWPX
	case mt == "application/x-hwp", mt == "application/haansofthwp", mt == "application/vnd.hancom.hwp":
		return fmtHWP
	case strings.HasPrefix(mt, "application/vnd.oasis.opendocument."):
		return fmtODF
	case mt == "text/html", mt == "application/xhtml+xml":
		return fmtHTML
	case strings.HasPrefix(mt, "text/"), mt == "application/json", mt == "application/xml":
		return fmtText
	}
	return fmtNone
}

// Supported reports whether Extract has a reader for the file.
func Supported(filename, contentType string) bool {
	return detect(filename, contentType) != fmtNone
}

// Extract returns the text of an attachment. It never panics: the PDF and
// HWP parsers run on untrusted bytes, so a crash in one becomes an error.
func Extract(filename, contentType string, data []byte) (text string, err error) {
	f := detect(filename, contentType)
	if f == fmtNone {
		return "", ErrUnsupported
	}
	if len(data) > MaxInput {
		return "", fmt.Errorf("file is larger than %d MB", MaxInput>>20)
	}
	defer func() {
		if r := recover(); r != nil {
			text, err = "", fmt.Errorf("could not read %s: %v", f, r)
		}
	}()
	switch f {
	case fmtText:
		text = decodeText(data)
	case fmtHTML:
		text, err = htmlText(decodeText(data))
	case fmtDOCX:
		text, err = docxText(data)
	case fmtXLSX:
		text, err = xlsxText(data)
	case fmtPPTX:
		text, err = pptxText(data)
	case fmtHWPX:
		text, err = hwpxText(data)
	case fmtODF:
		text, err = odfText(data)
	case fmtHWP:
		text, err = hwpText(data)
	case fmtPDF:
		text, err = pdfText(data)
	}
	if err != nil {
		return "", err
	}
	return tidy(text), nil
}

// decodeText reads bytes as UTF-8, falling back to EUC-KR (CP949): Korean
// mail clients still attach legacy-encoded .txt and .csv files.
func decodeText(b []byte) string {
	b = bytes.TrimPrefix(b, []byte("\xef\xbb\xbf"))
	if utf8.Valid(b) {
		return string(b)
	}
	if out, err := korean.EUCKR.NewDecoder().Bytes(b); err == nil {
		return string(out)
	}
	return strings.ToValidUTF8(string(b), "�")
}

// tidy trims trailing spaces and collapses runs of blank lines, which the
// XML formats produce plenty of, so agents don't spend tokens on whitespace.
func tidy(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.TrimRight(l, " \t ")
		if l == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
