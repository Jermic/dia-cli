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
	Name       string          `json:"name"`
	Args       []string        `json:"args,omitempty"`
	Window     string          `json:"window,omitempty"`
	Profile    string          `json:"profile,omitempty"`
	Domains    []DomainPattern `json:"domains,omitempty"`
	Keywords   []string        `json:"keywords,omitempty"`
	Target     string          `json:"target,omitempty"`
	Detail     bool            `json:"detail,omitempty"`
	Sort       string          `json:"sort,omitempty"`
	Pinned     bool            `json:"pinned,omitempty"`
	Loading    bool            `json:"loading,omitempty"`
	DryRun     bool            `json:"dryRun,omitempty"`
	Counts     bool            `json:"counts,omitempty"`
	Progress   bool            `json:"progress,omitempty"`
	Subdomains bool            `json:"subdomains,omitempty"`
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

type DedupeResult struct {
	Groups     int `json:"groups"`
	Duplicates int `json:"duplicates"`
	Closed     int `json:"closed"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
	Remaining  int `json:"remaining"`
}

func (d *DedupeResult) validate(preview bool, reason string) error {
	if d == nil {
		return errors.New("invalid Dia response: missing dedupe result")
	}
	if d.Groups < 0 || d.Duplicates < 0 || d.Closed < 0 || d.Skipped < 0 || d.Failed < 0 || d.Remaining < 0 ||
		d.Closed > d.Duplicates || d.Skipped > d.Duplicates-d.Closed || d.Remaining != d.Duplicates-d.Closed-d.Skipped || d.Failed > d.Remaining ||
		preview && d.Closed != 0 || reason == "" && (d.Failed != 0 || !preview && d.Remaining != 0) {
		return errors.New("invalid Dia response: inconsistent dedupe counts")
	}
	return nil
}

type Result struct {
	Dedupe *DedupeResult `json:"dedupe,omitempty"`
	URLs   []string      `json:"urls,omitempty"`
	Output string        `json:"output"`
	Tabs   []TabRef      `json:"tabs,omitempty"`
	Move   *MoveResult   `json:"move,omitempty"`
	Error  string        `json:"error,omitempty"`
}

const help = `Control Dia tabs, windows, and profiles from your terminal.

USAGE
  dia <command> [arguments] [flags]

CORE COMMANDS
  dedupe, dd              Preview duplicate tabs; -a closes safe duplicates
  tabs, t                 List tabs, sorted by title
  find, f <text>          Search titles and URLs
  current, c              Show the active tab
  count, n                Count tabs
  domains, ds             Count tabs by root domain (-s for full hosts)
  links, l                Print full URLs, one per line
  open, o <row>           Focus a saved row
  help, h                 Show this help
  version, v, -v, --version  Show the installed version

TAB COMMANDS
  focus, fo <tab-id>       Focus a tab by ID
  close, cl <row>         Close a saved row
  move, m <row> <profile> Move a saved row in its window
  move-domain, mv <selector> <target>  Move matching tabs in one batch (md also works)
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
  --keyword, -k <text>   Match URL text, case-insensitive; repeatable
  --exact-domain, -E      Match plain domains exactly (requires a domain)
  --window, -w <id>       Select one window
  --dry-run, -n           Preview move-domain or dedupe without changes
  --apply, -a             Close safe duplicates (dedupe only)
  --counts, -c            Show profile membership counts
  -d, -detail             Show full URLs beside tab titles
  --subdomains, -s        Count full hosts instead of roots (domains only)
  -s, -st / -sl           Sort tabs by title / URL (-sl requires -d)
  --pinned, --loading     Require these tab states
  --help, -h              Show help

Rows come from the last visible tabs/find list in the same terminal and shell.
Domains and URL keywords form a union; other filters intersect.
Use -- before literal positional arguments such as a profile named --help.
Move-domain never creates profiles or moves between windows.
Move progress goes to terminal stderr. Piped stdout keeps machine-readable output.

EXAMPLES
  dia help <command>
  dia <command> --help
  dia l -p Github -D github.com
  dia mv github.com Work -n
  dia mv youtube Watch -n`

var aliases = map[string]string{
	"dd": "dedupe", "ds": "domains", "t": "tabs", "n": "count", "l": "links", "c": "current", "f": "find", "o": "open", "fo": "focus",
	"ps": "profiles", "p": "profile", "h": "help", "w": "windows", "cl": "close", "m": "move", "mv": "move-domain", "md": "move-domain",
	"nav": "navigate", "la": "layout", "e": "exec", "v": "version", "-v": "version",
}

var arities = map[string]int{
	"dedupe": 0, "domains": 0, "help": 0, "version": 0, "tabs": 0, "links": 0, "count": 0, "current": 0, "find": 1, "focus": 1, "profiles": 0,
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

func keyword(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("keyword must not be empty")
	}
	for _, r := range value {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return errors.New("keyword must not contain control characters")
		}
	}
	return nil
}

func simpleKeyword(value string) bool {
	alnum := false
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			alnum = true
		} else if r != '_' && r != '-' {
			return false
		}
	}
	return alnum
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
	apply, dryRun := false, false
	explicitKeyword := name == "move-domain" && len(argv) > 1 && (argv[1] == "-k" || argv[1] == "--keyword")
	if explicitKeyword && len(argv) < 3 {
		return c, errors.New("--keyword requires a value")
	}
	literalAt := len(argv)
	normalized := []string{argv[0]}
	for i := 1; i < len(argv); i++ {
		value := argv[i]
		if value == "--" {
			literalAt = len(normalized)
			normalized = append(normalized, argv[i+1:]...)
			break
		}
		if value == "-h" || value == "--help" {
			return Command{Name: "help", Args: []string{name}}, nil
		}
		normalized = append(normalized, value)
		valued := !isRow(name) && (value == "-w" || value == "--window")
		if isFiltered(name) || name == "profiles" {
			valued = valued || value == "-p" || value == "--profile" || value == "-D" || value == "--domain" || value == "-k" || value == "--keyword"
		}
		if valued && i+1 < len(argv) {
			i++
			normalized = append(normalized, argv[i])
		}
	}
	if explicitKeyword {
		normalized = append(normalized[:1], normalized[2:]...)
		literalAt--
	}
	argv = normalized
	if len(argv)-1 < arity {
		if name == "move-domain" && len(argv) > 1 {
			return c, errors.New("missing target profile: use dia mv skills.sh GitHub; -p selects the source profile")
		}
		return c, fmt.Errorf("usage: dia %s requires %d argument(s)", name, arity)
	}
	if name == "move-domain" && len(argv) > 2 && literalAt > 2 {
		switch argv[2] {
		case "-p", "--profile", "-w", "--window", "-D", "--domain", "-k", "--keyword", "--pinned", "--loading", "-n", "--dry-run", "-E", "--exact-domain":
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
	if len(args) > 0 && literalAt < len(argv) {
		return c, fmt.Errorf("unexpected literal argument: %s", argv[max(literalAt, 1+arity)])
	}
	for len(args) > 0 {
		flag := args[0]
		args = args[1:]
		switch flag {
		case "--window", "-w", "--profile", "-p", "--domain", "-D", "--keyword", "-k":
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
			case "--keyword", "-k":
				if !(isFiltered(name) || name == "profiles") {
					return c, errors.New("--keyword is not valid here")
				}
				if err := keyword(value); err != nil {
					return c, err
				}
				c.Keywords = append(c.Keywords, value)
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
		case "--subdomains":
			if name != "domains" {
				return c, errors.New("--subdomains is valid only for domains")
			}
			c.Subdomains = true
		case "-s", "-st", "-sl":
			if name == "domains" && flag == "-s" {
				c.Subdomains = true
				break
			}
			if name != "tabs" {
				return c, errors.New("sort flags are valid only for tabs")
			}
			if flag == "-sl" {
				c.Sort = "url"
			} else {
				c.Sort = "title"
			}
		case "--dry-run", "-n":
			if name != "move-domain" && name != "dedupe" {
				return c, errors.New("--dry-run is valid only for move-domain or dedupe")
			}
			c.DryRun = true
			dryRun = true
		case "--apply", "-a":
			if name != "dedupe" {
				return c, errors.New("--apply is valid only for dedupe")
			}
			apply = true
		case "--counts", "-c":
			if name != "profiles" {
				return c, errors.New("--counts is valid only for profiles")
			}
			c.Counts = true
		default:
			return c, fmt.Errorf("unexpected argument: %s", flag)
		}
	}
	if name == "dedupe" {
		if apply && dryRun {
			return c, errors.New("--apply and --dry-run cannot be combined")
		}
		c.DryRun = !apply
	}
	if c.Sort == "url" && !c.Detail {
		return c, errors.New("link sorting (-sl) requires -d (or -detail)")
	}
	if name == "profiles" && (len(c.Domains) > 0 || len(c.Keywords) > 0) && !c.Counts {
		return c, errors.New("profiles --domain/--keyword requires --counts")
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
		if explicitKeyword {
			if err := keyword(c.Args[0]); err != nil {
				return c, err
			}
			c.Keywords = append([]string{c.Args[0]}, c.Keywords...)
		} else if err != nil {
			if !simpleKeyword(c.Args[0]) {
				return c, err
			}
			c.Keywords = append([]string{c.Args[0]}, c.Keywords...)
		} else {
			c.Domains = append([]DomainPattern{p}, c.Domains...)
		}
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
	return name == "dedupe" || name == "domains" || name == "tabs" || name == "find" || name == "count" || name == "links" || name == "move-domain"
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
	if c.Name == "dedupe" {
		if err := result.Dedupe.validate(c.DryRun, result.Error); err != nil {
			return Result{}, err
		}
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
	case "scan", "profile", "preflight", "plan", "moving", "moved", "closing", "closed", "failure", "done":
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
		(&presentation{errs: errs, mode: mode}).fail(err.Error())
		return 2
	}
	if c.Name == "help" {
		if len(c.Args) == 0 {
			fmt.Fprintln(out, help)
		} else {
			fmt.Fprintln(out, commandHelp[c.Args[0]]+"\n\nHELP\n  -h, --help              Show this command help\n  --                      Treat remaining positional arguments literally")
		}
		return 0
	}
	if c.Name == "version" {
		fmt.Fprintln(out, "dia", version)
		return 0
	}
	if runtime.GOOS != "darwin" {
		(&presentation{errs: errs, mode: mode}).fail("dia-cli requires macOS.")
		return 1
	}
	if cache == nil && (isRow(c.Name) || mode.stdoutTTY && (c.Name == "tabs" || c.Name == "find")) {
		cache = newRowCache()
	}
	p := presentation{command: c, out: out, errs: errs, mode: mode}
	if isRow(c.Name) {
		ref, err := cache.lookup(c.Args[0])
		if err != nil {
			(&presentation{errs: errs, mode: mode}).fail(err.Error())
			return 1
		}
		c.Name = "_" + c.Name
		c.Args = append([]string{ref.Window, ref.Tab}, c.Args[1:]...)
	}
	p.start()
	var onProgress func(Progress)
	if (c.Name == "move-domain" || c.Name == "dedupe") && mode.stderrTTY {
		c.Progress = true
		onProgress = p.progress
	}
	result, err := invoke(c, onProgress)
	p.clear()
	if err != nil {
		p.fail(err.Error())
		return 1
	}
	if c.Name == "dedupe" {
		if err := result.Dedupe.validate(c.DryRun, result.Error); err != nil {
			p.fail(err.Error())
			return 1
		}
	}
	if (c.Name == "tabs" || c.Name == "find") && mode.stdoutTTY && result.Error == "" {
		if err := cache.save(result.Tabs); err != nil {
			p.fail(err.Error())
			return 1
		}
	}
	if c.Name == "domains" && result.Error == "" {
		result.Output = domainsOutput(domainCounts(result.URLs, c.Subdomains))
	}
	p.finish(result)
	if result.Error != "" {
		return 1
	}
	return 0
}

func main() {
	terminal := os.Getenv("TERM") != "dumb"
	mode := outputMode{stdoutTTY: terminal && isTerminal(os.Stdout), stderrTTY: terminal && isTerminal(os.Stderr)}
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, backend, nil, mode))
}
