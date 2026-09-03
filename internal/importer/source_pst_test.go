package importer

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jhillyerd/enmime"

	"github.com/hwhang0917/local-eml/internal/store"
)

// go-pst ships sample archives; we borrow its 1 MB support.pst from the module
// cache instead of vendoring a copy (it contains other people's mail).
func samplePst(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/mooijtech/go-pst/v6").Output()
	if err != nil {
		t.Skipf("go-pst module dir unavailable: %v", err)
	}
	p := filepath.Join(strings.TrimSpace(string(out)), "data", "support.pst")
	if _, err := os.Stat(p); err != nil {
		t.Skipf("sample pst missing: %v", err)
	}
	return p
}

func TestPstSourceRebuildsMessages(t *testing.T) {
	src := NewPstSource(samplePst(t))
	defer src.Close()
	items, err := src.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 17 {
		t.Fatalf("want 17 messages, got %d", len(items))
	}
	if items[0].Name != "support.pst#00001.eml" {
		t.Errorf("name = %q", items[0].Name)
	}

	var withAttachments int
	for i, it := range items {
		env, err := enmime.ReadEnvelope(strings.NewReader(readItem(t, it)))
		if err != nil {
			t.Fatalf("item %d: %v", i, err)
		}
		if env.GetHeader("From") == "" || env.GetHeader("Date") == "" || env.GetHeader("Message-Id") == "" {
			t.Errorf("item %d missing core headers: from=%q date=%q id=%q",
				i, env.GetHeader("From"), env.GetHeader("Date"), env.GetHeader("Message-Id"))
		}
		if env.GetHeader("Content-Type") == "" || !strings.HasPrefix(env.GetHeader("Content-Type"), "multipart/mixed") {
			t.Errorf("item %d content-type = %q", i, env.GetHeader("Content-Type"))
		}
		if len(env.Attachments) > 0 {
			withAttachments++
		}
	}
	if withAttachments != 1 {
		t.Errorf("want exactly 1 message with attachments, got %d", withAttachments)
	}

	raw := readItem(t, items[0])
	if again := readItem(t, items[0]); again != raw {
		t.Error("rebuilding the same message must give identical bytes (dedup relies on it)")
	}
	first, err := enmime.ReadEnvelope(strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if got := first.GetHeader("Subject"); got != "Desktop exploits suspension notice" {
		t.Errorf("subject = %q", got)
	}
	if got := first.GetHeader("From"); !strings.Contains(got, "support@hackingteam.com") {
		t.Errorf("from = %q", got)
	}
}

func TestPstSourceImportsAndDedupes(t *testing.T) {
	path := samplePst(t)
	st := newTestStore(t)
	im := &Importer{Store: st, Paths: newTestPaths(t)}
	ctx := context.Background()

	runOnce := func(id string) *store.Import {
		if err := st.CreateImport(ctx, importStub(id)); err != nil {
			t.Fatal(err)
		}
		src := NewPstSource(path)
		defer src.Close()
		job := &Job{Importer: im, Hub: NewHub(), Store: st, ID: id}
		job.RunSource(ctx, src)
		imp, err := st.GetImport(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return imp
	}

	first := runOnce("pst1")
	if first.Processed != 17 || first.Errors != 0 || first.Duplicates != 0 {
		t.Fatalf("first run: %+v", first)
	}
	second := runOnce("pst2")
	if second.Duplicates != 17 || second.Errors != 0 {
		t.Fatalf("second run should be all duplicates: %+v", second)
	}
}

func TestPstSourceScanFailsOnMissingFile(t *testing.T) {
	src := NewPstSource(filepath.Join(t.TempDir(), "nope.pst"))
	if _, err := src.Scan(context.Background()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want ErrNotExist", err)
	}
}
