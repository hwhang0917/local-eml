package textextract

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/ledongthuc/pdf"
	"github.com/richardlehane/mscfb"
	"golang.org/x/net/html"
)

// maxPart caps how much of one zip member we inflate, so a zip bomb can't
// exhaust memory.
const maxPart = 64 << 20

func openZip(data []byte) (*zip.Reader, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("not a valid document: %w", err)
	}
	return zr, nil
}

func readMember(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(io.LimitReader(rc, maxPart))
		}
	}
	return nil, fmt.Errorf("%s is missing", name)
}

// members lists the zip entries matching re, ordered by the first number in
// their name (slide2 before slide10).
func members(zr *zip.Reader, re *regexp.Regexp) []string {
	var names []string
	for _, f := range zr.File {
		if re.MatchString(f.Name) {
			names = append(names, f.Name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return nameNum(names[i]) < nameNum(names[j]) })
	return names
}

var digits = regexp.MustCompile(`\d+`)

func nameNum(s string) int {
	n, _ := strconv.Atoi(digits.FindString(s))
	return n
}

// xmlText walks an XML document and collects the character data inside
// elements named textEl (any namespace), ending a line at each paraEl. With
// textEl empty, all character data counts. Word, PowerPoint and Hangul's
// HWPX all mark runs and paragraphs this way, only with different prefixes.
func xmlText(data []byte, textEl string, paraEl map[string]bool) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	dec.Strict = false
	var b strings.Builder
	depth := 0 // >0 while inside a text element
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("not a valid document: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case textEl:
				depth++
			case "tab":
				b.WriteByte('\t')
			case "br", "cr", "lineBreak", "line-break":
				b.WriteByte('\n')
			case "s": // ODF run of spaces
				b.WriteByte(' ')
			}
		case xml.EndElement:
			if t.Name.Local == textEl && depth > 0 {
				depth--
			}
			if paraEl[t.Name.Local] {
				b.WriteByte('\n')
			}
		case xml.CharData:
			if textEl == "" || depth > 0 {
				b.Write(t)
			}
		}
	}
	return b.String(), nil
}

var wordPara = map[string]bool{"p": true}

func docxText(data []byte) (string, error) {
	zr, err := openZip(data)
	if err != nil {
		return "", err
	}
	doc, err := readMember(zr, "word/document.xml")
	if err != nil {
		return "", err
	}
	return xmlText(doc, "t", wordPara)
}

var slideRe = regexp.MustCompile(`^ppt/slides/slide\d+\.xml$`)

func pptxText(data []byte) (string, error) {
	zr, err := openZip(data)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for i, name := range members(zr, slideRe) {
		raw, err := readMember(zr, name)
		if err != nil {
			return "", err
		}
		t, err := xmlText(raw, "t", wordPara)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "--- Slide %d ---\n%s\n\n", i+1, t)
	}
	return b.String(), nil
}

var hwpxSectionRe = regexp.MustCompile(`^Contents/section\d+\.xml$`)

func hwpxText(data []byte) (string, error) {
	zr, err := openZip(data)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, name := range members(zr, hwpxSectionRe) {
		raw, err := readMember(zr, name)
		if err != nil {
			return "", err
		}
		t, err := xmlText(raw, "t", wordPara)
		if err != nil {
			return "", err
		}
		b.WriteString(t)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

var odfPara = map[string]bool{"p": true, "h": true, "table-row": true}

func odfText(data []byte) (string, error) {
	zr, err := openZip(data)
	if err != nil {
		return "", err
	}
	content, err := readMember(zr, "content.xml")
	if err != nil {
		return "", err
	}
	return xmlText(content, "", odfPara)
}

// xlsxText prints each sheet as tab-separated rows under its name.
func xlsxText(data []byte) (string, error) {
	zr, err := openZip(data)
	if err != nil {
		return "", err
	}
	shared, err := xlsxShared(zr)
	if err != nil {
		return "", err
	}
	sheets, err := xlsxSheets(zr)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, sh := range sheets {
		raw, err := readMember(zr, sh.path)
		if err != nil {
			continue // a dangling relationship; skip that sheet
		}
		fmt.Fprintf(&b, "--- Sheet: %s ---\n", sh.name)
		if err := xlsxRows(raw, shared, &b); err != nil {
			return "", err
		}
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func xlsxShared(zr *zip.Reader) ([]string, error) {
	raw, err := readMember(zr, "xl/sharedStrings.xml")
	if err != nil {
		return nil, nil // a workbook of only numbers has none
	}
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if err := xml.Unmarshal(raw, &sst); err != nil {
		return nil, fmt.Errorf("not a valid workbook: %w", err)
	}
	out := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		s := si.T
		for _, r := range si.R {
			s += r.T
		}
		out[i] = s
	}
	return out, nil
}

type xlsxSheet struct{ name, path string }

func xlsxSheets(zr *zip.Reader) ([]xlsxSheet, error) {
	raw, err := readMember(zr, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	var wb struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.Unmarshal(raw, &wb); err != nil {
		return nil, fmt.Errorf("not a valid workbook: %w", err)
	}
	targets := map[string]string{}
	if rels, err := readMember(zr, "xl/_rels/workbook.xml.rels"); err == nil {
		var r struct {
			Rel []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		if xml.Unmarshal(rels, &r) == nil {
			for _, x := range r.Rel {
				t := strings.TrimPrefix(x.Target, "/")
				if !strings.HasPrefix(t, "xl/") {
					t = "xl/" + t
				}
				targets[x.ID] = t
			}
		}
	}
	out := make([]xlsxSheet, 0, len(wb.Sheets))
	for i, s := range wb.Sheets {
		p, ok := targets[s.RID]
		if !ok {
			p = fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)
		}
		out = append(out, xlsxSheet{name: s.Name, path: p})
	}
	return out, nil
}

func xlsxRows(raw []byte, shared []string, b *strings.Builder) error {
	var ws struct {
		Rows []struct {
			Cells []struct {
				Ref  string `xml:"r,attr"`
				Type string `xml:"t,attr"`
				V    string `xml:"v"`
				IS   struct {
					T string `xml:"t"`
					R []struct {
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(raw, &ws); err != nil {
		return fmt.Errorf("not a valid worksheet: %w", err)
	}
	for _, row := range ws.Rows {
		var cells []string
		for _, c := range row.Cells {
			v := c.V
			switch c.Type {
			case "s":
				if i, err := strconv.Atoi(c.V); err == nil && i >= 0 && i < len(shared) {
					v = shared[i]
				}
			case "inlineStr":
				v = c.IS.T
				for _, r := range c.IS.R {
					v += r.T
				}
			case "b":
				v = map[string]string{"1": "TRUE", "0": "FALSE"}[c.V]
			}
			// Keep columns aligned across gaps: B1 after an empty A1 stays
			// in the second column.
			if col := colIndex(c.Ref); col > len(cells) {
				for len(cells) < col {
					cells = append(cells, "")
				}
			}
			cells = append(cells, strings.ReplaceAll(v, "\n", " "))
		}
		if strings.TrimSpace(strings.Join(cells, "")) == "" {
			continue
		}
		b.WriteString(strings.Join(cells, "\t"))
		b.WriteByte('\n')
	}
	return nil
}

// colIndex turns the letters of a cell reference into a 0-based column; a
// missing reference returns -1 so the cell just follows the previous one.
func colIndex(ref string) int {
	n := 0
	seen := false
	for _, r := range ref {
		if r < 'A' || r > 'Z' {
			break
		}
		n = n*26 + int(r-'A'+1)
		seen = true
	}
	if !seen {
		return -1
	}
	return n - 1
}

// htmlText keeps the visible text of an HTML document, breaking lines at
// block elements.
func htmlText(s string) (string, error) {
	z := html.NewTokenizer(strings.NewReader(s))
	var b strings.Builder
	skip := 0
	for {
		switch z.Next() {
		case html.ErrorToken:
			if errors.Is(z.Err(), io.EOF) {
				return b.String(), nil
			}
			return "", z.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "noscript", "template":
				skip++
			case "br", "p", "div", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6":
				b.WriteByte('\n')
			case "td", "th":
				b.WriteByte('\t')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "noscript", "template":
				if skip > 0 {
					skip--
				}
			case "p", "div", "table", "ul", "ol":
				b.WriteByte('\n')
			}
		case html.TextToken:
			if skip == 0 {
				// Collapse whitespace like a browser, but keep one space
				// at either edge so "Hello <b>there</b>" stays two words.
				t := string(z.Text())
				words := strings.Join(strings.Fields(t), " ")
				if words == "" {
					if t != "" {
						b.WriteByte(' ')
					}
					continue
				}
				if t[0] == ' ' || t[0] == '\n' || t[0] == '\t' {
					b.WriteByte(' ')
				}
				b.WriteString(words)
				if c := t[len(t)-1]; c == ' ' || c == '\n' || c == '\t' {
					b.WriteByte(' ')
				}
			}
		}
	}
}

func pdfText(data []byte) (string, error) {
	r, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		if strings.Contains(err.Error(), "encrypt") {
			return "", ErrProtected
		}
		return "", fmt.Errorf("not a valid PDF: %w", err)
	}
	var b strings.Builder
	fonts := map[string]*pdf.Font{}
	for i := 1; i <= r.NumPage(); i++ {
		p := r.Page(i)
		if p.V.IsNull() {
			continue
		}
		for _, name := range p.Fonts() {
			if _, ok := fonts[name]; !ok {
				f := p.Font(name)
				fonts[name] = &f
			}
		}
		t, err := p.GetPlainText(fonts)
		if err != nil {
			continue // one unreadable page shouldn't hide the rest
		}
		fmt.Fprintf(&b, "--- Page %d ---\n%s\n\n", i, t)
	}
	return b.String(), nil
}

// HWP 5.0 is an OLE compound file. Paragraph text sits in BodyText/SectionN
// streams as tagged records, raw-deflated when the header says so. See the
// format notes Hancom publishes ("한글 문서 파일 형식 5.0").
const (
	hwpTagParaText  = 0x10 + 51
	hwpFlagCompress = 1 << 0
	hwpFlagPassword = 1 << 1
	hwpFlagDistrib  = 1 << 2
)

func hwpText(data []byte) (string, error) {
	doc, err := mscfb.New(bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("not a valid HWP file: %w", err)
	}
	var flags uint32
	headerSeen := false
	sections := map[int][]byte{}
	for e, err := doc.Next(); err == nil; e, err = doc.Next() {
		switch {
		case e.Name == "FileHeader" && len(e.Path) == 0:
			h := make([]byte, 40)
			if _, err := io.ReadFull(e, h); err != nil {
				return "", fmt.Errorf("not a valid HWP file: %w", err)
			}
			if !bytes.HasPrefix(h, []byte("HWP Document File")) {
				return "", errors.New("not a valid HWP file")
			}
			flags = binary.LittleEndian.Uint32(h[36:40])
			headerSeen = true
		case len(e.Path) == 1 && e.Path[0] == "BodyText" && strings.HasPrefix(e.Name, "Section"):
			n, convErr := strconv.Atoi(strings.TrimPrefix(e.Name, "Section"))
			if convErr != nil {
				continue
			}
			raw, err := io.ReadAll(io.LimitReader(e, maxPart))
			if err != nil {
				return "", err
			}
			sections[n] = raw
		}
	}
	if !headerSeen {
		return "", errors.New("not a valid HWP file")
	}
	if flags&(hwpFlagPassword|hwpFlagDistrib) != 0 {
		return "", ErrProtected
	}
	keys := make([]int, 0, len(sections))
	for k := range sections {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var b strings.Builder
	for _, k := range keys {
		raw := sections[k]
		if flags&hwpFlagCompress != 0 {
			raw, err = io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(raw)), maxPart))
			if err != nil && len(raw) == 0 {
				return "", fmt.Errorf("not a valid HWP file: %w", err)
			}
		}
		hwpRecords(raw, &b)
	}
	return b.String(), nil
}

func hwpRecords(raw []byte, b *strings.Builder) {
	for len(raw) >= 4 {
		h := binary.LittleEndian.Uint32(raw)
		raw = raw[4:]
		tag := h & 0x3ff
		size := int(h >> 20)
		if size == 0xfff {
			if len(raw) < 4 {
				return
			}
			size = int(binary.LittleEndian.Uint32(raw))
			raw = raw[4:]
		}
		if size < 0 || size > len(raw) {
			return
		}
		if tag == hwpTagParaText {
			b.WriteString(hwpParaText(raw[:size]))
			b.WriteByte('\n')
		}
		raw = raw[size:]
	}
}

// hwpParaText decodes one paragraph's UTF-16LE text. Codes below 32 are
// controls: most take 8 code units (an inline or extended control such as a
// table or footnote anchor), a few take one.
func hwpParaText(p []byte) string {
	u := make([]uint16, 0, len(p)/2)
	for i := 0; i+1 < len(p); i += 2 {
		u = append(u, binary.LittleEndian.Uint16(p[i:]))
	}
	var out []uint16
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c >= 32 {
			out = append(out, c)
			continue
		}
		switch c {
		case 0, 13: // unused, paragraph end
		case 10:
			out = append(out, '\n')
		case 24, 25, 26, 27, 28, 29, 30, 31: // hyphen and fixed-width spaces
			out = append(out, ' ')
		case 9:
			out = append(out, '\t')
			i += 7
		default:
			i += 7
		}
	}
	return string(utf16.Decode(out))
}
