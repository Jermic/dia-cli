package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDedupeParsing(t *testing.T) {
	for _, args := range [][]string{{"dedupe"}, {"dd", "-n"}, {"dd", "-p", "Work", "-D", "example.com", "-E", "-k", "docs", "--pinned", "--loading", "-w", "w1"}} {
		c, err := parse(args)
		if err != nil || c.Name != "dedupe" || !c.DryRun {
			t.Fatalf("%v: %+v %v", args, c, err)
		}
	}
	c, err := parse([]string{"dd", "--apply"})
	if err != nil || c.DryRun {
		t.Fatalf("apply: %+v %v", c, err)
	}
	for _, args := range [][]string{{"dd", "-a", "-n"}, {"dd", "--dry-run", "--apply"}, {"tabs", "-a"}, {"dd", "-s"}, {"dd", "extra"}} {
		if _, err := parse(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestDedupeResultValidation(t *testing.T) {
	tests := []struct {
		tally   *DedupeResult
		preview bool
		reason  string
		valid   bool
	}{
		{nil, true, "", false},
		{&DedupeResult{}, true, "", true},
		{&DedupeResult{Groups: 1, Duplicates: 3, Skipped: 1, Remaining: 2}, true, "", true},
		{&DedupeResult{Groups: 2, Duplicates: 1, Skipped: 1}, false, "", true},
		{&DedupeResult{Groups: 1, Duplicates: 2, Closed: 2}, false, "", true},
		{&DedupeResult{Groups: 1, Duplicates: 2, Closed: 1, Failed: 1, Remaining: 1}, false, "close failed", true},
		{&DedupeResult{Groups: -1}, true, "", false},
		{&DedupeResult{Groups: 1, Duplicates: 1, Closed: 1}, true, "", false},
		{&DedupeResult{Groups: 1, Duplicates: 1, Remaining: 1}, false, "", false},
		{&DedupeResult{Groups: 1, Duplicates: 2, Closed: 1, Skipped: 1, Remaining: 1}, false, "failed", false},
		{&DedupeResult{Groups: 1, Duplicates: 2, Failed: 2, Remaining: 1}, false, "failed", false},
		{&DedupeResult{Groups: 1, Duplicates: 2, Skipped: 1, Remaining: 1, Failed: 1}, true, "", false},
	}
	for _, tc := range tests {
		if err := tc.tally.validate(tc.preview, tc.reason); (err == nil) != tc.valid {
			t.Errorf("%+v preview=%v reason=%q valid=%v error=%v", tc.tally, tc.preview, tc.reason, tc.valid, err)
		}
	}
}

func TestDedupeBackendProtocol(t *testing.T) {
	for _, response := range []string{
		`{"output":""}`,
		`{"output":"","dedupe":{"groups":1,"duplicates":1,"closed":1,"remaining":1}}`,
		`{"output":"","dedupe":{"groups":1,"duplicates":1,"closed":1}}`,
	} {
		fakeBackend(t, "cat <<'RESULT'\n"+response+"\nRESULT\n")
		if _, err := backend(Command{Name: "dedupe", DryRun: true}, nil); err == nil {
			t.Fatalf("accepted malformed preview %s", response)
		}
	}
	fakeBackend(t, `printf '%s\n' '{"progress":{"phase":"closing","completed":0,"total":1}}' '{"progress":{"phase":"closed","completed":1,"total":1}}' '{"output":"","dedupe":{"groups":1,"duplicates":1,"closed":1}}'`)
	var events []string
	result, err := backend(Command{Name: "dedupe", Progress: true}, func(p Progress) { events = append(events, p.Phase) })
	if err != nil || result.Dedupe.Closed != 1 || strings.Join(events, ",") != "closing,closed" {
		t.Fatalf("valid close protocol: %+v %v %v", result, events, err)
	}
}

func TestDedupePresentationAndCache(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rows")
	if err := os.WriteFile(path, []byte("saved rows remain unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	preview := Result{Output: "ACTION\tWINDOW\tTAB\tPROFILE\tURL\tREASON\nKEEP\tw-private\tid-private\tWork\thttps://example.com/\tfirst copy\nCLOSE\tw-private\tid-other\tWork\thttps://example.com/\tduplicate URL", Dedupe: &DedupeResult{Groups: 1, Duplicates: 1, Remaining: 1}}
	for _, tty := range []bool{true, false} {
		var out, errs bytes.Buffer
		status := run([]string{"dd"}, &out, &errs, func(c Command, onProgress func(Progress)) (Result, error) {
			if !c.DryRun {
				t.Fatal("default invoked apply")
			}
			if onProgress != nil {
				onProgress(Progress{Phase: "scan"})
				onProgress(Progress{Phase: "plan", Completed: 1, Total: 1})
			}
			return preview, nil
		}, &rowCache{path: path}, outputMode{stdoutTTY: tty, stderrTTY: tty})
		text := out.String()
		if status != 0 {
			t.Fatalf("preview failed %s", errs.String())
		}
		if tty {
			if strings.Contains(text, "WINDOW") || strings.Contains(text, "id-private") || strings.Contains(text, "w-private") || !strings.Contains(text, "1 tab would close · no changes made") {
				t.Fatalf("human preview %q", text)
			}
		} else if !strings.Contains(text, "KEEP\tw-private\tid-private") || !strings.Contains(text, "remaining\t1") {
			t.Fatalf("raw audit output %q", text)
		}
		saved, err := os.ReadFile(path)
		if err != nil || string(saved) != "saved rows remain unchanged" {
			t.Fatalf("dedupe replaced row cache %q %v", saved, err)
		}
	}
	var out, errs bytes.Buffer
	status := run([]string{"dd"}, &out, &errs, func(Command, func(Progress)) (Result, error) {
		return Result{Output: "ACTION\tWINDOW\tTAB\tPROFILE\tURL\tREASON", Dedupe: &DedupeResult{}}, nil
	}, nil, outputMode{stdoutTTY: true})
	if status != 0 || !strings.Contains(out.String(), "No duplicate tabs · no changes made") || strings.Contains(out.String(), "ACTION") || strings.Contains(out.String(), "METRIC") {
		t.Fatalf("empty output %q %q", out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	status = run([]string{"dd", "-a"}, &out, &errs, func(Command, func(Progress)) (Result, error) {
		return Result{Error: "original close failure", Dedupe: &DedupeResult{Groups: 1, Duplicates: 2, Closed: 1, Remaining: 1, Failed: 1}}, nil
	}, nil, outputMode{stdoutTTY: true})
	if status != 1 || !strings.Contains(out.String(), "Incomplete · 1 tab closed") || !strings.Contains(errs.String(), "original close failure") {
		t.Fatalf("partial result %q %q", out.String(), errs.String())
	}
	out.Reset()
	errs.Reset()
	status = run([]string{"dd", "-a"}, &out, &errs, func(Command, func(Progress)) (Result, error) { return Result{}, nil }, nil, outputMode{stdoutTTY: true})
	if status != 1 || !strings.Contains(errs.String(), "missing dedupe result") || strings.Contains(out.String(), "closed") {
		t.Fatalf("fabricated success %q %q", out.String(), errs.String())
	}
}
