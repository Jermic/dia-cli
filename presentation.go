package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type presentation struct {
	command          Command
	out, errs        io.Writer
	mode             outputMode
	live, lastActive string
	planned          bool
}

func (p *presentation) title() string {
	if p.command.Name == "dedupe" {
		if p.command.DryRun {
			return "Dedupe preview"
		}
		return "Dedupe tabs"
	}
	if p.command.Name == "move-domain" {
		if p.command.DryRun {
			return "Move preview"
		}
		return "Move tabs"
	}
	return map[string]string{"history": "Search history", "bookmarks": "Search bookmarks", "domains": "Domains", "tabs": "Tabs", "find": "Search tabs", "current": "Current tab", "profiles": "Profiles", "windows": "Windows"}[p.command.Name]
}

func (p *presentation) start() {
	if p.command.JSON || !p.mode.stdoutTTY || p.title() == "" {
		return
	}
	fmt.Fprintf(p.out, "┌  %s\n│\n", p.title())
	c := p.command
	context := false
	for _, pair := range [][2]string{{"Profile", c.Profile}, {"Domain", displayDomains(c.Domains)}, {"Keyword", strings.Join(c.Keywords, ", ")}, {"Exclude", displayDomains(c.ExcludeDomains)}} {
		if pair[1] != "" {
			fmt.Fprintf(p.out, "│  %-7s %s\n", pair[0], safeText(pair[1]))
			context = true
		}
	}
	if c.Unpinned {
		fmt.Fprintln(p.out, "│  State   Unpinned")
		context = true
	} else if c.Pinned {
		fmt.Fprintln(p.out, "│  State   Pinned")
		context = true
	} else if c.Name == "move-domain" {
		fmt.Fprintln(p.out, "│  State   All tabs")
		context = true
	}
	if len(c.Domains) > 0 {
		for _, d := range c.Domains {
			if !d.Subdomains && !d.Exact {
				fmt.Fprintln(p.out, "│          includes subdomains")
				break
			}
		}
	}
	if c.Name == "find" || isLocalSearch(c.Name) {
		fmt.Fprintf(p.out, "│  Search  %s\n", safeText(c.Args[0]))
		context = true
	}
	if c.Name == "move-domain" {
		fmt.Fprintf(p.out, "│  Target  %s\n", safeText(c.Target))
		context = true
	}
	if context {
		fmt.Fprintln(p.out, "│")
	}
}

func (p *presentation) clear() {
	if p.live != "" {
		fmt.Fprint(p.errs, "\r\x1b[2K")
		p.live = ""
	}
}

func (p *presentation) progress(event Progress) {
	if !p.mode.stderrTTY {
		return
	}
	switch event.Phase {
	case "done", "failure":
		return
	case "plan":
		if p.planned || event.Total == 0 {
			return
		}
		p.clear()
		p.planned = true
		if p.command.Name == "dedupe" {
			fmt.Fprintf(p.errs, "◇  Found %s\n", quantity(event.Total, "duplicate tab"))
		} else {
			fmt.Fprintf(p.errs, "◇  Found %s\n◇  Target ready\n", quantity(event.Total, "matching tab"))
		}
		return
	}
	label := "Reading tabs"
	if event.Phase == "moving" || event.Phase == "moved" {
		label = fmt.Sprintf("Moving tabs · %d/%d", event.Completed, event.Total)
	}
	if event.Phase == "closing" || event.Phase == "closed" {
		label = fmt.Sprintf("Closing duplicates · %d/%d", event.Completed, event.Total)
	}
	if label == p.lastActive {
		return
	}
	p.lastActive = label
	p.clear()
	columns := terminalColumns(os.Stderr)
	if columns <= 0 {
		columns = 80
	}
	p.live = strings.TrimRight(cell("●  "+label, max(1, columns-1)), " ")
	fmt.Fprint(p.errs, p.live)
}

func (p *presentation) body(text string) {
	for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
		fmt.Fprintln(p.out, "│  "+line)
	}
}

func (p *presentation) footer(text string) { fmt.Fprintf(p.out, "│\n└  %s\n", text) }

func (p *presentation) fail(reason string) {
	p.clear()
	if p.mode.stderrTTY {
		fmt.Fprint(p.errs, "✕  ")
	}
	fmt.Fprintln(p.errs, reason)
	if !p.command.JSON && p.mode.stdoutTTY && p.title() != "" {
		p.footer("Failed")
	}
}

func quantity(n int, noun string) string {
	if n != 1 {
		noun += "s"
	}
	return fmt.Sprintf("%d %s", n, noun)
}

func (p *presentation) finish(result Result) {
	p.clear()
	if p.command.JSON {
		var value any = result.History
		if p.command.Name == "bookmarks" {
			value = result.Bookmarks
		}
		if err := json.NewEncoder(p.out).Encode(value); err != nil {
			p.fail(err.Error())
		}
		return
	}
	if result.Dedupe != nil {
		p.finishDedupe(result)
		return
	}
	if result.Move != nil {
		p.finishMove(result)
		return
	}
	if result.Error != "" {
		p.fail(result.Error)
		return
	}
	if !p.mode.stdoutTTY || p.command.Name == "links" || p.command.Name == "count" || p.command.Name == "exec" {
		if result.Output != "" {
			fmt.Fprintln(p.out, result.Output)
		}
		return
	}
	if p.title() == "" {
		if result.Output != "" {
			text := result.Output
			c := p.command
			if isRow(c.Name) && c.Name != "exec" {
				verb := map[string]string{"open": "Focused", "close": "Closed", "move": "Moved", "navigate": "Navigated"}[c.Name]
				text = verb + " tab " + c.Args[0]
				if c.Name == "move" {
					text += " to " + safeText(c.Args[1])
				}
				if c.Name == "navigate" {
					text += " to " + safeText(c.Args[1])
				}
			}
			fmt.Fprintln(p.out, "◇  "+text)
		}
		return
	}
	n := max(0, len(strings.Split(strings.TrimSpace(result.Output), "\n"))-1)
	noun := "tab"
	if p.command.Name == "history" {
		noun = "history match"
	}
	if p.command.Name == "bookmarks" {
		noun = "bookmark"
	}
	if p.command.Name == "domains" {
		noun = "domain"
	}
	if p.command.Name == "profiles" {
		noun = "profile"
	}
	if p.command.Name == "windows" {
		noun = "window"
	}
	if n == 0 {
		p.footer("No " + noun + "s found")
		return
	}
	switch p.command.Name {
	case "tabs", "find", "current":
		p.body(renderTabs(result.Output, p.command.Detail || p.command.Name != "tabs"))
	case "history", "bookmarks":
		p.body(renderTable(result.Output, nil))
	case "domains":
		p.body(renderTable(result.Output, nil, 1))
	case "profiles":
		p.body(renderTable(result.Output, []string{"WINDOW"}, 1, 3))
	case "windows":
		p.body(renderTable(result.Output, nil))
	}
	p.footer(quantity(n, noun))
}

func (p *presentation) finishMove(result Result) {
	m := result.Move
	if !p.mode.stdoutTTY {
		if result.Output != "" {
			fmt.Fprintln(p.out, result.Output)
		}
		prefix := ""
		if p.command.DryRun {
			prefix = "dry-run "
		}
		fmt.Fprintf(p.out, "%smatched=%d moved=%d skipped=%d failed=%d remaining=%d\n", prefix, m.Matched, m.Moved, m.Skipped, m.Failed, m.Remaining)
		if result.Error != "" {
			p.fail(result.Error)
		} else if p.mode.stderrTTY {
			label := "Done · " + quantity(m.Moved, "tab") + " moved"
			if p.command.DryRun {
				label = "Preview complete · no changes made"
			}
			fmt.Fprintln(p.errs, "└  "+label)
		}
		return
	}
	if result.Error != "" {
		fmt.Fprintln(p.out, "▲  Move stopped")
		p.body(renderTable(fmt.Sprintf("RESULT\tCOUNT\nmatched\t%d\nmoved\t%d\nskipped\t%d\nfailed\t%d\nremaining\t%d", m.Matched, m.Moved, m.Skipped, m.Failed, m.Remaining), nil, 1))
		if p.mode.stderrTTY {
			fmt.Fprint(p.errs, "✕  ")
		}
		fmt.Fprintln(p.errs, result.Error)
		p.footer("Incomplete · " + quantity(m.Moved, "tab") + " moved")
		return
	}
	if m.Matched == 0 {
		p.footer("No matching tabs · no changes made")
		return
	}
	if p.command.DryRun {
		if result.Output != "" {
			p.body(renderTable(result.Output, []string{"WINDOW", "TAB"}))
		}
		summary := quantity(m.Remaining, "tab") + " would move"
		if m.Skipped > 0 {
			summary += fmt.Sprintf(" · %d skipped", m.Skipped)
		}
		p.footer(summary + " · no changes made")
		return
	}
	if !p.planned {
		fmt.Fprintf(p.out, "◇  Found %s\n◇  Target ready\n", quantity(m.Matched, "matching tab"))
	}
	if m.Moved > 0 {
		fmt.Fprintf(p.out, "◇  Moved %s\n", quantity(m.Moved, "tab"))
	}
	if m.Skipped > 0 {
		fmt.Fprintf(p.out, "◇  Skipped %s · already in %s\n", quantity(m.Skipped, "tab"), safeText(p.command.Target))
	}
	p.footer("Done")
}

func (p *presentation) finishDedupe(result Result) {
	d := result.Dedupe
	if p.mode.stdoutTTY && d.Groups == 0 && result.Error == "" {
		p.footer("No duplicate tabs · no changes made")
		return
	}
	counts := fmt.Sprintf("METRIC\tCOUNT\ngroups\t%d\nduplicates\t%d\nclosed\t%d\nskipped\t%d\nfailed\t%d\nremaining\t%d", d.Groups, d.Duplicates, d.Closed, d.Skipped, d.Failed, d.Remaining)
	if result.Output != "" {
		if p.mode.stdoutTTY {
			p.body(renderTable(result.Output, []string{"WINDOW", "TAB"}))
		} else {
			fmt.Fprintln(p.out, result.Output)
		}
	}
	if p.mode.stdoutTTY {
		p.body(renderTable(counts, nil, 1))
	} else {
		fmt.Fprintln(p.out, counts)
	}
	summary := quantity(d.Closed, "tab") + " closed"
	if p.command.DryRun {
		summary = quantity(d.Remaining, "tab") + " would close · no changes made"
	}
	if result.Error != "" {
		if p.mode.stderrTTY {
			fmt.Fprint(p.errs, "✕  ")
		}
		fmt.Fprintln(p.errs, result.Error)
		summary = "Incomplete · " + quantity(d.Closed, "tab") + " closed"
	}
	if p.mode.stdoutTTY {
		p.footer(summary)
	} else if p.mode.stderrTTY {
		fmt.Fprintln(p.errs, "└  "+summary)
	}
}
