package main

var commandHelp = map[string]string{
	"help": `USAGE
  dia help [command]

ALIASES
  h

Show root help or help for one command or alias.

EXAMPLES
  dia help
  dia help mv`,
	"version": `USAGE
  dia version

ALIASES
  --version

Show the installed version without contacting Dia.

EXAMPLE
  dia --version`,
	"tabs": `USAGE
  dia tabs [flags]

ALIASES
  t

List tabs from all windows and profiles, sorted by title. A visible terminal list saves row IDs for later row commands.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  --pinned, --loading     Require these tab states
  -d, -detail             Show full URLs beside titles
  -s, -st                 Sort by title
  -sl                     Sort by URL; requires -d

Other filters intersect with the domain union. Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLES
  dia tabs -d -sl
  dia tabs -p Github -D github.com -D '*.github.com'`,
	"find": `USAGE
  dia find <text> [flags]

ALIASES
  f

Search tab titles and URLs with literal, case-insensitive text. A visible terminal list saves row IDs for later row commands.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  --pinned, --loading     Require these tab states

Other filters intersect with the domain union. Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLE
  dia find github -p Work`,
	"current": `USAGE
  dia current [-w <id>]

ALIASES
  c

Show the active tab of the front or selected window, including its URL.

FLAG
  -w, --window <id>       Select one window

EXAMPLE
  dia current`,
	"count": `USAGE
  dia count [flags]

ALIASES
  n

Count distinct window and tab pairs after filtering.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  --pinned, --loading     Require these tab states

Other filters intersect with the domain union. Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLE
  dia count -D example.com`,
	"links": `USAGE
  dia links [flags]

ALIASES
  l

Print full, nonempty URLs, one per line. Different tabs may have the same URL.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  --pinned, --loading     Require these tab states

Other filters intersect with the domain union. Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLE
  dia l -p Github -D github.com -D '*.github.com'`,
	"open": `USAGE
  dia open <row>

ALIASES
  o

Focus a row from the last visible tabs or find list in this terminal and shell. The saved window ID is used.

EXAMPLE
  dia open 2`,
	"focus": `USAGE
  dia focus <tab-id> [-w <id>]

ALIASES
  fo

Focus an exact tab ID. Use -w when the ID occurs in more than one window.

FLAG
  -w, --window <id>       Select one window

EXAMPLE
  dia focus <tab-id>`,
	"close": `USAGE
  dia close <row>

ALIASES
  cl

Close a row from the last visible tabs or find list in this terminal and shell. The saved window ID is used.

EXAMPLE
  dia close 2`,
	"move": `USAGE
  dia move <row> <profile>

ALIASES
  m

Move a saved row to an exact profile name in the same saved window. The target profile must already exist.

EXAMPLE
  dia move 2 Work`,
	"move-domain": `USAGE
  dia move-domain <domain> <target-profile> [flags]

ALIASES
  mv (recommended), md (compatible)

Move tabs matching the first domain or any repeated -D domain into the target profile. The target must already exist in each involved window. Tabs stay in their windows.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Add an HTTP(S) host to the domain union
  --pinned, --loading     Require these tab states
  -n, --dry-run           Preview decisions and counts; move no tabs

Other filters intersect with the domain union. Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLE
  dia mv github.com Work -D '*.github.com' -p Personal -n`,
	"navigate": `USAGE
  dia navigate <row> <url>

ALIASES
  nav

Navigate a saved row to an absolute HTTP(S) URL. The saved window ID is used.

EXAMPLE
  dia navigate 2 https://example.com`,
	"exec": `USAGE
  dia exec <row> <javascript>

ALIASES
  e

Run one quoted JavaScript argument in a saved row using Dia's logged-in session. The saved window ID is used.

EXAMPLE
  dia exec 2 'document.title'`,
	"windows": `USAGE
  dia windows [-w <id>]

ALIASES
  w

List window IDs, active profile names, and layouts.

FLAG
  -w, --window <id>       Select one window

EXAMPLE
  dia windows`,
	"profiles": `USAGE
  dia profiles [flags]

ALIASES
  ps

List profiles in each window. With -c, count distinct tabs in each profile.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive profile
  -c, --counts            Show profile membership counts
  -D, --domain <domain>   Limit counts to an HTTP(S) host; requires -c; repeat for a union

Quote '*.example.com' to match subdomains only; it excludes example.com.

EXAMPLE
  dia profiles -c -D example.com`,
	"profile": `USAGE
  dia profile <name> [-w <id>]

ALIASES
  p

Focus an exact profile name. Use -w when the name occurs in more than one window.

FLAG
  -w, --window <id>       Select one window

EXAMPLE
  dia profile Work`,
	"layout": `USAGE
  dia layout <sidebar|tabStrip> [-w <id>]

ALIASES
  la

Set the sidebar or tabStrip layout on the front or selected window.

FLAG
  -w, --window <id>       Select one window

EXAMPLE
  dia layout sidebar`,
}
