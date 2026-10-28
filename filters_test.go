package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNegativeFilters(t *testing.T) {
	for _, args := range [][]string{
		{"l", "--unpinned", "--exclude-domain", "x.com"},
		{"ds", "-up", "-exd", "x.com", "-exd", "*.example.com", "-E"},
		{"mv", "Read", "-up", "-exd", "x.com", "-n"},
		{"mv", "Read", "-k", "youtube", "-n"},
		{"mv", "github.com", "Read", "-up", "-exd", "docs.github.com", "-n"},
		{"ps", "-c", "-exd", "x.com"},
	} {
		c, err := parse(args)
		if err != nil {
			t.Fatalf("%q: %v", args, err)
		}
		if c.Name == "move-domain" && (c.Target != "Read" || !c.DryRun) {
			t.Fatalf("%q: %+v", args, c)
		}
		data, _ := json.Marshal(c)
		if args[0] != "ps" && args[2] != "-k" && !strings.Contains(string(data), `"unpinned":true`) {
			t.Fatalf("%q did not select unpinned: %s", args, data)
		}
	}
	for _, args := range [][]string{
		{"l", "--pinned", "-up"}, {"l", "-up", "--pinned"},
		{"l", "-exd"}, {"l", "-exd", "youtube"}, {"l", "-exd", "https://x.com"},
		{"l", "-exd", "--help"},
		{"ps", "-exd", "x.com"}, {"current", "-up"}, {"windows", "-exd", "x.com"},
		{"mv", "Read", "-n"}, {"mv", "skills.sh", "-p", "GitHub"},
	} {
		if c, err := parse(args); err == nil {
			t.Errorf("accepted %q: %+v", args, c)
		}
	}
}

func TestNegativeFilterDomainSemantics(t *testing.T) {
	c, err := parse([]string{"ds", "-up", "-exd", "X.COM", "--exclude-domain", "*.example.com", "-E"})
	if err != nil || !c.Unpinned || len(c.ExcludeDomains) != 2 || c.ExcludeDomains[0].Host != "x.com" || !c.ExcludeDomains[0].Exact || !c.ExcludeDomains[1].Subdomains || c.ExcludeDomains[1].Exact {
		t.Fatalf("excluded domain semantics: %+v %v", c, err)
	}
	for _, args := range [][]string{{"md", "skills.sh", "-D", "*.skills.sh"}, {"mv", "Read", "-p", "Work", "-up", "-n"}} {
		if c, err := parse(args); err != nil || c.Target != args[1] {
			t.Errorf("%q: %+v %v", args, c, err)
		}
	}
	for _, args := range [][]string{{"l", "-exd", "--help"}, {"mv", "Read", "-exd", "--help", "-n"}} {
		if c, err := parse(args); err == nil || !strings.Contains(err.Error(), "invalid domain") || c.Name == "help" {
			t.Errorf("domain value became help: %+v %v", c, err)
		}
	}
}

func TestTargetOnlyMoveContext(t *testing.T) {
	var out, errs bytes.Buffer
	status := run([]string{"mv", "Read", "-up", "-exd", "x.com", "-n"}, &out, &errs, func(c Command, _ func(Progress)) (Result, error) {
		if c.Target != "Read" || len(c.Domains) != 0 || len(c.Keywords) != 0 || !c.DryRun {
			t.Fatalf("wrong target-only plan: %+v", c)
		}
		return Result{Move: &MoveResult{}}, nil
	}, nil, outputMode{stdoutTTY: true})
	if status != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "Unpinned") || !strings.Contains(out.String(), "x.com") || !strings.Contains(out.String(), "Read") {
		t.Fatalf("status=%d out=%q err=%q", status, out.String(), errs.String())
	}
}

func TestMovePinnedPolicy(t *testing.T) {
	for _, name := range []string{"move-domain", "mv", "md"} {
		for _, flags := range [][]string{nil, {"-up"}, {"--pinned"}, {"-a"}, {"--include-pinned"}} {
			args := append([]string{name, "github.com", "GitHub"}, flags...)
			c, err := parse(args)
			if err != nil {
				t.Fatalf("%q: %v", args, err)
			}
			wantPinned := len(flags) > 0 && flags[0] == "--pinned"
			wantUnpinned := len(flags) == 0 || flags[0] == "-up"
			data, _ := json.Marshal(c)
			if c.Pinned != wantPinned || c.Unpinned != wantUnpinned || strings.Contains(string(data), `"unpinned":true`) != wantUnpinned {
				t.Errorf("%q: wrong pin selection %s", args, data)
			}
		}
	}
	for _, include := range []string{"-a", "--include-pinned"} {
		for _, state := range []string{"--pinned", "--unpinned", "-up"} {
			for _, flags := range [][]string{{include, state}, {state, include}} {
				if c, err := parse(append([]string{"mv", "github.com", "GitHub"}, flags...)); err == nil {
					t.Errorf("accepted conflicting flags %q: %+v", flags, c)
				}
			}
		}
	}
	for _, args := range [][]string{
		{"mv", "Read", "-p", "Day", "-n"}, {"mv", "Read", "-a", "-n"}, {"mv", "Read", "--include-pinned", "-n"},
		{"mv", "github.com", "GitHub", "--apply"}, {"t", "--include-pinned"}, {"dd", "--include-pinned"},
	} {
		if c, err := parse(args); err == nil {
			t.Errorf("accepted %q: %+v", args, c)
		}
	}
	for _, args := range [][]string{{"t"}, {"n"}, {"l"}, {"ds"}, {"dd"}, {"m", "2", "Read"}} {
		if c, err := parse(args); err != nil || c.Pinned || c.Unpinned {
			t.Errorf("changed non-batch default %q: %+v %v", args, c, err)
		}
	}
	if c, err := parse([]string{"dd", "-a"}); err != nil || c.DryRun {
		t.Fatalf("dedupe apply changed: %+v %v", c, err)
	}
	if c, err := parse([]string{"mv", "Read", "-exd", "x.com", "-a", "-n"}); err != nil || c.Target != "Read" || c.Unpinned || len(c.ExcludeDomains) != 1 {
		t.Fatalf("include-pinned cleared selection: %+v %v", c, err)
	}
}

func TestProfileCountStateFilters(t *testing.T) {
	for _, flag := range []string{"--pinned", "--unpinned", "-up", "--loading"} {
		for _, args := range [][]string{{"ps", "-c", flag}, {"profiles", flag, "--counts"}} {
			if c, err := parse(args); err != nil || !c.Counts || !(c.Pinned || c.Unpinned || c.Loading) {
				t.Errorf("%q: %+v %v", args, c, err)
			}
		}
		if _, err := parse([]string{"ps", flag}); err == nil {
			t.Errorf("accepted state filter without counts: %s", flag)
		}
	}
	if _, err := parse([]string{"ps", "-c", "--pinned", "-up"}); err == nil {
		t.Fatal("accepted conflicting profile pin filters")
	}
}

func TestMovePinPolicyReachesBackend(t *testing.T) {
	for _, tc := range []struct {
		flags    []string
		state    string
		pinned   bool
		unpinned bool
	}{
		{nil, "Unpinned", false, true},
		{[]string{"-a"}, "All tabs", false, false},
		{[]string{"--pinned"}, "Pinned", true, false},
	} {
		var out, errs bytes.Buffer
		called := false
		args := append([]string{"mv", "github.com", "GitHub", "-n"}, tc.flags...)
		status := run(args, &out, &errs, func(c Command, _ func(Progress)) (Result, error) {
			called = true
			if c.Pinned != tc.pinned || c.Unpinned != tc.unpinned || !c.DryRun || c.Target != "GitHub" || len(c.Domains) != 1 {
				t.Fatalf("wrong backend selection: %+v", c)
			}
			return Result{Move: &MoveResult{}}, nil
		}, nil, outputMode{stdoutTTY: true})
		if !called || status != 0 || errs.Len() != 0 || !strings.Contains(out.String(), "State   "+tc.state) {
			t.Errorf("%q status=%d out=%q err=%q", args, status, out.String(), errs.String())
		}
	}
}
