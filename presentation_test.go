package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobalQueryFrame(t *testing.T) {
	var out, errs bytes.Buffer
	status := run([]string{"ps", "-c"}, &out, &errs, func(Command, func(Progress)) (Result, error) {
		return Result{Output: "WINDOW\tINDEX\tPROFILE\tCOUNT\nw-secret\t1\tDay\t3"}, nil
	}, nil, outputMode{stdoutTTY: true})
	if status != 0 || !strings.HasPrefix(out.String(), "┌  Profiles\n") || !strings.Contains(out.String(), "└  1 profile") || strings.Contains(out.String(), "w-secret") {
		t.Fatalf("%d %q %q", status, out.String(), errs.String())
	}
}

func TestGlobalCommandFamilies(t *testing.T) {
	for _, tc := range []struct {
		args                  []string
		output, title, footer string
	}{
		{[]string{"t"}, "WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw-secret\tDay\ta\tTitle\thttps://example.com", "Tabs", "1 tab"},
		{[]string{"f", "query", "-p", "Day", "-D", "example.com"}, "WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw-secret\tDay\ta\tTitle\thttps://example.com", "Search tabs", "1 tab"},
		{[]string{"c"}, "WINDOW\tTAB\tTITLE\tURL\nw-secret\ta\tTitle\thttps://example.com", "Current tab", "1 tab"},
		{[]string{"ps"}, "WINDOW\tINDEX\tPROFILE\nw-secret\t1\tDay", "Profiles", "1 profile"},
		{[]string{"w"}, "WINDOW\tPROFILE\tLAYOUT\nw-secret\tDay\tsidebar", "Windows", "1 window"},
	} {
		t.Run(tc.args[0], func(t *testing.T) {
			var out, errs bytes.Buffer
			cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
			status := run(tc.args, &out, &errs, func(Command, func(Progress)) (Result, error) {
				return Result{Output: tc.output, Tabs: []TabRef{{Window: "w-secret", Tab: "a"}}}, nil
			}, cache, outputMode{stdoutTTY: true})
			if status != 0 || !strings.HasPrefix(out.String(), "┌  "+tc.title) || !strings.Contains(out.String(), "└  "+tc.footer) {
				t.Fatalf("%d %q %q", status, out.String(), errs.String())
			}
			if tc.args[0] != "w" && strings.Contains(out.String(), "w-secret") {
				t.Fatal(out.String())
			}
			if tc.args[0] == "w" && !strings.Contains(out.String(), "w-secret") {
				t.Fatal(out.String())
			}
			if tc.args[0] == "f" && (!strings.Contains(out.String(), "Search  query") || !strings.Contains(out.String(), "Profile Day") || !strings.Contains(out.String(), "example.com")) {
				t.Fatal(out.String())
			}
			if tc.args[0] == "t" && !strings.Contains(out.String(), "\x1b]8;;https://example.com") {
				t.Fatal("clickable title lost")
			}
		})
	}
	for _, args := range [][]string{{"o", "1"}, {"cl", "1"}, {"m", "1", "Work"}, {"nav", "1", "https://example.com"}, {"fo", "a"}, {"p", "Work"}, {"la", "sidebar"}} {
		var out, errs bytes.Buffer
		cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
		if err := cache.save([]TabRef{{Window: "w-secret", Tab: "a"}}); err != nil {
			t.Fatal(err)
		}
		status := run(args, &out, &errs, func(Command, func(Progress)) (Result, error) { return Result{Output: "Action complete"}, nil }, cache, outputMode{stdoutTTY: true})
		if status != 0 || out.String() != "◇  Action complete\n" || errs.Len() != 0 {
			t.Fatalf("%q %d %q %q", args, status, out.String(), errs.String())
		}
	}
	for _, args := range [][]string{{"l"}, {"n"}, {"e", "1", "document.title"}} {
		var out, errs bytes.Buffer
		cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
		if err := cache.save([]TabRef{{Window: "w", Tab: "a"}}); err != nil {
			t.Fatal(err)
		}
		status := run(args, &out, &errs, func(Command, func(Progress)) (Result, error) { return Result{Output: "untouched\nraw"}, nil }, cache, outputMode{stdoutTTY: true, stderrTTY: true})
		if status != 0 || out.String() != "untouched\nraw\n" || errs.Len() != 0 {
			t.Fatalf("%q %q %q", args, out.String(), errs.String())
		}
	}
}

func TestEmptyQueriesHaveNoTable(t *testing.T) {
	for _, args := range [][]string{{"t"}, {"f", "x"}, {"c"}, {"ps"}, {"w"}} {
		var out, errs bytes.Buffer
		cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
		status := run(args, &out, &errs, func(Command, func(Progress)) (Result, error) { return Result{Output: "WINDOW\tTAB\tTITLE\tURL"}, nil }, cache, outputMode{stdoutTTY: true})
		if status != 0 || !strings.Contains(out.String(), "└  No ") || strings.Contains(out.String(), "TITLE") || strings.Contains(out.String(), "WINDOW") {
			t.Fatalf("%q %q", args, out.String())
		}
	}
}

func TestMoveStagesCoalesceInterleavedWindowsAndClear(t *testing.T) {
	var stream bytes.Buffer
	status := run([]string{"mv", "example.com", "Work"}, &stream, &stream, func(c Command, notify func(Progress)) (Result, error) {
		if !strings.Contains(stream.String(), "┌  Move tabs") || !c.Progress {
			t.Fatal("header did not precede backend")
		}
		for _, window := range []string{"window-one", "window-two"} {
			for _, phase := range []string{"scan", "profile", "preflight"} {
				notify(Progress{Phase: phase, Window: window, Profile: "secret-profile", Completed: 1, Total: 1})
			}
			if strings.Contains(stream.String(), "Target ready") {
				t.Fatal("preflight does not prove global readiness")
			}
		}
		notify(Progress{Phase: "plan", Completed: 3, Total: 3})
		notify(Progress{Phase: "plan", Completed: 3, Total: 3})
		for _, phase := range []string{"moving", "moved", "moving", "done"} {
			notify(Progress{Phase: phase, Completed: 1, Total: 2})
		}
		if strings.Contains(stream.String(), "└  Done") {
			t.Fatal("premature completion")
		}
		return Result{Move: &MoveResult{Matched: 3, Moved: 2, Skipped: 1}}, nil
	}, nil, outputMode{stdoutTTY: true, stderrTTY: true})
	shown := stream.String()
	if status != 0 || strings.Count(shown, "●  Reading tabs") != 1 || strings.Count(shown, "Target ready") != 1 || strings.Count(shown, "Found 3 matching tabs") != 1 || strings.Count(shown, "Moving tabs · 1/2") != 1 || !strings.Contains(shown, "\r\x1b[2K◇  Moved 2 tabs") || strings.Contains(shown, "window-") || strings.Contains(shown, "secret-profile") {
		t.Fatalf("%d %q", status, shown)
	}
}

func TestMoveFailureAndZeroMatchOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result Result
		err    error
		want   string
		code   int
	}{
		{"empty", Result{Move: &MoveResult{}}, nil, "No matching tabs · no changes made", 0},
		{"skipped", Result{Move: &MoveResult{Matched: 2, Skipped: 2}}, nil, "Skipped 2 tabs · already in Work", 0},
		{"partial", Result{Move: &MoveResult{Matched: 3, Moved: 1, Failed: 1, Remaining: 2}, Error: "Original window w-secret tab error"}, nil, "Incomplete · 1 tab moved", 1},
		{"protocol", Result{}, errors.New("Original protocol error"), "Failed", 1},
	} {
		for _, mode := range []outputMode{{}, {stdoutTTY: true}, {stderrTTY: true}, {stdoutTTY: true, stderrTTY: true}} {
			t.Run(tc.name, func(t *testing.T) {
				var stream, out, errs bytes.Buffer
				outWriter, errWriter := &out, &errs
				if mode.stdoutTTY && mode.stderrTTY {
					outWriter = &stream
					errWriter = &stream
				}
				code := run([]string{"mv", "example.com", "Work"}, outWriter, errWriter, func(_ Command, notify func(Progress)) (Result, error) {
					if notify != nil {
						notify(Progress{Phase: "scan"})
						notify(Progress{Phase: "plan"})
						notify(Progress{Phase: "done"})
					}
					return tc.result, tc.err
				}, nil, mode)
				shown := stream.String() + out.String() + errs.String()
				if code != tc.code {
					t.Fatalf("%d %q", code, shown)
				}
				if mode.stdoutTTY && !strings.Contains(shown, tc.want) {
					t.Fatalf("missing outcome %q: %q", tc.want, shown)
				}
				if tc.code != 0 && strings.Contains(shown, "└  Done") {
					t.Fatal(shown)
				}
				if tc.name == "empty" && (strings.Contains(shown, "Target ready") || strings.Contains(shown, "Found 0")) {
					t.Fatal(shown)
				}
				if tc.name == "partial" && mode.stdoutTTY {
					if !strings.Contains(shown, "RESULT") || !strings.Contains(shown, "remaining") || !strings.Contains(shown, tc.result.Error) {
						t.Fatal(shown)
					}
				}
				if tc.name == "protocol" && strings.Contains(shown, "matched") {
					t.Fatal(shown)
				}
				if tc.name == "skipped" && mode.stdoutTTY && strings.Contains(shown, "Moved 0") {
					t.Fatal(shown)
				}
				if tc.err != nil && mode.stderrTTY && !strings.Contains(shown, "\r\x1b[2K✕  Original protocol error") {
					t.Fatal(shown)
				}
			})
		}
	}
}

func TestGlobalErrorsRespectStderrMode(t *testing.T) {
	for _, args := range [][]string{{"unknown"}, {"o", "1"}} {
		for _, tty := range []bool{false, true} {
			var out, errs bytes.Buffer
			status := run(args, &out, &errs, func(Command, func(Progress)) (Result, error) { t.Fatal("backend must not run"); return Result{}, nil }, &rowCache{path: filepath.Join(t.TempDir(), "absent")}, outputMode{stderrTTY: tty})
			if status == 0 || strings.HasPrefix(errs.String(), "✕  ") != tty || out.Len() != 0 {
				t.Fatalf("%q %d %q %q", args, status, out.String(), errs.String())
			}
		}
	}
}

func TestPreviewHidesIdentifierColumnsOnly(t *testing.T) {
	var out, errs bytes.Buffer
	status := run([]string{"mv", "example.com", "Work", "-n"}, &out, &errs, func(Command, func(Progress)) (Result, error) {
		return Result{Output: "ACTION\tWINDOW\tTAB\tSOURCE\tTARGET\tURL\nMOVE\tprivate-window\tprivate-tab\tTAB\tWINDOW\thttps://example.com", Move: &MoveResult{Matched: 1, Remaining: 1}}, nil
	}, nil, outputMode{stdoutTTY: true})
	if status != 0 || strings.Contains(out.String(), "private-") || !strings.Contains(out.String(), "ACTION  SOURCE  TARGET  URL") || !strings.Contains(out.String(), "MOVE    TAB     WINDOW  https://example.com") {
		t.Fatalf("%d %q %q", status, out.String(), errs.String())
	}
}
