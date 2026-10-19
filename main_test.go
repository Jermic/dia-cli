package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func stripTerminal(s string) string {
	s = regexp.MustCompile(`\x1b]8;;[^\x1b]*\x1b\\`).ReplaceAllString(s, "")
	return regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(s, "")
}

func TestMovePreviewPrintedBeforeSummary(t *testing.T) {
	var out, stderr bytes.Buffer
	status := run([]string{"md", "example.com", "Work", "-n"}, &out, &stderr, func(Command) (Result, error) {
		return Result{Output: "ACTION\tWINDOW\tTAB\tSOURCE\tTARGET\tURL\nMOVE\tw1\ta\tPersonal\tWork\thttps://example.com", Move: &MoveResult{Matched: 1, Remaining: 1}}, nil
	}, nil, false)
	if status != 0 || !strings.Contains(out.String(), "MOVE\tw1\ta") || !strings.Contains(out.String(), "dry-run matched=1") || strings.Index(out.String(), "MOVE") > strings.Index(out.String(), "dry-run") {
		t.Fatalf("preview missing or ordered after summary: %q %q", out.String(), stderr.String())
	}
}

func TestRunFindAndCurrentShowURLs(t *testing.T) {
	for _, name := range []string{"find", "current"} {
		args := []string{name}
		if name == "find" {
			args = append(args, "title")
		}
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command) (Result, error) {
			return Result{Output: "WINDOW\tTAB\tTITLE\tURL\nw1\ta\tTitle\thttps://example.com/path"}, nil
		}, &rowCache{path: filepath.Join(t.TempDir(), "rows")}, true)
		plain := stripTerminal(out.String())
		if status != 0 || !strings.Contains(plain, "URL") || !strings.Contains(plain, "https://example.com/path") {
			t.Fatalf("%s visible URL missing: %q %q", name, plain, stderr.String())
		}
	}
}

func TestRunProfilesAlignsTerminalAndPreservesPipedTSV(t *testing.T) {
	counts := "WINDOW\tINDEX\tPROFILE\tCOUNT\nw1\t1\tDay\t1\nw1\t2\tReadLater\t27\nw1\t12\t中文档\t785"
	plain := "WINDOW  INDEX  PROFILE    COUNT\n" +
		"w1          1  Day            1\n" +
		"w1          2  ReadLater     27\n" +
		"w1         12  中文档       785\n"
	noCounts := "WINDOW\tINDEX\tPROFILE\nw1\t1\tDay\nw1\t2\tReadLater\nw1\t12\t中文档"
	noCountsPlain := "WINDOW  INDEX  PROFILE\n" +
		"w1          1  Day\n" +
		"w1          2  ReadLater\n" +
		"w1         12  中文档\n"
	for _, tc := range []struct {
		args        []string
		tsv, want   string
		interactive bool
	}{
		{[]string{"ps", "-c"}, counts, plain, true},
		{[]string{"ps", "-c"}, counts, counts + "\n", false},
		{[]string{"ps"}, noCounts, noCountsPlain, true},
		{[]string{"ps"}, noCounts, noCounts + "\n", false},
	} {
		var out, stderr bytes.Buffer
		status := run(tc.args, &out, &stderr, func(Command) (Result, error) {
			return Result{Output: tc.tsv}, nil
		}, nil, tc.interactive)
		if status != 0 || out.String() != tc.want || stderr.Len() != 0 {
			t.Errorf("%q interactive=%v: status=%d output=%q stderr=%q, want %q", tc.args, tc.interactive, status, out.String(), stderr.String(), tc.want)
		}
	}
}

func TestParseDomainsAndAliases(t *testing.T) {
	c, err := parse([]string{"md", "example.com", "Work", "-D", "*.other.test", "-D", "api.example.com", "-p", "Personal", "-w", "w1", "-n"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "move-domain" || c.Target != "Work" || c.Profile != "Personal" || c.Window != "w1" || !c.DryRun || len(c.Domains) != 3 {
		t.Fatalf("unexpected command: %+v", c)
	}
	for _, bad := range []string{"", "https://example.com", "example.com/path", "example.com:443", "example.*", "*example.com", "*.example.com ", "-example.com", "example..com"} {
		if _, err := parse([]string{"tabs", "-D", bad}); err == nil {
			t.Errorf("accepted domain %q", bad)
		}
	}
	for _, args := range [][]string{{"tabs", "-sl"}, {"find", ""}, {"open", "01"}, {"move-domain", "example.com", ""}, {"count", "-n"}, {"profiles", "-D", "example.com"}} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	if _, err := parse([]string{"ps", "-c", "-D", "*.example.com", "-p", "Work", "-w", "w2"}); err != nil {
		t.Fatal(err)
	}
}

func TestMoveDomainMissingTargetDiagnostic(t *testing.T) {
	for _, args := range [][]string{
		{"mv", "skills.sh", "-p", "GitHub"},
		{"move-domain", "skills.sh", "--profile", "Personal"},
		{"md", "skills.sh", "-D", "*.skills.sh"},
		{"mv", "skills.sh"},
		{"mv", "skills.sh", "-p"},
	} {
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command) (Result, error) {
			t.Fatal("incomplete move invoked Dia")
			return Result{}, nil
		}, nil, false)
		if status != 2 || !strings.Contains(stderr.String(), "dia mv skills.sh GitHub") || !strings.Contains(stderr.String(), "-p") || !strings.Contains(stderr.String(), "source profile") {
			t.Errorf("%q: status=%d stderr=%q", args, status, stderr.String())
		}
	}
	for _, args := range [][]string{{"exec", "1", "-h"}, {"move-domain", "example.com", "-h"}} {
		c, err := parse(args)
		if err != nil || len(c.Args) != 2 || c.Args[1] != "-h" {
			t.Errorf("positional -h changed for %q: %+v %v", args, c, err)
		}
	}
}

func TestHelpIsLocal(t *testing.T) {
	c, err := parse([]string{"help"})
	if err != nil || c.Name != "help" || !strings.Contains(help, "dia <command>") {
		t.Fatalf("help unavailable: %+v %v", c, err)
	}
}

func TestScopedHelpIsLocal(t *testing.T) {
	for _, args := range [][]string{{"help", "tabs"}, {"h", "t"}, {"tabs", "-h"}, {"t", "--help"}, {"move-domain", "-h"}, {"mv", "-h"}, {"md", "--help"}, {"exec", "-h"}} {
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command) (Result, error) {
			t.Fatal("help invoked Dia")
			return Result{}, nil
		}, nil, false)
		if status != 0 || !strings.Contains(out.String(), "USAGE") || stderr.Len() != 0 {
			t.Errorf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
		}
	}
}

func TestVersionIsLocal(t *testing.T) {
	for _, args := range [][]string{{"--version"}, {"version"}} {
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command) (Result, error) {
			t.Fatal("version invoked Dia")
			return Result{}, nil
		}, nil, false)
		if status != 0 || out.String() != "dia dev\n" || stderr.Len() != 0 {
			t.Fatalf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"version", "extra"}, {"--version", "extra"}} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestExistingCommandsAndValidation(t *testing.T) {
	for _, tc := range []struct {
		args []string
		name string
	}{
		{[]string{"t", "-d", "-sl", "--window", "w1"}, "tabs"},
		{[]string{"find", "-d", "-p", "Personal", "-D", "example.com"}, "find"},
		{[]string{"o", "2"}, "open"},
		{[]string{"nav", "2", "HTTPS://example.com/?q=quoted"}, "navigate"},
		{[]string{"ps", "-c", "-D", "example.com"}, "profiles"},
	} {
		c, err := parse(tc.args)
		if err != nil || c.Name != tc.name {
			t.Errorf("%q: %+v %v", tc.args, c, err)
		}
	}
	for _, args := range [][]string{
		{"bad"}, {"tabs", "extra"}, {"count", "--window"}, {"open", "0"}, {"open", "999999999999999999999"},
		{"move", "2", "Work", "-w", "w1"}, {"navigate", "2", "javascript:alert(1)"}, {"navigate", "2", "https://example.com/%ZZ"},
		{"exec", "2", "   "}, {"layout", "grid"}, {"profiles", "-n"}, {"current", "-D", "example.com"},
	} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
}

func TestCacheContainsOnlyIDsAndIsolatesSessions(t *testing.T) {
	dir := t.TempDir()
	a := &rowCache{path: filepath.Join(dir, "one")}
	b := &rowCache{path: filepath.Join(dir, "two")}
	if cacheKey("tty1", 2, "first") == cacheKey("tty1", 2, "second") || cacheKey("tty1", 2, "first") == cacheKey("tty2", 2, "first") || cacheKey("tty1", 2, "first") == cacheKey("tty1", 3, "first") {
		t.Fatal("cache identity collision")
	}
	if err := a.save([]TabRef{{Window: "w1", Tab: "a"}, {Window: "w2", Tab: "b"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.lookup("1"); err == nil {
		t.Fatal("other session read cache")
	}
	if got, err := a.lookup("2"); err != nil || got.Tab != "b" || got.Window != "w2" {
		t.Fatalf("wrong cached row: %+v %v", got, err)
	}
	data, err := os.ReadFile(a.path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "https://") || strings.Contains(string(data), "TITLE") {
		t.Fatal("cache contains display data")
	}
	info, err := os.Stat(a.path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("cache permissions: %v %v", info, err)
	}
	if err := a.save(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.lookup("1"); err == nil {
		t.Fatal("empty list retained old rows")
	}
}

func TestCacheIdentityIgnoresRedirectedStdin(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "ps"), []byte("#!/bin/sh\nprintf 'ttys999 Mon Oct 19 01:01:01 2026\\n'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	t.Setenv("TMPDIR", dir)
	first := newRowCache()
	stdin := os.Stdin
	file, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = stdin; file.Close() })
	second := newRowCache()
	if first.err != nil || second.err != nil || first.path == "" || first.path != second.path {
		t.Fatalf("cache identity changed with stdin: %+v %+v", first, second)
	}
}

func TestVisibleListReplacesCacheOnlyAfterSuccess(t *testing.T) {
	cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
	if err := cache.save([]TabRef{{Window: "old", Tab: "old"}}); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	failed := func(Command) (Result, error) { return Result{}, errors.New("read failed") }
	if status := run([]string{"tabs"}, &out, &stderr, failed, cache, true); status != 1 {
		t.Fatal(status)
	}
	if got, _ := cache.lookup("1"); got.Window != "old" {
		t.Fatal("failed list overwrote cache")
	}
	out.Reset()
	stderr.Reset()
	called := 0
	success := func(c Command) (Result, error) {
		called++
		return Result{Output: "WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw1\tP\ta\tTitle\thttps://example.com", Tabs: []TabRef{{Window: "w1", Tab: "a"}}}, nil
	}
	if status := run([]string{"t"}, &out, &stderr, success, cache, false); status != 0 {
		t.Fatal(stderr.String())
	}
	if got, _ := cache.lookup("1"); got.Window != "old" {
		t.Fatal("piped list overwrote cache")
	}
	out.Reset()
	stderr.Reset()
	if status := run([]string{"t"}, &out, &stderr, success, cache, true); status != 0 {
		t.Fatal(stderr.String())
	}
	if got, _ := cache.lookup("1"); got.Tab != "a" {
		t.Fatal("visible list did not update cache")
	}
	out.Reset()
	stderr.Reset()
	if status := run([]string{"open", "1"}, &out, &stderr, func(c Command) (Result, error) {
		if c.Name != "_open" || len(c.Args) != 2 || c.Args[0] != "w1" || c.Args[1] != "a" {
			t.Fatalf("action did not use saved IDs: %+v", c)
		}
		return Result{Output: "Focused a"}, nil
	}, cache, false); status != 0 {
		t.Fatal(stderr.String())
	}
	if called != 2 {
		t.Fatal("unexpected list calls")
	}
}

func TestEmbeddedBackendProtocol(t *testing.T) {
	dir := t.TempDir()
	fake := filepath.Join(dir, "osascript")
	script := "#!/bin/sh\n[ \"$1\" = -l ] && [ \"$2\" = JavaScript ] && [ \"$3\" = -e ] || exit 41\ncase \"$4\" in *'function run(argv)'*) ;; *) exit 42;; esac\ncase \"$5\" in *'\"name\":\"count\"'*) printf '{\"output\":\"7\"}';; *) exit 43;; esac\n"
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	result, err := backend(Command{Name: "count"})
	if err != nil || result.Output != "7" {
		t.Fatalf("embedded JXA request failed: %+v %v", result, err)
	}
}

func TestBackendReportsMissingOsascript(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := backend(Command{Name: "count"})
	if err == nil || !strings.Contains(err.Error(), "osascript") {
		t.Fatalf("missing executable was hidden: %v", err)
	}
}

func TestGoRendererKeepsCurrentAndSafeLinks(t *testing.T) {
	current := renderTabs("WINDOW\tTAB\tTITLE\tURL\nw1\ta\t中文标题\thttps://example.com/path", false)
	if !strings.Contains(current, "中文标题") || !strings.Contains(current, "\x1b]8;;https://example.com/path\x1b\\") {
		t.Fatalf("current row missing: %q", current)
	}
	unsafe := renderTabs("WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw1\tP\ta\tTitle\tjavascript:alert(1)", true)
	if strings.Contains(unsafe, "\x1b]8;;javascript:") {
		t.Fatal("unsafe OSC link")
	}
	if width("中文") != 4 || width("e\u0301") != 1 || !strings.Contains(cell("中文中文", 5), "…") {
		t.Fatal("Unicode display width or clipping")
	}
	if got := cell("%F{red}$(echo bad)", 24); !strings.HasPrefix(got, "%F{red}$(echo bad)") {
		t.Fatal("cell interpreted shell syntax")
	}
	for _, columns := range []string{"20", "40", "120"} {
		t.Setenv("COLUMNS", columns)
		rendered := renderTabs("WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw1\tP\ta\t中文标题\thttps://example.com/long/path\nw1\tP\tb\tASCII title with a long suffix\thttps://example.org", true)
		plain := stripTerminal(rendered)
		if !strings.Contains(plain, "https://example.com/long/path") || !strings.Contains(plain, "https://example.org") || (columns != "120" && !strings.Contains(plain, "…")) {
			t.Fatalf("width %s lost URL or clipping: %q", columns, plain)
		}
	}
	t.Setenv("COLUMNS", "40")
	compact := renderTabs("WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw1\tP\ta\t中文标题\thttps://example.com/long/path", false)
	if strings.Contains(stripTerminal(compact), "https://") || !strings.Contains(compact, "\x1b]8;;https://example.com/long/path\x1b\\") {
		t.Fatalf("compact visible URL or hyperlink changed: %q", compact)
	}
	unsafeURL := renderTabs("WINDOW\tTAB\tTITLE\tURL\nw\ta\tTest\thttps://example.org/\x1b]8;;evil", false)
	if strings.Contains(unsafeURL, "\x1b]8;;https://") {
		t.Fatal("control character URL became hyperlink")
	}
}
