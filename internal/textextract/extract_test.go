package textextract

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"math/rand"
	"os"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/text/encoding/korean"
)

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func mustExtract(t *testing.T, name, ct string, data []byte) string {
	t.Helper()
	got, err := Extract(name, ct, data)
	if err != nil {
		t.Fatalf("Extract(%s): %v", name, err)
	}
	return got
}

func TestPlainTextAndLegacyKorean(t *testing.T) {
	if got := mustExtract(t, "a.txt", "", []byte("\xef\xbb\xbfhello\n\n\n\nworld  ")); got != "hello\n\nworld" {
		t.Errorf("utf-8: %q", got)
	}
	euckr, _ := korean.EUCKR.NewEncoder().Bytes([]byte("임대료,금액\n월세,120만원"))
	if got := mustExtract(t, "rent.csv", "text/csv", euckr); got != "임대료,금액\n월세,120만원" {
		t.Errorf("euc-kr csv: %q", got)
	}
}

func TestHTML(t *testing.T) {
	got := mustExtract(t, "page.html", "", []byte(
		`<html><head><title>x</title><style>p{}</style></head><body><p>Hello <b>there</b></p><script>alert(1)</script><p>Bye</p></body></html>`))
	if got != "Hello there\n\nBye" {
		t.Errorf("html: %q", got)
	}
}

func TestDOCX(t *testing.T) {
	doc := zipOf(t, map[string]string{"word/document.xml": `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>
<w:p><w:r><w:t>Rent goes up</w:t></w:r><w:r><w:t xml:space="preserve"> 5%</w:t></w:r></w:p>
<w:p><w:r><w:t>Term:</w:t><w:tab/><w:t>2 years</w:t></w:r></w:p>
<w:p><w:r><w:instrText>IGNORED FIELD</w:instrText></w:r></w:p>
</w:body></w:document>`})
	got := mustExtract(t, "lease.docx", "", doc)
	if got != "Rent goes up 5%\nTerm:\t2 years" {
		t.Errorf("docx: %q", got)
	}
}

func TestPPTXSlideOrder(t *testing.T) {
	slide := func(s string) string {
		return `<p:sld xmlns:p="p" xmlns:a="a"><a:p><a:r><a:t>` + s + `</a:t></a:r></a:p></p:sld>`
	}
	deck := zipOf(t, map[string]string{
		"ppt/slides/slide10.xml": slide("ten"),
		"ppt/slides/slide2.xml":  slide("two"),
		"ppt/slides/slide1.xml":  slide("one"),
	})
	got := mustExtract(t, "deck.pptx", "", deck)
	if i1, i2, i10 := strings.Index(got, "one"), strings.Index(got, "two"), strings.Index(got, "ten"); !(i1 < i2 && i2 < i10) {
		t.Errorf("slides out of order: %q", got)
	}
	if !strings.Contains(got, "--- Slide 3 ---\nten") {
		t.Errorf("slide headers: %q", got)
	}
}

func TestXLSX(t *testing.T) {
	book := zipOf(t, map[string]string{
		"xl/workbook.xml": `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="Budget" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/></Relationships>`,
		"xl/sharedStrings.xml":       `<sst><si><t>Item</t></si><si><t>Cost</t></si><si><r><t>Ren</t></r><r><t>t</t></r></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
<row r="2"><c r="A2" t="s"><v>2</v></c><c r="C2"><v>1200</v></c></row>
<row r="3"><c r="B3" t="inlineStr"><is><t>note</t></is></c><c r="C3" t="b"><v>1</v></c></row>
</sheetData></worksheet>`,
	})
	got := mustExtract(t, "budget.xlsx", "", book)
	want := "--- Sheet: Budget ---\nItem\tCost\nRent\t\t1200\n\tnote\tTRUE"
	if got != want {
		t.Errorf("xlsx:\n got %q\nwant %q", got, want)
	}
}

func TestHWPX(t *testing.T) {
	sec := func(s string) string {
		return `<hs:sec xmlns:hs="s" xmlns:hp="p"><hp:p><hp:run><hp:t>` + s + `</hp:t></hp:run></hp:p></hs:sec>`
	}
	doc := zipOf(t, map[string]string{
		"Contents/section1.xml": sec("둘째 구역"),
		"Contents/section0.xml": sec("임대차 계약서"),
	})
	if got := mustExtract(t, "계약.hwpx", "", doc); got != "임대차 계약서\n\n둘째 구역" {
		t.Errorf("hwpx: %q", got)
	}
}

func TestODT(t *testing.T) {
	doc := zipOf(t, map[string]string{"content.xml": `<office:document-content xmlns:office="o" xmlns:text="t">
<office:body><office:text><text:h>Title</text:h><text:p>Body<text:s/>text</text:p></office:text></office:body></office:document-content>`})
	if got := mustExtract(t, "a.odt", "", doc); got != "Title\nBody text" {
		t.Errorf("odt: %q", got)
	}
}

func TestPDF(t *testing.T) {
	data, err := os.ReadFile("testdata/lease.pdf")
	if err != nil {
		t.Fatal(err)
	}
	got := mustExtract(t, "lease.pdf", "application/pdf", data)
	for _, want := range []string{"--- Page 1 ---", "Lease renewal terms", "1,200,000"} {
		if !strings.Contains(got, want) {
			t.Errorf("pdf text missing %q: %q", want, got)
		}
	}
}

func TestUnsupportedAndBroken(t *testing.T) {
	if _, err := Extract("photo.jpg", "image/jpeg", []byte{0xff, 0xd8}); !errors.Is(err, ErrUnsupported) {
		t.Errorf("jpg: %v", err)
	}
	if Supported("old.doc", "application/msword") {
		t.Error("legacy .doc should be unsupported")
	}
	for _, name := range []string{"x.docx", "x.xlsx", "x.pdf", "x.hwp", "x.hwpx"} {
		if _, err := Extract(name, "", []byte("not really")); err == nil {
			t.Errorf("%s: garbage should fail", name)
		}
	}
}

// --- HWP 5.0 ---

// cfb writes a minimal compound file (version 3, 512-byte sectors) holding
// FileHeader and BodyText/Section0. Both streams must be at least 4096 bytes
// so they live in regular sectors rather than the mini stream, which keeps
// this writer short.
func cfb(t *testing.T, fileHeader, section []byte) []byte {
	t.Helper()
	pad := func(b []byte) []byte {
		if len(b) < 4096 {
			b = append(b, make([]byte, 4096-len(b))...)
		}
		return b
	}
	fileHeader, section = pad(fileHeader), pad(section)
	const (
		sec       = 512
		endChain  = 0xFFFFFFFE
		freeSect  = 0xFFFFFFFF
		fatSect   = 0xFFFFFFFD
		noStream  = 0xFFFFFFFF
		dirSector = 1
	)
	sectors := func(n int) int { return (n + sec - 1) / sec }
	fhStart := 2
	secStart := fhStart + sectors(len(fileHeader))
	total := secStart + sectors(len(section))
	if total > sec/4 {
		t.Fatal("test file too big for one FAT sector")
	}

	fat := make([]uint32, sec/4)
	for i := range fat {
		fat[i] = freeSect
	}
	fat[0] = fatSect
	fat[dirSector] = endChain
	chain := func(start, n int) {
		for i := 0; i < n; i++ {
			if i == n-1 {
				fat[start+i] = endChain
			} else {
				fat[start+i] = uint32(start + i + 1)
			}
		}
	}
	chain(fhStart, sectors(len(fileHeader)))
	chain(secStart, sectors(len(section)))

	hdr := make([]byte, sec)
	copy(hdr, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	le := binary.LittleEndian
	le.PutUint16(hdr[0x18:], 0x3E)
	le.PutUint16(hdr[0x1A:], 3)
	le.PutUint16(hdr[0x1C:], 0xFFFE)
	le.PutUint16(hdr[0x1E:], 9)
	le.PutUint16(hdr[0x20:], 6)
	le.PutUint32(hdr[0x2C:], 1)
	le.PutUint32(hdr[0x30:], dirSector)
	le.PutUint32(hdr[0x38:], 4096)
	le.PutUint32(hdr[0x3C:], endChain)
	le.PutUint32(hdr[0x44:], endChain)
	for i := 0; i < 109; i++ {
		le.PutUint32(hdr[0x4C+4*i:], freeSect)
	}
	le.PutUint32(hdr[0x4C:], 0)

	dir := make([]byte, sec)
	entry := func(i int, name string, typ byte, left, right, child uint32, start, size int) {
		e := dir[i*128 : (i+1)*128]
		u := utf16.Encode([]rune(name))
		for j, c := range u {
			le.PutUint16(e[2*j:], c)
		}
		le.PutUint16(e[0x40:], uint16(2*len(u)+2))
		e[0x42] = typ
		e[0x43] = 1
		le.PutUint32(e[0x44:], left)
		le.PutUint32(e[0x48:], right)
		le.PutUint32(e[0x4C:], child)
		le.PutUint32(e[0x74:], uint32(start))
		le.PutUint64(e[0x78:], uint64(size))
	}
	// Siblings sort by name length first, so BodyText is FileHeader's left.
	entry(0, "Root Entry", 5, noStream, noStream, 1, endChain, 0)
	entry(1, "FileHeader", 2, 2, noStream, noStream, fhStart, len(fileHeader))
	entry(2, "BodyText", 1, noStream, noStream, 3, 0, 0)
	entry(3, "Section0", 2, noStream, noStream, noStream, secStart, len(section))

	out := append([]byte{}, hdr...)
	fatBytes := make([]byte, sec)
	for i, v := range fat {
		le.PutUint32(fatBytes[4*i:], v)
	}
	out = append(out, fatBytes...)
	out = append(out, dir...)
	for _, s := range [][]byte{fileHeader, section} {
		b := append([]byte{}, s...)
		if r := len(b) % sec; r != 0 {
			b = append(b, make([]byte, sec-r)...)
		}
		out = append(out, b...)
	}
	return out
}

func hwpRecord(tag uint32, data []byte) []byte {
	var b []byte
	if len(data) >= 0xfff {
		b = binary.LittleEndian.AppendUint32(b, tag|0xfff<<20)
		b = binary.LittleEndian.AppendUint32(b, uint32(len(data)))
	} else {
		b = binary.LittleEndian.AppendUint32(b, tag|uint32(len(data))<<20)
	}
	return append(b, data...)
}

func paraText(units ...uint16) []byte {
	var b []byte
	for _, u := range units {
		b = binary.LittleEndian.AppendUint16(b, u)
	}
	return b
}

func utf16s(s string) []uint16 { return utf16.Encode([]rune(s)) }

func hwpSection(t *testing.T) []byte {
	// "임대료" TAB "인상" CR, where TAB is an 8-unit inline control, then a
	// paragraph with a table anchor (extended control 11) before its text.
	p1 := append(utf16s("임대료"), 9, 0, 0, 0, 0, 0, 0, 9)
	p1 = append(p1, utf16s("인상")...)
	p1 = append(p1, 13)
	p2 := append([]uint16{11, 1, 2, 3, 4, 5, 6, 11}, utf16s("5% 인상")...)
	p2 = append(p2, 13)
	// A large binary record of another tag: real files carry many, and it
	// keeps the stream above the mini-stream cutoff even when deflated.
	filler := make([]byte, 6000)
	rand.New(rand.NewSource(1)).Read(filler)
	var s []byte
	s = append(s, hwpRecord(0x10+50, []byte{0, 0, 0, 0})...) // PARA_HEADER
	s = append(s, hwpRecord(hwpTagParaText, paraText(p1...))...)
	s = append(s, hwpRecord(0x10+50, []byte{0, 0, 0, 0})...)
	s = append(s, hwpRecord(hwpTagParaText, paraText(p2...))...)
	s = append(s, hwpRecord(0x10+60, filler)...)
	return s
}

func hwpHeader(flags uint32) []byte {
	h := make([]byte, 256)
	copy(h, "HWP Document File")
	binary.LittleEndian.PutUint32(h[32:], 0x05000300)
	binary.LittleEndian.PutUint32(h[36:], flags)
	return h
}

func TestHWP(t *testing.T) {
	plain := hwpSection(t)
	var deflated bytes.Buffer
	fw, _ := flate.NewWriter(&deflated, flate.BestCompression)
	_, _ = fw.Write(plain)
	_ = fw.Close()

	for name, file := range map[string][]byte{
		"uncompressed": cfb(t, hwpHeader(0), plain),
		"compressed":   cfb(t, hwpHeader(hwpFlagCompress), deflated.Bytes()),
	} {
		got, err := Extract("계약.hwp", "application/x-hwp", file)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got != "임대료\t인상\n5% 인상" {
			t.Errorf("%s: %q", name, got)
		}
	}

	if _, err := Extract("locked.hwp", "", cfb(t, hwpHeader(hwpFlagPassword), plain)); !errors.Is(err, ErrProtected) {
		t.Errorf("password-protected: %v", err)
	}
}
