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
