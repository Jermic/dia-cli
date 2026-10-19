package main

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestCommandHelpRoutes(t *testing.T) {
	want := map[string]string{
		"dedupe":  "dia dedupe [flags]",
		"domains": "dia domains [flags]", "help": "dia help [command]", "version": "dia version", "tabs": "dia tabs [flags]",
		"find": "dia find <text> [flags]", "current": "dia current [-w <id>]",
		"count": "dia count [flags]", "links": "dia links [flags]", "open": "dia open <row>",
		"focus": "dia focus <tab-id> [-w <id>]", "close": "dia close <row>",
		"move": "dia move <row> <profile>", "move-domain": "dia move-domain <selector> <target-profile> [flags]",
		"navigate": "dia navigate <row> <url>", "exec": "dia exec <row> <javascript>",
		"windows": "dia windows [-w <id>]", "profiles": "dia profiles [flags]",
		"profile": "dia profile <name> [-w <id>]", "layout": "dia layout <sidebar|tabStrip> [-w <id>]",
	}
	if len(want) != len(arities) || len(commandHelp) != len(want) {
		t.Fatalf("command help coverage: expected=%d arities=%d help=%d", len(want), len(arities), len(commandHelp))
	}
	for name := range arities {
		if want[name] == "" {
			t.Errorf("missing expected usage for %s", name)
		}
	}
	for name, usage := range want {
		var reference string
		for _, args := range [][]string{{"help", name}, {name, "-h"}, {name, "--help"}} {
			var out, stderr bytes.Buffer
			status := run(args, &out, &stderr, func(Command, func(Progress)) (Result, error) {
				t.Fatalf("%q invoked Dia", args)
				return Result{}, nil
			}, nil, outputMode{stdoutTTY: true})
			if status != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "USAGE\n  "+usage+"\n") || !strings.Contains(out.String(), "-h, --help") {
				t.Errorf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
			}
			if reference != "" && out.String() != reference {
				t.Errorf("%q differs from dia help %s", args, name)
			}
			reference = out.String()
		}
	}
	for alias, canonical := range aliases {
		for _, args := range [][]string{{"help", alias}, {alias, "-h"}, {alias, "--help"}} {
			var out, stderr bytes.Buffer
			status := run(args, &out, &stderr, func(Command, func(Progress)) (Result, error) {
				t.Fatalf("%q invoked Dia", args)
				return Result{}, nil
			}, nil, outputMode{stdoutTTY: true})
			if status != 0 || stderr.Len() != 0 || !strings.Contains(out.String(), "USAGE\n  "+want[canonical]+"\n") {
				t.Errorf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
			}
		}
	}
	for _, args := range [][]string{{"help", "-h"}, {"h", "--help"}} {
		c, err := parse(args)
		if err != nil || c.Name != "help" || !reflect.DeepEqual(c.Args, []string{"help"}) {
			t.Errorf("%q: %+v %v", args, c, err)
		}
	}
}

func TestHelpRootAndInvalidTargets(t *testing.T) {
	for _, args := range [][]string{nil, {"-h"}, {"--help"}, {"help"}, {"h"}} {
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command, func(Progress)) (Result, error) {
			t.Fatalf("%q invoked Dia", args)
			return Result{}, nil
		}, nil, outputMode{stdoutTTY: true})
		if status != 0 || stderr.Len() != 0 || out.String() != help+"\n" || !strings.Contains(out.String(), "dia l -p Github -D github.com") || !strings.Contains(out.String(), "dia mv github.com Work") {
			t.Errorf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
		}
	}
	for _, args := range [][]string{{"help", "unknown"}, {"help", "tabs", "extra"}, {"h", "mv", "extra"}} {
		var out, stderr bytes.Buffer
		status := run(args, &out, &stderr, func(Command, func(Progress)) (Result, error) {
			t.Fatalf("%q invoked Dia", args)
			return Result{}, nil
		}, nil, outputMode{stdoutTTY: true})
		if status != 2 || out.Len() != 0 || stderr.Len() == 0 {
			t.Errorf("%q: status=%d output=%q stderr=%q", args, status, out.String(), stderr.String())
		}
	}
}

func TestHelpKeepsArgumentValues(t *testing.T) {
	for _, tc := range []struct {
		args []string
		name string
		arg  string
		flag string
	}{
		{[]string{"exec", "1", "--", "-h"}, "exec", "-h", ""},
		{[]string{"exec", "1", "-v"}, "exec", "-v", ""},
		{[]string{"move-domain", "example.com", "-v"}, "move-domain", "-v", ""},
		{[]string{"tabs", "-p", "-v"}, "tabs", "", "-v"},
		{[]string{"move", "1", "--", "-h"}, "move", "-h", ""},
		{[]string{"tabs", "-p", "-h"}, "tabs", "", "-h"},
		{[]string{"tabs", "-w", "-h"}, "tabs", "", "-h"},
		{[]string{"move-domain", "example.com", "--", "-h"}, "move-domain", "-h", ""},
	} {
		c, err := parse(tc.args)
		if err != nil || c.Name != tc.name {
			t.Errorf("%q: %+v %v", tc.args, c, err)
			continue
		}
		if tc.arg != "" && c.Args[len(c.Args)-1] != tc.arg || tc.flag != "" && c.Profile != tc.flag && c.Window != tc.flag {
			t.Errorf("%q lost data: %+v", tc.args, c)
		}
	}
	for _, args := range [][]string{{"find", "github", "-h"}, {"move-domain", "example.com", "Work", "-h"}, {"exec", "1", "script", "--help"}} {
		c, err := parse(args)
		if err != nil || c.Name != "help" || len(c.Args) != 1 {
			t.Errorf("%q did not show scoped help: %+v %v", args, c, err)
		}
	}
	for _, alias := range []string{"mv", "md", "move-domain"} {
		c, err := parse([]string{alias, "example.com", "Work", "-n"})
		if err != nil || c.Name != "move-domain" || !c.DryRun || c.Target != "Work" {
			t.Errorf("%s differs from move-domain: %+v %v", alias, c, err)
		}
	}
}
