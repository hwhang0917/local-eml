package server

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jhillyerd/enmime"

	"github.com/hwhang0917/local-eml/internal/parser"
	"github.com/hwhang0917/local-eml/internal/sanitize"
	"github.com/hwhang0917/local-eml/internal/store"
)

func (s *Server) handleListEmails(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := store.ListOptions{
		Query:       q.Get("q"),
		StarredOnly: q.Get("starred") == "1",
		Sort:        q.Get("sort"),
		Order:       q.Get("order"),
		Limit:       intParam(q.Get("limit"), 50),
		Offset:      intParam(q.Get("offset"), 0),
	}
	emails, total, err := s.Store.ListEmails(r.Context(), opts)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"total":  total,
		"limit":  opts.Limit,
		"offset": opts.Offset,
		"items":  emails,
	})
}

func (s *Server) handleGetEmail(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	e, err := s.Store.GetEmailBySHA(r.Context(), sha)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) handleEmailRaw(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	if !validSHA(sha) {
		http.Error(w, "invalid sha", http.StatusBadRequest)
		return
	}
	f, err := os.Open(s.Importer.Paths.BlobFor(sha))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "message/rfc822")
	_, _ = io.Copy(w, f)
}

type partInfo struct {
	Index       int    `json:"index"`
	ContentID   string `json:"content_id,omitempty"`
	ContentType string `json:"content_type"`
	Filename    string `json:"filename,omitempty"`
	Size        int    `json:"size"`
}

func (s *Server) handleEmailParts(w http.ResponseWriter, r *http.Request) {
	env, err := s.openEnvelope(chi.URLParam(r, "sha"))
	if err != nil {
		httpEnvErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"has_text":    env.Text != "",
		"has_html":    env.HTML != "",
		"inlines":     partsManifest(env.Inlines),
		"attachments": partsManifest(env.Attachments),
	})
}

func partsManifest(parts []*enmime.Part) []partInfo {
	out := make([]partInfo, 0, len(parts))
	for i, p := range parts {
		out = append(out, partInfo{
			Index:       i,
			ContentID:   strings.Trim(p.Header.Get("Content-ID"), "<>"),
			ContentType: p.ContentType,
			Filename:    p.FileName,
			Size:        len(p.Content),
		})
	}
	return out
}

func (s *Server) handleEmailText(w http.ResponseWriter, r *http.Request) {
	env, err := s.openEnvelope(chi.URLParam(r, "sha"))
	if err != nil {
		httpEnvErr(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, env.Text)
}

func (s *Server) handleEmailHTML(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	env, err := s.openEnvelope(sha)
	if err != nil {
		httpEnvErr(w, err)
		return
	}
	if env.HTML == "" {
		http.Error(w, "no html part", http.StatusNotFound)
		return
	}
	showRemote := r.URL.Query().Get("remote") == "1"
	out, err := sanitize.Sanitize(env.HTML, sanitize.Options{
		CIDBaseURL: fmt.Sprintf("/api/emails/%s/cid/", sha),
		ShowRemote: showRemote,
		Lang:       r.URL.Query().Get("lang"),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", csp(showRemote))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = io.WriteString(w, out)
}

func csp(showRemote bool) string {
	imgSrc := "img-src 'self' data:"
	if showRemote {
		imgSrc += " http: https:"
	}
	return strings.Join([]string{
		"default-src 'none'",
		imgSrc,
		"style-src 'unsafe-inline'",
		"font-src 'self' data:",
	}, "; ")
}

func (s *Server) handleEmailCID(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	cid := chi.URLParam(r, "cid")
	env, err := s.openEnvelope(sha)
	if err != nil {
		httpEnvErr(w, err)
		return
	}
	part := findPartByCID(env, cid)
	if part == nil {
		http.Error(w, "cid not found", http.StatusNotFound)
		return
	}
	// Content-Type here comes straight from the message, so a crafted email
	// could declare an inline part as text/html and get it served from our own
	// origin. Inside the viewer the iframe sandbox contains that, but "open
	// image in new tab" is an unsandboxed top-level navigation — stored XSS
	// with full API access. This endpoint only ever backs <img src="cid:...">,
	// so images are the only thing worth serving.
	mediaType, _, err := mime.ParseMediaType(part.ContentType)
	if err != nil || !strings.HasPrefix(mediaType, "image/") {
		http.Error(w, "cid part is not an image", http.StatusUnsupportedMediaType)
		return
	}
	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", "inline")
	// image/svg+xml is a legitimate image that can also carry <script>. The
	// sandbox directive makes it inert if navigated to directly, without
	// breaking it as an <img> source.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; sandbox")
	w.Header().Set("Cache-Control", "private, max-age=300")
	_, _ = w.Write(part.Content)
}

func (s *Server) handleEmailAttachment(w http.ResponseWriter, r *http.Request) {
	sha := chi.URLParam(r, "sha")
	idx, err := strconv.Atoi(chi.URLParam(r, "idx"))
	if err != nil {
		http.Error(w, "invalid index", http.StatusBadRequest)
		return
	}
	env, err := s.openEnvelope(sha)
	if err != nil {
		httpEnvErr(w, err)
		return
	}
	if idx < 0 || idx >= len(env.Attachments) {
		http.Error(w, "out of range", http.StatusNotFound)
		return
	}
	p := env.Attachments[idx]
	filename := p.FileName
	if filename == "" {
		filename = fmt.Sprintf("attachment-%d", idx)
	}
	w.Header().Set("Content-Type", p.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf(`attachment; filename="%s"`, safeFilename(filename)))
	_, _ = w.Write(p.Content)
}

type envErr struct {
	status int
	err    error
}

func (e *envErr) Error() string { return e.err.Error() }

func (s *Server) openEnvelope(sha string) (*enmime.Envelope, error) {
	if !validSHA(sha) {
		return nil, &envErr{status: http.StatusBadRequest, err: fmt.Errorf("invalid sha")}
	}
	f, err := os.Open(s.Importer.Paths.BlobFor(sha))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &envErr{status: http.StatusNotFound, err: fmt.Errorf("not found")}
		}
		return nil, &envErr{status: http.StatusInternalServerError, err: err}
	}
	defer f.Close()
	env, err := parser.Open(f)
	if err != nil {
		return nil, &envErr{status: http.StatusInternalServerError, err: fmt.Errorf("parse: %w", err)}
	}
	return env, nil
}

func httpEnvErr(w http.ResponseWriter, err error) {
	if e, ok := err.(*envErr); ok {
		http.Error(w, e.err.Error(), e.status)
		return
	}
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func findPartByCID(env *enmime.Envelope, cid string) *enmime.Part {
	target := strings.ToLower(strings.Trim(cid, "<>"))
	check := func(parts []*enmime.Part) *enmime.Part {
		for _, p := range parts {
			id := strings.ToLower(strings.Trim(p.Header.Get("Content-ID"), "<>"))
			if id == target {
				return p
			}
		}
		return nil
	}
	if p := check(env.Inlines); p != nil {
		return p
	}
	if p := check(env.OtherParts); p != nil {
		return p
	}
	if p := check(env.Attachments); p != nil {
		return p
	}
	return nil
}

func safeFilename(s string) string {
	r := strings.NewReplacer("\"", "", "\n", "", "\r", "", "\\", "")
	return r.Replace(s)
}

func intParam(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

func validSHA(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
