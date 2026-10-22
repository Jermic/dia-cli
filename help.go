package main

var commandHelp = map[string]string{
	"dedupe": `USAGE
  dia dedupe [flags]

ALIASES
  dd

Preview duplicate HTTP(S) tabs by exact, nonempty full URL within each window and Profile. Default and -n never close tabs. Add -a to close only safe extra copies. This command does not replace saved rows.

FLAGS
  -a, --apply            Close safe duplicate copies
  -n, --dry-run          Preview only (default); cannot combine with -a
  -w, --window <id>      Select one window
  -p, --profile <name>   Select an exact, case-sensitive source Profile
  -D, --domain <domain>  Match HTTP(S) root and subdomains; repeat for a union
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading    Require these tab states; protected copies never close

Domains and keywords form a union. Excluded domains veto matches. Other filters intersect. Quote '*.example.com' for subdomains only. URLs are compared literally, preserving query, fragment, case, and trailing slash. Distinct windows and Profile instances never merge. A tab shared across Profiles, pinned, or active in its window is protected. A loading copy protects its entire duplicate group. Keep a protected nonloading copy when available, otherwise the first copy, plus every additional protected copy.

Every group passes preflight before any close. Each close rechecks membership, URL, protected states, and the group's loading state. Changes or errors stop immediately without retrying. Partial results retain completed closes. Separate browser reads and writes are not atomic. Preview lists KEEP/CLOSE/SKIP decisions, and terminal output hides internal IDs. Piped TSV includes window and tab IDs. Failed copies remain included in remaining.

EXAMPLES
  dia dd
  dia dd -p Work -D github.com -n
  dia dd -p Work -k youtube -a`,

	"domains": `USAGE
  dia domains [flags]

ALIASES
  ds

Count tabs by registrable root domain, including private Public Suffix List rules. Use -s to count each full hostname separately. Counts descend, with alphabetical domain order for ties. Duplicate URLs in different tabs count separately.

FLAGS
  -s, --subdomains        Count each full hostname separately
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  -k, --keyword <text>    Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain      Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states

Domains and URL keywords form a union. Excluded domains veto matches. Other filters intersect. Plain example.com includes subdomains; quote '*.example.com' for subdomains only. -E matches plain domains exactly. IP addresses and localhost stay separate. Empty, invalid, and hostless URLs count as (no domain). All tab counts are retained. This command does not replace saved rows.

EXAMPLES
  dia ds
  dia ds -s -D github.com
  dia ds -p Work -k youtube`,

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
  v, -v, --version

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
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states
  -d, -detail             Show full URLs beside titles
  -s, -st                 Sort by title
  -sl                     Sort by URL; requires -d

Domains and URL keywords form a union. Excluded domains veto matches. Keywords match literal, case-insensitive text in the full URL, including paths and queries. Other filters intersect with this union. Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

EXAMPLES
  dia tabs -d -sl
  dia tabs -p Github -D github.com`,
	"find": `USAGE
  dia find <text> [flags]

ALIASES
  f

Search tab titles and URLs with literal, case-insensitive text. A visible terminal list saves row IDs for later row commands.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Match an HTTP(S) host; repeat for a union
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states

Domains and URL keywords form a union. Excluded domains veto matches. Keywords match literal, case-insensitive text in the full URL, including paths and queries. Other filters intersect with this union. Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

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
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states

Domains and URL keywords form a union. Excluded domains veto matches. Keywords match literal, case-insensitive text in the full URL, including paths and queries. Other filters intersect with this union. Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

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
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states

Domains and URL keywords form a union. Excluded domains veto matches. Keywords match literal, case-insensitive text in the full URL, including paths and queries. Other filters intersect with this union. Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

EXAMPLE
  dia l -p Github -D github.com`,
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
  dia move-domain <selector> <target-profile> [flags]
  dia move-domain -k <text> <target-profile> [flags]
  dia move-domain <target-profile> <tab-filter> [flags]

ALIASES
  mv (recommended), md (compatible)

Move tabs selected by filters into the target profile. The target-only form requires --domain, --keyword, --exclude-domain, --pinned, --unpinned, or --loading; -p, -w, and -n alone are insufficient. A simple word such as youtube matches literal, case-insensitive text in the full URL. Use -D for a strict domain or -k for arbitrary URL text. The target must already exist in each involved window. Tabs stay in their windows.

All windows pass preflight before any move. Terminal stderr shows live progress. Terminal stdout shows aligned previews and a concise result. Full counts appear on failure. Piped stdout keeps TSV plans and key=value counts. Dry-run shows scan and preview progress without moving tabs.

FLAGS
  -w, --window <id>       Select one window
  -p, --profile <name>    Select an exact, case-sensitive source profile
  -D, --domain <domain>   Add an HTTP(S) host to the domain union
  -k, --keyword <text>   Match URL text, case-insensitive; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain domains exactly; requires a domain
  -up, --unpinned         Exclude pinned tabs; cannot combine with --pinned
  --pinned, --loading     Require these tab states
  -n, --dry-run           Preview decisions and counts; move no tabs

Domains and URL keywords form a union. Excluded domains veto matches. Keywords match literal, case-insensitive text in the full URL, including paths and queries. Other filters intersect with this union. Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

EXAMPLE
  dia mv github.com Work -p Personal -n
  dia mv youtube Watch -n
  dia mv -k "youtube.com/watch" Watch -n
  dia mv skills.sh Watch -k youtube -n
  dia mv Read -up -exd x.com -n`,
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
  -k, --keyword <text>   Limit counts to URL text; requires -c; repeat for a union
  -exd, --exclude-domain <domain>  Exclude a host and subdomains; repeatable
  -E, --exact-domain     Match plain included/excluded domains exactly; requires -c

Plain included or excluded example.com matches the root and all subdomain levels. Use -E for only the plain host. Quote '*.example.com' for subdomains only; -E leaves wildcards unchanged.

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
