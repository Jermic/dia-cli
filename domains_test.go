package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDomainCounts(t *testing.T) {
	urls := []string{"https://NEWS.BBC.CO.UK.:443/a", "https://bbc.co.uk", "https://docs.github.com/a", "https://github.com", "https://alice.github.io", "https://blog.alice.github.io", "https://bob.github.io", "http://127.0.0.1:80", "http://[::1]:80", "http://localhost:3000", "", "about:blank", "not a URL", "https://bad host/", "https://bad..com/", "https://example.com/path with space", "https://user%zz@example.com/"}
	want := []DomainCount{{"(no domain)", 7}, {"alice.github.io", 2}, {"bbc.co.uk", 2}, {"github.com", 2}, {"127.0.0.1", 1}, {"::1", 1}, {"bob.github.io", 1}, {"localhost", 1}}
	if got := domainCounts(urls, false); !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	want = []DomainCount{{"a.example.com", 2}, {"example.com", 1}, {"z.example.com", 1}}
	if got := domainCounts([]string{"https://z.example.com", "https://a.example.com", "https://a.example.com/second", "https://example.com"}, true); !reflect.DeepEqual(got, want) {
		t.Fatalf("%+v", got)
	}
	if got := domainCounts(nil, false); len(got) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestDomainsParse(t *testing.T) {
	for _, name := range []string{"domains", "ds"} {
		c, err := parse([]string{name, "-s", "-p", "Work", "-w", "w1", "-D", "github.com", "-D", "*.example.com", "-E", "-k", "watch", "-k", "video", "--pinned", "--loading"})
		if err != nil || c.Name != "domains" || !c.Subdomains || c.Profile != "Work" || c.Window != "w1" || len(c.Domains) != 2 || len(c.Keywords) != 2 || !c.Domains[0].Exact || !c.Pinned || !c.Loading {
			t.Fatalf("%+v %v", c, err)
		}
	}
	c, err := parse([]string{"ds", "--subdomains"})
	if err != nil || !c.Subdomains {
		t.Fatalf("%+v %v", c, err)
	}
	for _, args := range [][]string{{"ds", "-d"}, {"ds", "-st"}, {"ds", "-sl"}, {"ds", "-c"}, {"ds", "-n"}, {"ds", "-E"}, {"ds", "-p"}, {"ds", "-D", "youtube"}, {"ds", "--", "-s"}, {"links", "--subdomains"}} {
		if _, err := parse(args); err == nil {
			t.Errorf("accepted %q", args)
		}
	}
	c, err = parse([]string{"tabs", "-s"})
	if err != nil || c.Sort != "title" || c.Subdomains {
		t.Fatalf("tabs -s changed: %+v %v", c, err)
	}
}

func TestDomainsOutputAndCache(t *testing.T) {
	for _, tty := range []bool{false, true} {
		cache := &rowCache{path: filepath.Join(t.TempDir(), "rows")}
		if err := cache.save([]TabRef{{Window: "w", Tab: "saved"}}); err != nil {
			t.Fatal(err)
		}
		before, _ := os.ReadFile(cache.path)
		calls := 0
		var out, errs bytes.Buffer
		status := run([]string{"ds"}, &out, &errs, func(c Command, _ func(Progress)) (Result, error) {
			calls++
			return Result{URLs: []string{"https://docs.example.com", "https://example.com", ""}}, nil
		}, cache, outputMode{stdoutTTY: tty})
		after, _ := os.ReadFile(cache.path)
		if status != 0 || calls != 1 || errs.Len() != 0 || !bytes.Equal(before, after) {
			t.Fatalf("status=%d calls=%d out=%q err=%q", status, calls, out.String(), errs.String())
		}
		if !tty && out.String() != "DOMAIN\tCOUNT\nexample.com\t2\n(no domain)\t1\n" {
			t.Fatalf("%q", out.String())
		}
		if tty && (!strings.Contains(out.String(), "DOMAIN") || !strings.Contains(out.String(), "2 domains") || strings.Contains(out.String(), "WINDOW")) {
			t.Fatalf("%q", out.String())
		}
	}
	var out, errs bytes.Buffer
	status := run([]string{"ds"}, &out, &errs, func(Command, func(Progress)) (Result, error) { return Result{}, nil }, nil, outputMode{stdoutTTY: true})
	if status != 0 || !strings.Contains(out.String(), "No domains found") {
		t.Fatalf("%q %q", out.String(), errs.String())
	}
}
