package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"unicode"
)

//go:embed scripts/dia.js
var scripts embed.FS

var version = "dev"

type DomainPattern struct {
	Host       string `json:"host"`
	Subdomains bool   `json:"subdomains"`
	Exact      bool   `json:"exact,omitempty"`
}

type TabRef struct {
	Window string `json:"window"`
	Tab    string `json:"tab"`
}

type Command struct {
	Name     string          `json:"name"`
	Args     []string        `json:"args,omitempty"`
	Window   string          `json:"window,omitempty"`
	Profile  string          `json:"profile,omitempty"`
	Domains  []DomainPattern `json:"domains,omitempty"`
	Target   string          `json:"target,omitempty"`
	Detail   bool            `json:"detail,omitempty"`
	Sort     string          `json:"sort,omitempty"`
	Pinned   bool            `json:"pinned,omitempty"`
	Loading  bool            `json:"loading,omitempty"`
	DryRun   bool            `json:"dryRun,omitempty"`
	Counts   bool            `json:"counts,omitempty"`
	Progress bool            `json:"progress,omitempty"`
}

type Progress struct {
	Phase     string `json:"phase"`
	Window    string `json:"window,omitempty"`
	Profile   string `json:"profile,omitempty"`
	Completed int    `json:"completed"`
	Total     int    `json:"total"`
}

type outputMode struct {
	stdoutTTY bool
	stderrTTY bool
}

type MoveResult struct {
	Matched   int `json:"matched"`
	Moved     int `json:"moved"`
	Skipped   int `json:"skipped"`
	Failed    int `json:"failed"`
	Remaining int `json:"remaining"`
}

type Result struct {
	Output string      `json:"output"`
	Tabs   []TabRef    `json:"tabs,omitempty"`
	Move   *MoveResult `json:"move,omitempty"`
	Error  string      `json:"error,omitempty"`
}

const help = `Control Dia tabs, windows, and profiles from your terminal.

USAGE
  dia <command> [arguments] [flags]

CORE COMMANDS
  tabs, t                 List tabs, sorted by title
  find, f <text>          Search titles and URLs
  current, c              Show the active tab
  count, n                Count tabs
  links, l                Print full URLs, one per line
  open, o <row>           Focus a saved row
  help, h                 Show this help
  version, --version      Show the installed version

TAB COMMANDS
  focus, fo <tab-id>       Focus a tab by ID
  close, cl <row>         Close a saved row
  move, m <row> <profile> Move a saved row in its window
  move-domain, mv <domain> <target>  Move matching tabs in one batch (md also works)
  navigate, nav <row> <url>  Navigate a saved row
  exec, e <row> <javascript> Run JavaScript in a saved row

WINDOW AND PROFILE COMMANDS
  windows, w              List windows and their active profiles
  profiles, ps            List profiles (--counts, -c for tab counts)
  profile, p <name>       Focus a profile by exact name
  layout, la <layout>     Set sidebar or tabStrip layout

FILTERS AND FLAGS
  --profile, -p <name>    Select a source profile (case-sensitive)
  --domain, -D <domain>   Match HTTP(S) root + subdomains; repeatable
                         Use *.example.com for subdomains only
  --exact-domain, -E      Match plain domains exactly (requires a domain)
  --window, -w <id>       Select one window
  --dry-run, -n           Plan move-domain without moving
  --counts, -c            Show profile membership counts
  -d, -detail             Show full URLs beside tab titles
  -s, -st / -sl           Sort tabs by title / URL (-sl requires -d)
  --pinned, --loading     Require these tab states
  --help, -h              Show help

Rows come from the last visible tabs/find list in the same terminal and shell.
Move-domain never creates profiles or moves between windows.
Move progress goes to terminal stderr. Piped stdout keeps machine-readable output.

EXAMPLES
  dia help <command>
  dia <command> --help
  dia l -p Github -D github.com
  dia mv github.com Work -n`

var aliases = map[string]string{
	"t": "tabs", "n": "count", "l": "links", "c": "current", "f": "find", "o": "open", "fo": "focus",
	"ps": "profiles", "p": "profile", "h": "help", "w": "windows", "cl": "close", "m": "move", "mv": "move-domain", "md": "move-domain",
	"nav": "navigate", "la": "layout", "e": "exec",
}

var arities = map[string]int{
	"help": 0, "version": 0, "tabs": 0, "links": 0, "count": 0, "current": 0, "find": 1, "focus": 1, "profiles": 0,
	"profile": 1, "windows": 0, "layout": 1, "open": 1, "close": 1, "move": 2, "move-domain": 2,
	"navigate": 2, "exec": 2,
}

func domain(raw string) (DomainPattern, error) {
	p := DomainPattern{Host: strings.ToLower(raw)}
	if strings.HasPrefix(p.Host, "*.") {
		p.Subdomains = true
		p.Host = strings.TrimPrefix(p.Host, "*.")
	}
	if len(p.Host) > 253 || !strings.Contains(p.Host, ".") || strings.HasSuffix(p.Host, ".") {
		return p, fmt.Errorf("invalid domain: %q", raw)
	}
	for _, label := range strings.Split(p.Host, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return p, fmt.Errorf("invalid domain: %q", raw)
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return p, fmt.Errorf("invalid domain: %q", raw)
			}
		}
	}
	return p, nil
}

func webURL(raw string) bool {
	for _, ch := range raw {
		if unicode.IsSpace(ch) || unicode.IsControl(ch) {
			return false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || (!strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Scheme, "https")) || u.Hostname() == "" {
		return false
	}
	return true
}

func parse(argv []string) (Command, error) {
	c := Command{Name: "help", Sort: "title"}
	exactDomain := false
	if len(argv) == 0 {
		return c, nil
	}
	name := argv[0]
	if name == "-h" || name == "--help" {
		name = "help"
	} else if name == "--version" {
		name = "version"
	}
	if full := aliases[name]; full != "" {
		name = full
	}
	arity, ok := arities[name]
	if !ok {
		return c, fmt.Errorf("unknown command: %s", name)
	}
	c.Name = name
	if name == "help" {
		if len(argv) > 2 {
			return c, errors.New("usage: dia help [command] accepts at most one command")
		}
		if len(argv) == 2 {
			target := argv[1]
			if target == "-h" || target == "--help" {
				target = "help"
			}
			if full := aliases[target]; full != "" {
				target = full
			}
			if target == "--version" {
				target = "version"
			}
			if _, ok := arities[target]; !ok {
				return c, fmt.Errorf("unknown command: %s", argv[1])
			}
			c.Args = []string{target}
		}
		return c, nil
	}
	if len(argv) == 2 && (argv[1] == "-h" || argv[1] == "--help") {
		return Command{Name: "help", Args: []string{name}}, nil
	}
	if len(argv)-1 < arity {
		if name == "move-domain" && len(argv) > 1 {
			return c, errors.New("missing target profile: use dia mv skills.sh GitHub; -p selects the source profile")
		}
		return c, fmt.Errorf("usage: dia %s requires %d argument(s)", name, arity)
	}
	if name == "move-domain" && len(argv) > 2 {
		switch argv[2] {
		case "-p", "--profile", "-w", "--window", "-D", "--domain", "--pinned", "--loading", "-n", "--dry-run", "-E", "--exact-domain":
			return c, errors.New("missing target profile: use dia mv skills.sh GitHub; -p selects the source profile")
		}
	}
	c.Args = append([]string(nil), argv[1:1+arity]...)
	for _, value := range c.Args {
		if value == "" {
			return c, fmt.Errorf("usage: dia %s requires non-empty arguments", name)
		}
	}
	args := argv[1+arity:]
	for len(args) > 0 {
		flag := args[0]
		args = args[1:]
		switch flag {
		case "--window", "-w", "--profile", "-p", "--domain", "-D":
			if len(args) == 0 || args[0] == "" {
				return c, fmt.Errorf("%s requires a value", flag)
			}
			value := args[0]
			args = args[1:]
			switch flag {
			case "--window", "-w":
				if c.Window != "" || isRow(name) || name == "help" {
					return c, fmt.Errorf("--window is not valid here or was repeated")
				}
				c.Window = value
			case "--profile", "-p":
				if !(isFiltered(name) || name == "profiles") || c.Profile != "" {
					return c, fmt.Errorf("--profile is not valid here or was repeated")
				}
				c.Profile = value
			case "--domain", "-D":
				if !(isFiltered(name) || name == "profiles") {
					return c, errors.New("--domain is not valid here")
				}
				p, err := domain(value)
				if err != nil {
					return c, err
				}
				c.Domains = append(c.Domains, p)
			}
		case "--exact-domain", "-E":
			if !(isFiltered(name) || name == "profiles") {
				return c, errors.New("--exact-domain is not valid here")
			}
			exactDomain = true
		case "--pinned":
			if !isFiltered(name) {
				return c, errors.New("--pinned is not valid here")
			}
			c.Pinned = true
		case "--loading":
			if !isFiltered(name) {
				return c, errors.New("--loading is not valid here")
			}
			c.Loading = true
		case "-d", "-detail":
			if name != "tabs" {
				return c, errors.New("-d is valid only for tabs")
			}
			c.Detail = true
		case "-s", "-st", "-sl":
			if name != "tabs" {
				return c, errors.New("sort flags are valid only for tabs")
			}
			if flag == "-sl" {
				c.Sort = "url"
			} else {
				c.Sort = "title"
			}
		case "--dry-run", "-n":
			if name != "move-domain" {
				return c, errors.New("--dry-run is valid only for move-domain")
			}
			c.DryRun = true
		case "--counts", "-c":
			if name != "profiles" {
				return c, errors.New("--counts is valid only for profiles")
			}
			c.Counts = true
		case "--help", "-h":
			if len(args) != 0 {
				return c, errors.New("--help accepts no extra arguments")
			}
			return Command{Name: "help", Args: []string{name}}, nil
		default:
			return c, fmt.Errorf("unexpected argument: %s", flag)
		}
	}
	if c.Sort == "url" && !c.Detail {
		return c, errors.New("link sorting (-sl) requires -d (or -detail)")
	}
	if name == "profiles" && len(c.Domains) > 0 && !c.Counts {
		return c, errors.New("profiles --domain requires --counts")
	}
	if name == "layout" && c.Args[0] != "sidebar" && c.Args[0] != "tabStrip" {
		return c, errors.New("layout must be sidebar or tabStrip")
	}
	if name == "navigate" && !webURL(c.Args[1]) {
		return c, errors.New("navigation requires an absolute HTTP(S) URL with a valid host and no whitespace or control characters")
	}
	if name == "exec" && strings.TrimSpace(c.Args[1]) == "" {
		return c, errors.New("pass one non-empty JavaScript argument")
	}
	if isRow(name) {
		row := c.Args[0]
		value, err := strconv.ParseUint(row, 10, 32)
		if err != nil || value == 0 || strconv.FormatUint(value, 10) != row {
			return c, errors.New("row must be a positive number without leading zeros")
		}
	}
	if name == "move-domain" {
		p, err := domain(c.Args[0])
		if err != nil {
			return c, err
		}
		c.Domains = append([]DomainPattern{p}, c.Domains...)
		c.Target = c.Args[1]
	}
	if exactDomain {
		if len(c.Domains) == 0 {
			return c, errors.New("--exact-domain requires a domain")
		}
		for i := range c.Domains {
			c.Domains[i].Exact = !c.Domains[i].Subdomains
		}
	}
	return c, nil
}

func isFiltered(name string) bool {
	return name == "tabs" || name == "find" || name == "count" || name == "links" || name == "move-domain"
}

func isRow(name string) bool {
	return name == "open" || name == "close" || name == "move" || name == "navigate" || name == "exec"
}

func backend(c Command, onProgress func(Progress)) (Result, error) {
	source, err := scripts.ReadFile("scripts/dia.js")
	if err != nil {
		return Result{}, err
	}
	request, err := json.Marshal(c)
	if err != nil {
		return Result{}, err
	}
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", string(source), string(request))
	var diagnostics bytes.Buffer
	cmd.Stderr = &diagnostics
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, err
	}
	if err := cmd.Start(); err != nil {
		return Result{}, err
	}
	decoder := json.NewDecoder(stdout)
	var result Result
	final := false
	var protocolErr error
	for {
		var frame map[string]json.RawMessage
		err := decoder.Decode(&frame)
		if err == io.EOF {
			break
		}
		if err != nil {
			protocolErr = err
			break
		}
		if final {
			protocolErr = errors.New("frame after final result")
			break
		}
		if raw, ok := frame["progress"]; ok {
			var event Progress
			if len(frame) != 1 || !c.Progress {
				protocolErr = errors.New("unexpected progress frame")
				break
			}
			eventDecoder := json.NewDecoder(bytes.NewReader(raw))
			eventDecoder.DisallowUnknownFields()
			if err := eventDecoder.Decode(&event); err != nil {
				protocolErr = err
				break
			}
			if err := event.validate(); err != nil {
				protocolErr = err
				break
			}
			if onProgress != nil {
				onProgress(event)
			}
		} else {
			if _, ok := frame["output"]; !ok {
				protocolErr = errors.New("missing result output")
				break
			}
			raw, _ := json.Marshal(frame)
			if err := json.Unmarshal(raw, &result); err != nil {
				protocolErr = err
				break
			}
			final = true
		}
	}
	if protocolErr != nil {
		// A malformed stream may leave the child blocked on stdout.
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if protocolErr != nil {
		return Result{}, fmt.Errorf("invalid Dia response: %w%s", protocolErr, diagnosticSuffix(diagnostics.String()))
	}
	if waitErr != nil {
		return Result{}, fmt.Errorf("%w%s", waitErr, diagnosticSuffix(diagnostics.String()))
	}
	if !final {
		return Result{}, errors.New("invalid Dia response: missing final result")
	}
	if c.Name == "move-domain" && result.Move == nil {
		return Result{}, errors.New("invalid Dia response: missing move result")
	}
	return result, nil
}

func diagnosticSuffix(text string) string {
	if text = strings.TrimSpace(text); text != "" {
		return ": " + text
	}
	return ""
}

func (p Progress) validate() error {
	switch p.Phase {
	case "scan", "profile", "preflight", "plan", "moving", "moved", "failure", "done":
	default:
		return errors.New("unknown progress phase")
	}
	if p.Completed < 0 || p.Total < 0 || p.Completed > p.Total {
		return errors.New("invalid progress counts")
	}
	for _, value := range []string{p.Window, p.Profile} {
		for _, r := range value {
			if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
				return errors.New("invalid progress context")
			}
		}
	}
	return nil
}

func run(argv []string, out, errs io.Writer, invoke func(Command, func(Progress)) (Result, error), cache *rowCache, mode outputMode) int {
	c, err := parse(argv)
	if err != nil {
		fmt.Fprintln(errs, err)
		return 2
	}
	if c.Name == "help" {
		if len(c.Args) == 0 {
			fmt.Fprintln(out, help)
		} else {
			fmt.Fprintln(out, commandHelp[c.Args[0]]+"\n\nHELP\n  -h, --help              Show this command help")
		}
		return 0
	}
	if c.Name == "version" {
		fmt.Fprintln(out, "dia", version)
		return 0
	}
	if runtime.GOOS != "darwin" {
		fmt.Fprintln(errs, "dia-cli requires macOS.")
		return 1
	}
	if cache == nil && (isRow(c.Name) || mode.stdoutTTY && (c.Name == "tabs" || c.Name == "find")) {
		cache = newRowCache()
	}
	if isRow(c.Name) {
		ref, err := cache.lookup(c.Args[0])
		if err != nil {
			fmt.Fprintln(errs, err)
			return 1
		}
		c.Name = "_" + c.Name
		c.Args = append([]string{ref.Window, ref.Tab}, c.Args[1:]...)
	}
	var onProgress func(Progress)
	if c.Name == "move-domain" && mode.stderrTTY {
		c.Progress = true
		onProgress = func(p Progress) {
			if p.Phase != "done" {
				fmt.Fprintln(errs, renderProgress(p, c.DryRun))
			}
		}
	}
	result, err := invoke(c, onProgress)
	if err != nil {
		fmt.Fprintln(errs, err)
		return 1
	}
	if (c.Name == "tabs" || c.Name == "find") && mode.stdoutTTY && result.Error == "" {
		if err := cache.save(result.Tabs); err != nil {
			fmt.Fprintln(errs, err)
			return 1
		}
	}
	if result.Move != nil {
		if c.Progress && result.Error == "" {
			completed, total := result.Move.Moved, result.Move.Matched-result.Move.Skipped
			if c.DryRun {
				completed, total = result.Move.Matched, result.Move.Matched
			}
			fmt.Fprintln(errs, renderProgress(Progress{Phase: "done", Profile: c.Target, Completed: completed, Total: total}, c.DryRun))
		}
		if mode.stdoutTTY {
			if c.DryRun {
				fmt.Fprintln(out, "Preview (no tabs moved)")
			}
			fmt.Fprintf(out, "Domains: %s → %s\n", displayDomains(c.Domains), safeText(c.Target))
			if result.Move.Matched == 0 {
				fmt.Fprintln(out, "No matching tabs.")
			} else if result.Output != "" {
				fmt.Fprint(out, renderTable(result.Output, true))
			}
			m := result.Move
			fmt.Fprint(out, renderTable(fmt.Sprintf("METRIC\tCOUNT\nmatched\t%d\nmoved\t%d\nskipped\t%d\nfailed\t%d\nremaining\t%d", m.Matched, m.Moved, m.Skipped, m.Failed, m.Remaining), false, 1))
		} else {
			if result.Output != "" {
				fmt.Fprintln(out, result.Output)
			}
			prefix := ""
			if c.DryRun {
				prefix = "dry-run "
			}
			fmt.Fprintf(out, "%smatched=%d moved=%d skipped=%d failed=%d remaining=%d\n", prefix, result.Move.Matched, result.Move.Moved, result.Move.Skipped, result.Move.Failed, result.Move.Remaining)
		}
	} else if mode.stdoutTTY && (c.Name == "tabs" || c.Name == "find" || c.Name == "current") {
		fmt.Fprint(out, renderTabs(result.Output, c.Detail || c.Name == "find" || c.Name == "current"))
	} else if mode.stdoutTTY && c.Name == "profiles" {
		fmt.Fprint(out, renderTable(result.Output, true, 1, 3))
	} else if mode.stdoutTTY && c.Name == "windows" {
		fmt.Fprint(out, renderTable(result.Output, false))
	} else if result.Output != "" {
		fmt.Fprintln(out, result.Output)
	}
	if result.Error != "" {
		fmt.Fprintln(errs, result.Error)
		return 1
	}
	return 0
}

func main() {
	terminal := os.Getenv("TERM") != "dumb"
	mode := outputMode{stdoutTTY: terminal && isTerminal(os.Stdout), stderrTTY: terminal && isTerminal(os.Stderr)}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, backend, nil, mode))
}
