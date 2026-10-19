package main

import (
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
}

type TabRef struct {
	Window string `json:"window"`
	Tab    string `json:"tab"`
}

type Command struct {
	Name    string          `json:"name"`
	Args    []string        `json:"args,omitempty"`
	Window  string          `json:"window,omitempty"`
	Profile string          `json:"profile,omitempty"`
	Domains []DomainPattern `json:"domains,omitempty"`
	Target  string          `json:"target,omitempty"`
	Detail  bool            `json:"detail,omitempty"`
	Sort    string          `json:"sort,omitempty"`
	Pinned  bool            `json:"pinned,omitempty"`
	Loading bool            `json:"loading,omitempty"`
	DryRun  bool            `json:"dryRun,omitempty"`
	Counts  bool            `json:"counts,omitempty"`
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
  --domain, -D <domain>   Select HTTP(S) host; repeatable
                         Use *.example.com for subdomains only
  --window, -w <id>       Select one window
  --dry-run, -n           Plan move-domain without moving
  --counts, -c            Show profile membership counts
  -d, -detail             Show full URLs beside tab titles
  -s, -st / -sl           Sort tabs by title / URL (-sl requires -d)
  --pinned, --loading     Require these tab states
  --help, -h              Show help

Rows come from the last visible tabs/find list in the same terminal and shell.
Move-domain never creates profiles or moves between windows.

EXAMPLES
  dia help <command>
  dia <command> --help
  dia l -p Github -D github.com -D '*.github.com'
  dia mv github.com Work -D '*.github.com' -n`

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
		case "-p", "--profile", "-w", "--window", "-D", "--domain", "--pinned", "--loading", "-n", "--dry-run":
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
	return c, nil
}

func isFiltered(name string) bool {
	return name == "tabs" || name == "find" || name == "count" || name == "links" || name == "move-domain"
}

func isRow(name string) bool {
	return name == "open" || name == "close" || name == "move" || name == "navigate" || name == "exec"
}

func backend(c Command) (Result, error) {
	source, err := scripts.ReadFile("scripts/dia.js")
	if err != nil {
		return Result{}, err
	}
	request, err := json.Marshal(c)
	if err != nil {
		return Result{}, err
	}
	cmd := exec.Command("osascript", "-l", "JavaScript", "-e", string(source), string(request))
	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.TrimSpace(string(output)) == "" {
			return Result{}, err
		}
		return Result{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	var result Result
	if err := json.Unmarshal(output, &result); err != nil {
		return Result{}, fmt.Errorf("invalid Dia response: %w", err)
	}
	return result, nil
}

func run(argv []string, out, errs io.Writer, invoke func(Command) (Result, error), cache *rowCache, interactive bool) int {
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
	if cache == nil && (isRow(c.Name) || interactive && (c.Name == "tabs" || c.Name == "find")) {
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
	result, err := invoke(c)
	if err != nil {
		fmt.Fprintln(errs, err)
		return 1
	}
	if (c.Name == "tabs" || c.Name == "find") && interactive && result.Error == "" {
		if err := cache.save(result.Tabs); err != nil {
			fmt.Fprintln(errs, err)
			return 1
		}
	}
	if result.Move != nil {
		if result.Output != "" {
			fmt.Fprintln(out, result.Output)
		}
		prefix := ""
		if c.DryRun {
			prefix = "dry-run "
		}
		fmt.Fprintf(out, "%smatched=%d moved=%d skipped=%d failed=%d remaining=%d\n", prefix, result.Move.Matched, result.Move.Moved, result.Move.Skipped, result.Move.Failed, result.Move.Remaining)
	} else if interactive && (c.Name == "tabs" || c.Name == "find" || c.Name == "current") {
		fmt.Fprint(out, renderTabs(result.Output, c.Detail || c.Name == "find" || c.Name == "current"))
	} else if interactive && c.Name == "profiles" {
		fmt.Fprint(out, renderProfiles(result.Output))
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
	interactive := isTerminal(os.Stdout) && os.Getenv("TERM") != "dumb"
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, backend, nil, interactive))
}
