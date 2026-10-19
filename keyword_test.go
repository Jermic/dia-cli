package main

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestKeywordSelectors(t *testing.T) {
	for _, args := range [][]string{{"mv", "youtube", "Watch"}, {"tabs", "-k", "YouTube", "--keyword", "/watch?v=", "-D", "skills.sh"}} {
		c, err := parse(args)
		want := []string{"youtube"}
		if args[0] == "tabs" {
			want = []string{"YouTube", "/watch?v="}
		}
		if err != nil || !reflect.DeepEqual(c.Keywords, want) {
			t.Errorf("%q: %+v %v", args, c, err)
		}
	}
	for _, args := range [][]string{{"mv", "bad..com", "Watch"}, {"mv", "bad/token", "Watch"}, {"mv", "---", "Watch"}, {"mv", "youtube", "Watch", "-E"}, {"tabs", "-D", "youtube"}, {"tabs", "-k", ""}, {"tabs", "-k", "a\nb"}, {"profiles", "-k", "youtube"}, {"current", "-k", "youtube"}, {"mv", "youtube", "-k", "other"}, {"mv", "youtube", "Watch", "-n", "--", "-D", "example.com"}, {"tabs", "-p", "Day", "--", "-D", "example.com"}} {
		if c, err := parse(args); err == nil {
			t.Errorf("accepted %q: %+v", args, c)
		}
	}
}
func TestHelpPartialAndLiteralBoundary(t *testing.T) {
	for _, args := range [][]string{{"mv", "skills.sh", "--help"}, {"exec", "1", "-h"}, {"exec", "1", "-w", "--help"}, {"tabs", "--help", "extra"}, {"find", "--help"}} {
		c, err := parse(args)
		if err != nil || c.Name != "help" {
			t.Errorf("%q: %+v %v", args, c, err)
		}
	}
	for _, args := range [][]string{{"mv", "example.com", "--", "--help"}, {"exec", "1", "--", "-h"}, {"find", "--", "--help"}, {"tabs", "-p", "--help"}, {"tabs", "-k", "--help"}} {
		c, err := parse(args)
		if err != nil || c.Name == "help" {
			t.Errorf("%q: %+v %v", args, c, err)
		}
	}
}

func TestKeywordsAcrossQueries(t *testing.T) {
	for _, name := range []string{"tabs", "links", "count", "find", "mv", "profiles"} {
		args := map[string][]string{"find": {"find", "title"}, "mv": {"mv", "skills.sh", "Watch"}, "profiles": {"profiles", "-c"}}[name]
		if args == nil {
			args = []string{name}
		}
		c, err := parse(append(args, "-k", "YouTube", "--keyword", "/watch?v=", "-D", "github.com", "-E"))
		if err != nil || len(c.Keywords) != 2 || len(c.Domains) == 0 || !c.Domains[0].Exact {
			t.Errorf("%s: %+v %v", name, c, err)
		}
	}
	for _, word := range []string{"视频_2", "you-tube", "2", "YouTube"} {
		c, err := parse([]string{"mv", word, "Watch"})
		if err != nil || len(c.Keywords) != 1 {
			t.Errorf("%s: %+v %v", word, c, err)
		}
	}
	for _, value := range []string{"document.title.includes('--help')", "-v"} {
		c, err := parse([]string{"exec", "1", value})
		if err != nil || c.Args[1] != value {
			t.Errorf("%q %+v %v", value, c, err)
		}
	}
}

func TestKeywordContextAndLocalHelp(t *testing.T) {
	for _, args := range [][]string{{"mv", "skills.sh", "--help"}, {"exec", "1", "--help"}, {"tabs", "--help", "extra"}} {
		var out, errs bytes.Buffer
		status := run(args, &out, &errs, func(Command, func(Progress)) (Result, error) { t.Fatal("help contacted Dia"); return Result{}, nil }, nil, outputMode{})
		if status != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "USAGE") {
			t.Fatalf("%q: %d %q %q", args, status, out.String(), errs.String())
		}
	}
	for _, tty := range []bool{false, true} {
		var out, errs bytes.Buffer
		status := run([]string{"mv", "youtube", "Watch", "-n"}, &out, &errs, func(c Command, _ func(Progress)) (Result, error) {
			if len(c.Keywords) != 1 || len(c.Domains) != 0 {
				t.Fatalf("%+v", c)
			}
			return Result{Move: &MoveResult{}}, nil
		}, nil, outputMode{stdoutTTY: tty})
		if status != 0 || tty != strings.Contains(out.String(), "Keyword youtube") || strings.Contains(out.String(), "Domain") {
			t.Fatalf("%v %q", tty, out.String())
		}
	}
}

func TestRowActionUsesRowOnlyInTerminal(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{{[]string{"o", "2"}, "Focused tab 2"}, {[]string{"cl", "2"}, "Closed tab 2"}, {[]string{"m", "2", "ReadLater"}, "Moved tab 2 to ReadLater"}, {[]string{"nav", "2", "https://example.com"}, "Navigated tab 2 to https://example.com"}} {
		for _, tty := range []bool{false, true} {
			cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
			if err := cache.save([]TabRef{{Window: "w", Tab: "first"}, {Window: "w", Tab: "secret"}}); err != nil {
				t.Fatal(err)
			}
			var out, errs bytes.Buffer
			status := run(tc.args, &out, &errs, func(c Command, _ func(Progress)) (Result, error) {
				if c.Args[1] != "secret" {
					t.Fatalf("wrong resolved tab: %+v", c)
				}
				return Result{Output: "Raw secret"}, nil
			}, cache, outputMode{stdoutTTY: tty})
			want := "Raw secret\n"
			if tty {
				want = "◇  " + tc.want + "\n"
			}
			if status != 0 || out.String() != want || errs.Len() != 0 {
				t.Fatalf("%q tty=%v: %d %q %q", tc.args, tty, status, out.String(), errs.String())
			}
		}
	}
}

func TestExplicitKeywordMoveSelector(t *testing.T) {
	for _, flag := range []string{"-k", "--keyword"} {
		for _, text := range []string{"youtube.com/watch", "--help", "--", "watch?v=1", "video tutorial"} {
			c, err := parse([]string{"mv", flag, text, "Watch", "-n", "-k", "second"})
			if err != nil || c.Name != "move-domain" || c.Target != "Watch" || !c.DryRun || len(c.Domains) != 0 || !reflect.DeepEqual(c.Keywords, []string{text, "second"}) {
				t.Errorf("%s %q: %+v %v", flag, text, c, err)
			}
		}
	}
	for _, args := range [][]string{{"mv", "-k"}, {"mv", "-k", "text"}, {"mv", "-k", "", "Watch"}, {"mv", "-k", "text", "-p", "Watch"}, {"mv", "-k", "text", "Watch", "-E"}} {
		if c, err := parse(args); err == nil {
			t.Errorf("accepted %q %+v", args, c)
		}
	}
	c, err := parse([]string{"mv", "-k", "--help", "--help"})
	if err != nil || c.Name != "help" {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = parse([]string{"mv", "-k", "--help", "--", "-h"})
	if err != nil || c.Target != "-h" || !reflect.DeepEqual(c.Keywords, []string{"--help"}) {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestLiteralKeywordFlagSelector(t *testing.T) {
	c, err := parse([]string{"mv", "--", "-k", "Watch"})
	if err != nil || !reflect.DeepEqual(c.Keywords, []string{"-k"}) || c.Target != "Watch" {
		t.Fatalf("%+v %v", c, err)
	}
	c, err = parse([]string{"mv", "-k", "/watch?v=", "--", "--help"})
	if err != nil || !reflect.DeepEqual(c.Keywords, []string{"/watch?v="}) || c.Target != "--help" {
		t.Fatalf("%+v %v", c, err)
	}
}
