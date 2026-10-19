package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fakeBackend(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "osascript"), []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func TestBackendProgressArrivesBeforeExit(t *testing.T) {
	release := filepath.Join(t.TempDir(), "release")
	t.Setenv("DIA_TEST_RELEASE", release)
	fakeBackend(t, `printf '%s\n' '{"progress":{"phase":"scan","completed":0,"total":0}}'
# Wait for the Go callback, not a timer, before returning the result.
i=0
while [ ! -f "$DIA_TEST_RELEASE" ]; do
 i=$((i+1))
 [ "$i" -lt 300 ] || exit 44
 sleep .01
done
printf '%s\n' '{"output":"done","move":{"matched":0,"moved":0,"skipped":0,"failed":0,"remaining":0}}'
`)
	observed := false
	result, err := backend(Command{Name: "move-domain", Progress: true}, func(p Progress) {
		observed = true
		if p.Phase != "scan" {
			t.Errorf("unexpected phase: %+v", p)
		}
		if err := os.WriteFile(release, []byte("observed"), 0600); err != nil {
			t.Error(err)
		}
	})
	if err != nil || !observed || result.Output != "done" {
		t.Fatalf("stream: %+v %v observed=%v", result, err, observed)
	}
}

func TestBackendLargeResultAndDiagnostics(t *testing.T) {
	payload := strings.Repeat("x", 128*1024)
	raw, _ := json.Marshal(Result{Output: payload})
	fakeBackend(t, "cat <<'RESULT'\n"+string(raw)+"\nRESULT\n")
	result, err := backend(Command{Name: "count"}, nil)
	if err != nil || result.Output != payload {
		t.Fatalf("large response: %v len=%d", err, len(result.Output))
	}
	fakeBackend(t, "printf '%s\\n' '{\"output\":\"partial\"}'\nprintf '%s\\n' 'original Apple Event diagnostic' >&2\nexit 7\n")
	result, err = backend(Command{Name: "count"}, nil)
	if err == nil || !strings.Contains(err.Error(), "original Apple Event diagnostic") || result.Output != "" {
		t.Fatalf("diagnostic/result: %+v %v", result, err)
	}
}

func TestBackendRejectsBrokenProtocolAndReaps(t *testing.T) {
	for _, tc := range []struct {
		name, frames string
		progress     bool
	}{
		{"empty", "", true},
		{"progress only", `{"progress":{"phase":"scan","completed":0,"total":0}}`, true},
		{"duplicate result", `{"output":"a"} {"output":"b"}`, true},
		{"progress after final", `{"output":"a"} {"progress":{"phase":"scan"}}`, true},
		{"unknown phase", `{"progress":{"phase":"evil"}}`, true},
		{"negative", `{"progress":{"phase":"scan","completed":-1}}`, true},
		{"over total", `{"progress":{"phase":"moved","completed":2,"total":1}}`, true},
		{"control", `{"progress":{"phase":"scan","window":"\u001b[2J"}}`, true},
		{"mixed", `{"progress":{"phase":"scan"},"output":"bad"}`, true},
		{"unexpected progress", `{"progress":{"phase":"scan"}}`, false},
		{"no output", `{"move":{"matched":1}}`, true},
		{"null", "null", true},
		{"malformed", "{broken", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fakeBackend(t, "cat <<'FRAMES'\n"+tc.frames+"\nFRAMES\n")
			if result, err := backend(Command{Name: "move-domain", Progress: tc.progress}, nil); err == nil || result.Output != "" {
				t.Fatalf("accepted: %+v %v", result, err)
			}
		})
	}
	fakeBackend(t, "printf 'broken\\n'\nwhile :; do printf 'more invalid data so stdout cannot finish\\n'; done\n")
	done := make(chan error, 1)
	go func() { _, err := backend(Command{Name: "count"}, nil); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("malformed stream accepted")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("decoder failed to stop and reap a child writing stdout")
	}
}

func TestExactDomainScopeAndOrder(t *testing.T) {
	for _, args := range [][]string{
		{"mv", "example.com", "Work", "-E", "-D", "other.test", "-D", "*.example.com"},
		{"mv", "example.com", "Work", "-D", "other.test", "-D", "*.example.com", "--exact-domain"},
	} {
		c, err := parse(args)
		if err != nil || len(c.Domains) != 3 || !c.Domains[0].Exact || !c.Domains[1].Exact || c.Domains[2].Exact || !c.Domains[2].Subdomains {
			t.Fatalf("%q: %+v %v", args, c, err)
		}
	}
	for _, name := range []string{"tabs", "links", "count", "find", "profiles"} {
		args := []string{name}
		if name == "find" {
			args = append(args, "query")
		}
		if name == "profiles" {
			args = append(args, "-c")
		}
		c, err := parse(append(args, "-E", "-D", "example.com"))
		if err != nil || !c.Domains[0].Exact {
			t.Fatalf("%q: %+v %v", args, c, err)
		}
	}
	for _, args := range [][]string{{"tabs", "-E"}, {"current", "-E"}, {"profiles", "-D", "example.com", "-E"}, {"mv", "example.com", "-E"}, {"mv", "example.com", "--exact-domain"}} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	c, err := parse([]string{"mv", "example.com", "Work"})
	if err != nil || c.Domains[0].Exact || c.Domains[0].Subdomains {
		t.Fatalf("default: %+v %v", c, err)
	}
}

func TestProgressUsesStderrIndependentlyOfStdout(t *testing.T) {
	for _, mode := range []outputMode{{}, {stdoutTTY: true}, {stderrTTY: true}, {stdoutTTY: true, stderrTTY: true}} {
		var out, errs bytes.Buffer
		status := run([]string{"mv", "example.com", "Work", "-n"}, &out, &errs, func(c Command, notify func(Progress)) (Result, error) {
			if c.Progress != mode.stderrTTY || (notify != nil) != mode.stderrTTY {
				t.Errorf("mode=%+v progress=%v", mode, c.Progress)
			}
			if notify != nil {
				notify(Progress{Phase: "scan"})
				notify(Progress{Phase: "done"})
			}
			return Result{Move: &MoveResult{}}, nil
		}, nil, mode)
		if status != 0 || strings.Contains(out.String(), "Scanning") || strings.Contains(errs.String(), "Scanning") != mode.stderrTTY {
			t.Fatalf("mode=%+v out=%q err=%q", mode, out.String(), errs.String())
		}
		if mode.stdoutTTY && (!strings.Contains(out.String(), "Preview (no tabs moved)") || !strings.Contains(out.String(), "example.com → Work") || !strings.Contains(out.String(), "No matching tabs.")) {
			t.Fatal(out.String())
		}
		if !mode.stdoutTTY && out.String() != "dry-run matched=0 moved=0 skipped=0 failed=0 remaining=0\n" {
			t.Fatal(out.String())
		}
	}
}

func TestWindowsAndPreviewUseTextColumns(t *testing.T) {
	for _, tc := range []struct {
		args   []string
		result Result
		want   string
	}{
		{[]string{"windows"}, Result{Output: "WINDOW\tPROFILE\tLAYOUT\nw1\tA\tsidebar\nw2\tLong\ttabStrip"}, "w1      A        sidebar\n"},
		{[]string{"mv", "example.com", "Work", "-n"}, Result{Output: "ACTION\tWINDOW\tTAB\tSOURCE\tTARGET\tURL\nMOVE\tw1\ta\tP\tWork\thttps://example.com", Move: &MoveResult{Matched: 1, Remaining: 1}}, "MOVE    a    P       Work    https://example.com\n"},
	} {
		var out, errs bytes.Buffer
		status := run(tc.args, &out, &errs, func(Command, func(Progress)) (Result, error) { return tc.result, nil }, nil, outputMode{stdoutTTY: true})
		if status != 0 || !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "\t") {
			t.Fatalf("%q: %q %q", tc.args, out.String(), errs.String())
		}
	}
}

func TestProgressContextSanitizedWithoutRejectingProfile(t *testing.T) {
	event := Progress{Phase: "profile", Profile: "Team 👩\u200d💻\u202e"}
	if err := event.validate(); err != nil {
		t.Fatal(err)
	}
	shown := renderProgress(event, false)
	if strings.ContainsAny(shown, "\u200d\u202e") || !strings.Contains(shown, "Team") {
		t.Fatalf("unsafe or lost profile: %q", shown)
	}
}

func TestCompletionRequiresSuccessfulFinalMoveResult(t *testing.T) {
	for _, tc := range []struct {
		name, tail string
		wantStatus int
	}{
		{"missing final", "", 1},
		{"malformed final", "printf 'broken'", 1},
		{"missing move", `printf '%s\n' '{"output":""}'`, 1},
		{"null move", `printf '%s\n' '{"output":"","move":null}'`, 1},
		{"failed exit", `printf '%s\n' '{"output":"","move":{}}'; exit 7`, 1},
		{"move error", `printf '%s\n' '{"output":"","move":{"failed":1},"error":"Move failed"}'`, 1},
		{"valid result", `printf '%s\n' '{"output":"","move":{}}'`, 0},
	} {
		for _, preview := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/preview=%v", tc.name, preview), func(t *testing.T) {
				fakeBackend(t, `printf '%s\n' '{"progress":{"phase":"scan","completed":0,"total":0}}' '{"progress":{"phase":"done","completed":0,"total":0}}'`+"\n"+tc.tail+"\n")
				args := []string{"mv", "example.com", "Work"}
				if preview {
					args = append(args, "-n")
				}
				var out, errs bytes.Buffer
				status := run(args, &out, &errs, backend, nil, outputMode{stderrTTY: true})
				if status != tc.wantStatus || !strings.Contains(errs.String(), "Scanning tabs") {
					t.Fatalf("status=%d stdout=%q stderr=%q", status, out.String(), errs.String())
				}
				complete := strings.Contains(errs.String(), "Move complete") || strings.Contains(errs.String(), "Preview complete")
				if complete != (tc.wantStatus == 0) {
					t.Fatalf("premature/missing completion: %q", errs.String())
				}
				if tc.wantStatus == 0 && strings.Contains(errs.String(), "Preview complete") != preview {
					t.Fatalf("wrong completion: %q", errs.String())
				}
			})
		}
	}
}

func TestProgressHidesWindowContext(t *testing.T) {
	for _, phase := range []string{"scan", "profile", "preflight", "plan", "moving", "moved", "failure", "done"} {
		shown := renderProgress(Progress{Phase: phase, Window: "private-window-id", Profile: "Work", Completed: 1, Total: 2}, false)
		if strings.Contains(shown, "window") || !strings.Contains(shown, "Work") || !strings.Contains(shown, "1/2") {
			t.Fatalf("%s: %q", phase, shown)
		}
	}
}
