package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalSearchParse(t *testing.T) {
	for _, name := range []string{"history", "hs", "bookmarks", "bm"} {
		for _, args := range [][]string{{name, "中文 phrase", "--json"}, {name, "--json", "中文 phrase"}, {name, "--json", "--", "-literal"}} {
			c, err := parse(args)
			if err != nil || !c.JSON || len(c.Args) != 1 {
				t.Fatalf("%v: %+v %v", args, c, err)
			}
		}
		for _, args := range [][]string{{name}, {name, " "}, {name, "word", "extra"}, {name, "word", "-p", "Day"}, {name, "word", "--window", "1"}, {name, "-unknown"}} {
			if _, err := parse(args); err == nil {
				t.Errorf("accepted %v", args)
			}
		}
	}
	if _, err := parse([]string{"tabs", "--json"}); err == nil {
		t.Fatal("accepted --json on tabs")
	}
}
func localFixture(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "Library/Application Support/Dia/User Data")
	if err := os.MkdirAll(filepath.Join(root, "Default"), 0700); err != nil {
		t.Fatal(err)
	}
	return root
}
func TestBookmarkSearch(t *testing.T) {
	root := localFixture(t)
	data := `{"roots":{"other":{"name":"Other","children":[{"type":"url","id":"3","name":"中文 Match","url":"https://same"}]},"bookmark_bar":{"name":"Bar","children":[{"type":"folder","name":"Nested","children":[{"type":"url","id":"1","name":"中文 match\t\u001b","url":"https://same"},{"type":"url","id":"2","name":"literal '%_","url":"https://example"}]}]}}}`
	if err := os.WriteFile(filepath.Join(root, "Default/Bookmarks"), []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	r, err := localSearch(Command{Name: "bookmarks", Args: []string{"中文 MATCH"}})
	if err != nil || len(r.Bookmarks) != 2 || r.Bookmarks[0].Folder != "Bar / Nested" || r.Bookmarks[0].ID != "1" {
		t.Fatalf("%+v %v", r, err)
	}
	if strings.ContainsAny(r.Output, "\x1b") {
		t.Fatal("terminal escape leaked")
	}
	r, err = localSearch(Command{Name: "bookmarks", Args: []string{"'%_"}})
	if err != nil || len(r.Bookmarks) != 1 {
		t.Fatalf("literal search: %+v %v", r, err)
	}
	for _, tty := range []bool{false, true} {
		var out, errs bytes.Buffer
		status := run([]string{"bm", "absent", "--json"}, &out, &errs, backend, &rowCache{err: os.ErrPermission}, outputMode{stdoutTTY: tty, stderrTTY: tty})
		if status != 0 || out.String() != "[]\n" || errs.Len() != 0 {
			t.Fatalf("JSON: %d %q %q", status, out.String(), errs.String())
		}
	}
}
func TestLocalSourceValidation(t *testing.T) {
	root := localFixture(t)
	for _, last := range []string{"../outside", ".", "a/b"} {
		os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"`+last+`"}}`), 0600)
		if _, err := localSearch(Command{Name: "bookmarks", Args: []string{"x"}}); err == nil {
			t.Fatalf("accepted %q", last)
		}
	}
	os.WriteFile(filepath.Join(root, "Local State"), []byte(`broken`), 0600)
	if _, err := localSearch(Command{Name: "bookmarks", Args: []string{"x"}}); err == nil {
		t.Fatal("accepted corrupt state")
	}
	os.Remove(filepath.Join(root, "Local State"))
	if _, err := localSearch(Command{Name: "history", Args: []string{"x"}}); err == nil {
		t.Fatal("missing history accepted")
	}
	if _, err := os.Stat(filepath.Join(root, "Default/History")); !os.IsNotExist(err) {
		t.Fatal("created history")
	}
	os.WriteFile(filepath.Join(root, "Default/Bookmarks"), []byte(`broken`), 0600)
	if _, err := localSearch(Command{Name: "bookmarks", Args: []string{"x"}}); err == nil {
		t.Fatal("accepted corrupt bookmarks")
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "Bookmarks"), []byte(`{"roots":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	os.Symlink(outside, filepath.Join(root, "Escape"))
	os.WriteFile(filepath.Join(root, "Local State"), []byte(`{"profile":{"last_used":"Escape"}}`), 0600)
	if _, err := localSearch(Command{Name: "bookmarks", Args: []string{"x"}}); err == nil {
		t.Fatal("accepted symlink escape")
	}
}

func TestHistoryLimitOrderAndWAL(t *testing.T) {
	root := localFixture(t)
	path := filepath.Join(root, "Default/History")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "/usr/bin/sqlite3", "-batch", "-init", "/dev/null", path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stdin.Close()
		if err := cmd.Wait(); err != nil {
			t.Errorf("fixture SQLite: %v: %s", err, diagnostics.String())
		}
	})
	sql := `PRAGMA journal_mode=WAL;
PRAGMA wal_autocheckpoint=0;
CREATE TABLE urls(id INTEGER PRIMARY KEY,title TEXT,url TEXT,last_visit_time INTEGER);
WITH RECURSIVE ids(id) AS (SELECT 1 UNION ALL SELECT id+1 FROM ids WHERE id<105)
INSERT INTO urls SELECT id,'match','https://example.test/' || id,11644473600000000 FROM ids;
INSERT INTO urls VALUES(106,'unrelated','https://other.test/',11644473601000000);
.print ready
`
	if _, err := stdin.Write([]byte(sql)); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	ready := false
	for scanner.Scan() {
		if scanner.Text() == "ready" {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("WAL fixture not ready: %v", scanner.Err())
	}
	before := map[string][]byte{}
	for _, file := range []string{path, path + "-wal"} {
		data, err := os.ReadFile(file)
		if err != nil || len(data) == 0 {
			t.Fatalf("WAL fixture file %s: %v", file, err)
		}
		before[file] = data
	}
	r, err := localSearch(Command{Name: "history", Args: []string{"match"}})
	if err != nil || len(r.History) != 100 {
		t.Fatalf("WAL search: %d results, %v", len(r.History), err)
	}
	for i, item := range r.History {
		if item.ID != int64(105-i) {
			t.Fatalf("tie ordering at %d: got %d", i, item.ID)
		}
	}
	for file, expected := range before {
		actual, err := os.ReadFile(file)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("history search modified %s: %v", file, err)
		}
	}
}
func TestHistorySearch(t *testing.T) {
	root := localFixture(t)
	path := filepath.Join(root, "Default/History")
	sql := `CREATE TABLE urls(id INTEGER PRIMARY KEY,title TEXT,url TEXT,last_visit_time INTEGER);
INSERT INTO urls VALUES(1,'中文 MATCH','https://one',11644473600000000),(2,NULL,'https://中文-match',11644473601000000),(3,'literal ''%_','https://three',11644473601000000);`
	if out, err := exec.Command("/usr/bin/sqlite3", path, sql).CombinedOutput(); err != nil {
		t.Fatalf("fixture %s %v", out, err)
	}
	r, err := localSearch(Command{Name: "history", Args: []string{"中文"}})
	if err != nil || len(r.History) != 2 || r.History[0].ID != 2 || r.History[0].LastVisitedAt != "1970-01-01T00:00:01Z" {
		t.Fatalf("%+v %v", r, err)
	}
	r, err = localSearch(Command{Name: "history", Args: []string{"'%_"}})
	if err != nil || len(r.History) != 1 || r.History[0].ID != 3 {
		t.Fatalf("%+v %v", r, err)
	}
	os.WriteFile(path, []byte("broken"), 0600)
	if _, err := localSearch(Command{Name: "history", Args: []string{"x"}}); err == nil {
		t.Fatal("accepted corrupt database")
	}
}
