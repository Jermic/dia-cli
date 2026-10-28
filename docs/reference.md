# Command reference

[Back to README](../README.md)

- [Commands](#commands)
- [Filters and arguments](#filters-and-arguments)
- [History and bookmarks](#search-history-and-bookmarks)
- [Domain counts](#domain-counts)
- [Duplicate tabs](#duplicate-tabs)
- [Batch moving](#batch-moving)
- [Output and saved rows](#output-and-saved-rows)

## Commands

Every command has a short form. Both forms accept the same arguments and flags.

| Command | Short form | Purpose |
| --- | --- | --- |
| `tabs` | `t` | List tabs across windows and profiles. |
| `find` | `f` | Search tab titles and URLs. |
| `history` | `hs` | Search local history, latest 100 matching URLs. |
| `bookmarks` | `bm` | Search local bookmarks, including nested folders. |
| `current` | `c` | Show the active tab. |
| `count` | `n` | Count distinct window and tab pairs. |
| `dedupe` | `dd` | Preview duplicate tabs; add `-a` to close safe copies. |
| `domains` | `ds` | Count tabs by root domain, or by full hostname with `-s`. |
| `links` | `l` | Print full URLs, one per line. |
| `windows` | `w` | List window IDs, active profiles, and layouts. |
| `profiles` | `ps` | List profiles, with optional tab counts. |
| `profile` | `p` | Focus a profile by its exact name. |
| `focus` | `fo` | Focus a tab by its ID. |
| `open` | `o` | Focus a saved row. |
| `close` | `cl` | Close a saved row. |
| `move` | `m` | Move a saved row to a profile. |
| `move-domain` | `mv` | Move tabs selected by domains, URL keywords, and tab filters. |
| `navigate` | `nav` | Open an HTTP(S) URL in a saved row. |
| `exec` | `e` | Run JavaScript in a saved row. |
| `layout` | `la` | Set the window layout to `sidebar` or `tabStrip`. |
| `help` | `h` | Show root or command help. |
| `version` | `v` | Show the version without connecting to Dia. |

The older `md` alias for `move-domain` still works. `dia version`, `dia v`, `dia -v`, and `dia --version` are equivalent.

```zsh
dia t -d -sl                    # Show full URLs, sorted by URL
dia f github                    # Search titles and URLs
dia l -p GitHub -D github.com    # URLs from one profile and domain
dia ps -c -k youtube            # Count matching URLs in each profile
dia o 2                         # Focus row 2 from the last tabs/find list
dia m 2 Work                    # Move saved row 2 to Work
dia mv youtube Watch -n         # Preview a batch move by URL keyword
dia mv Read -up -exd x.com -n    # Preview unpinned tabs outside x.com
dia help mv                     # Show batch move usage and flags
```

## Filters and arguments

Use `dia help <command>` or `dia <command> -h` for command help. `-h` and `--help` before `--` show help, even when a positional argument is missing. A flag that takes a value consumes that value literally, so `-p --help` selects a profile named `--help`. To pass a literal positional argument named `-h` or `--help`, put it after `--`, as in `dia find -- -h`. JavaScript and quoted text that contain those strings stay literal.

Use `--window ID` or `-w ID` to scope a query or explicit ID/name action. Saved row actions always use the window ID from the saved list and reject `-w`. `tabs` accepts `-d`/`-detail` for full URLs, `-s`/`-st` for title order, and `-sl` for URL order; URL order requires `-d`. The last sort flag wins. `--pinned`, `--unpinned` (or `-up`), and `--loading` can filter tabs, find, count, links, domains, dedupe, move-domain, and profiles with `-c`.

Queries include pinned tabs by default. `move-domain`, `mv`, and `md` exclude pinned tabs by default. Explicit `--unpinned` or `-up` has the same effect for batch moves. `--pinned` selects only pinned tabs. `--include-pinned` or `-a` selects both states for batch moves, preserving every other filter. It cannot combine with `--pinned` or `--unpinned`. Other commands reject `--include-pinned`. On `dedupe`, `-a` retains its `--apply` meaning, and pinned tabs remain protected. `move` and `m` act on an explicitly selected saved row and can move a pinned tab.

Use `--profile NAME` or `-p NAME` to select a source profile by its exact, case-sensitive name. A requested profile that does not exist is an error. Queries skip other profiles before reading their tabs. Batch moves and dedupe still inspect all profile memberships to detect tabs that already belong to the target.

Use `--domain DOMAIN` or `-D DOMAIN` repeatedly to match HTTP(S) hosts. A plain domain matches the host and every subdomain level. Add `--exact-domain` or `-E` to match plain domains exactly, regardless of flag order. `*.example.com` matches subdomains but excludes `example.com`, even with `-E`. Domain matching ignores the URL path and port and folds host case. Domain arguments are DNS names, not URLs, paths, ports, or arbitrary globs.

Use `--exclude-domain DOMAIN` or `-exd DOMAIN` repeatedly to exclude HTTP(S) hosts. Plain `x.com` excludes the root and all subdomains; `'*.x.com'` excludes only subdomains. Exclusions veto positive domain or keyword matches. `-E` applies to included and excluded plain domains. Non-HTTP(S) URLs remain eligible for the other filters. `--unpinned` excludes fixed tabs and cannot combine with `--pinned`.

Use `--keyword TEXT` or `-k TEXT` repeatedly to match text anywhere in a full URL. Keywords are case-insensitive literal text, not title searches, globs, or regular expressions. A keyword can match the host, path, or query. For example, `-k youtube` also matches `https://example.com/?next=youtube`.

Domains and keywords form one union. Profile, state, and `find` filters intersect with that union. Domain and keyword filters are available on `tabs`, `find`, `count`, `links`, `domains`, `dedupe`, `move-domain`, and `profiles -c`. Included/excluded domain, keyword, pinned, unpinned, and loading filters on `profiles` require `-c`. State filters intersect with URL filters before counting distinct tab IDs in each Profile. A tab shared across Profiles counts in each Profile, and a Profile with no matches retains a zero count. `-E` requires at least one included or excluded domain and has no effect on keywords.

Without a positive positional selector, put the destination first, such as `dia mv Read --exclude-domain x.com --dry-run`. This form requires an explicit tab-selection filter; `-p`, `-w`, `-n`, the default unpinned selection, and `--include-pinned` do not qualify by themselves. Legacy `dia mv github.com GitHub` and `dia mv -k youtube Watch` still work, now excluding pinned tabs by default. Add `-a` to restore selection of both states. Preview with `-n` before removing it to apply. An unpinned move rechecks pin state during preflight and immediately before each move; a newly pinned candidate stops the batch.

## Search history and bookmarks

```zsh
dia hs github
dia bm "reading list"
dia history --json github
dia bookmarks github --json
dia hs -- -literal
```

Both commands require one nonempty search argument. Quote phrases with spaces. Search matches literal text in titles or URLs, ignoring case, with no wildcard or regular expression syntax. `--json` can appear before or after the query, before `--`. JSON output is an array with original source strings, including `[]` for no matches. Terminal output uses aligned tables; redirected output is TSV with control characters replaced by spaces. These searches do not replace saved tab rows.

The source is `~/Library/Application Support/Dia/User Data/<disk-profile>/`. `Local State` selects `profile.last_used`; a missing state file or unset value selects `Default`. A disk profile is not a Dia sidebar Profile such as Day or GitHub. Tab filters such as `--profile`, `--domain`, and `--window` are not supported by these commands.

History uses the macOS system `/usr/bin/sqlite3` in read-only mode and includes committed WAL data. It returns at most 100 matching URL records, ordered by last visit and then ID descending. `lastVisitedAt` is UTC, or empty when no visit time is recorded. Search scans the history records before applying the result cap, so large histories can take longer. History reads have a 10-second timeout. Bookmarks read the `Bookmarks` JSON file, traverse roots by key and children in stored order, and include the folder path. Duplicate bookmark URLs stay separate. Missing or malformed sources are errors. Neither command needs Apple Events permission or adds a runtime dependency beyond macOS system tools.

## Domain counts

`dia domains` (or `dia ds`) counts tabs by registrable root domain. `news.bbc.co.uk` groups under `bbc.co.uk`. Private Public Suffix List rules keep `alice.github.io` and `bob.github.io` separate. The list is bundled in the binary and needs no runtime network access. Root grouping uses ASCII/punycode hostnames; raw Unicode hostnames are not IDNA-normalized. `-s` or `--subdomains` groups by full hostname instead.

```bash
dia ds
dia ds -s
dia ds -p Work -D github.com
dia ds -D github.com -s
dia ds -k youtube
```

Output has `DOMAIN` and `COUNT` columns, sorted by count descending and then domain alphabetically. Each distinct window and tab pair counts once, even if several tabs have the same URL. Hostnames ignore case, ports, and a trailing dot. IP addresses and `localhost` stay separate. Empty, invalid, and hostless URLs group under `(no domain)`, so totals match `dia count` with the same filters. Domain counts do not replace saved rows. Redirected output is TSV.

## Duplicate tabs

`dia dedupe` or `dia dd` previews duplicate tabs. Nothing closes until you add `-a` or `--apply`. `-n` or `--dry-run` explicitly requests the default preview and cannot be combined with `-a`.

```bash
dia dd                         # Preview all Profiles
dia dd -p Work -D github.com -n  # Preview with existing filters
dia dd -p Work -k youtube -a    # Close safe duplicates from this selection
```

Only valid, nonempty HTTP(S) URLs that are exactly equal belong to a duplicate group. Query parameters, fragments, URL case, and trailing slashes remain significant. Groups stay within one window and one Profile instance. Profiles with identical names do not merge. The usual Profile, window, domain, keyword, pinned, and loading filters apply before grouping.

Keep one protected, nonloading copy when available, otherwise the first copy in browser order. Every pinned tab, window-active tab, and tab shared across Profiles is protected, including memberships outside `-p`. Additional protected copies remain open. If any copy in the group is loading, skip the entire group. Shared duplicate copies count once per physical window and tab pair.

The preview lists `KEEP`, `CLOSE`, and `SKIP` with Profile, URL, and reason. Terminal output hides window and tab IDs. Redirected output is TSV and retains both IDs for auditing. Counts report duplicate groups, extra copies, closed, skipped, failed, and remaining. `duplicates = closed + skipped + remaining`; failed copies are included in remaining. Preview reports how many tabs would close and confirms no changes were made.

All groups pass preflight before the first close. Before each close, the backend checks the candidate and keeper IDs and URLs, full Profile membership, pinned and active protection, and whether any still-live group member is loading. A state change or browser error stops the batch immediately, reports exact partial counts and the original reason, and exits nonzero without retrying. Completed closes remain closed. Separate Apple Events cannot provide an atomic transaction, so inspect the preview and avoid editing the selected tabs during an apply. A later rerun recomputes from current browser state. Dedupe uses one JXA process and never writes URLs or replaces the saved-row cache.

## Batch moving

```zsh
dia move-domain example.com Work -p Personal -n
dia mv example.com Work -a -n                     # Include pinned tabs in the preview
dia mv youtube Watch -n                          # Match youtube anywhere in URLs
dia mv -k 'youtube.com/watch' Watch -n             # Treat the first selector as a URL keyword
dia mv example.com Work -D '*.example.org' -k docs # Match either domain or keyword
dia mv example.com Work -E -n                     # Preview only the exact host
```

`move-domain` treats its first argument as a domain when it is a valid domain. A simple word without dots, such as `youtube`, is a URL keyword. A malformed dotted domain stays an error. Put `-k` before the first selector to force a URL keyword, as in `dia mv -k 'youtube.com/watch' Watch -n`. This form also accepts keywords with dots, spaces, or punctuation and needs no additional selector. Explicit `-D` arguments always require valid domains.

The command combines its first selector with every `-D` and `-k`, then gathers matching tabs in one JXA call. It validates that each involved window contains exactly one target profile before moving anything. It skips a tab already in that window's target profile, even when the tab also belongs to another profile. It never creates profiles or moves tabs between windows.

The backend checks tab ID order around bulk property reads. Before any move, it rechecks all candidates by ID and URL, then checks each tab again immediately before moving it. A detected change stops the operation without retrying. A change found during the initial checks produces no moves. A change found after moves have started produces a partial result. These checks cannot make separate browser reads and writes atomic.

`--dry-run` or `-n` performs the same read and preflight with zero moves. The terminal preview lists each matching tab's source profile, target profile, URL, and `MOVE` or `SKIP` decision, hiding internal window and tab IDs. Piped TSV retains both IDs, and actionable errors preserve IDs needed to locate a failure. The terminal preview ends with how many tabs would move, how many would be skipped, and confirmation that no changes were made. An empty plan says `No matching tabs · no changes made`.

Terminal batch operations show the domain and keyword selectors and the target before the browser request starts. Scans and target checks share one updating `Reading tabs` line. Once every window has passed preflight, the display confirms the number of matches and target readiness. The move counter advances only after successful moves. Success reports moved tabs and any tabs skipped because they already belong to the target, with zero-valued metrics omitted. Partial failure shows all five counts (`matched`, `moved`, `skipped`, `failed`, and `remaining`) and the original error, then exits nonzero. `remaining` includes failed tabs and is not added to `failed`. Completed moves stay completed. Inspect the result before retrying.

Live progress uses terminal stderr, including when stdout is redirected, and is cleared before final results or errors. Redirected stderr omits live progress. Piped stdout retains window IDs, TSV plans, and one-line `key=value` counts. A backend or protocol failure exits nonzero without a fabricated summary or automatic retry.

## Output and saved rows

On an interactive terminal, queries use a shared title, optional filter context, aligned content, and a result-count footer. Empty queries show a message instead of an empty table. `tabs`, `find`, and `current` retain numbered titles and HTTP(S) OSC 8 links. `find` and `current` also show full URLs; `tabs -d` adds them to `tabs`. Profile footers count displayed profile rows, not distinct tabs. Ordinary listings and progress hide window IDs. Explicit `dia windows` still lists them, and actionable errors preserve their original diagnostics.

Simple actions use one success line with the requested row number and target profile when applicable, instead of internal IDs. Raw action output keeps its existing format. Errors use `✕` when stderr is a terminal. `links`, `count`, and `exec` keep their raw payloads even on a terminal; help, version, and shell completion stay undecorated. Redirected listings retain their existing TSV format without color, hyperlinks, or frames. `links` prints only nonempty URLs and retains duplicate URLs from different tabs.

The renderer reads the terminal's actual width, falling back to `COLUMNS` and then 80. Frames are included in the title-width budget, while full URLs remain visible. Chinese characters and combining marks are measured for terminal alignment; complex emoji widths can vary by terminal. Control characters from browser titles and URLs are flattened.

Each successful visible `tabs` or `find` replaces a row cache for that terminal and parent shell. The cache stores only window and tab IDs in a mode `0600` temporary file, written atomically; its key includes the parent shell's terminal, process ID, and start time. Empty visible results clear it. Failed listings, pipes, redirected output, `TERM=dumb`, `links`, `current`, `domains`, and `dedupe` do not replace it. Row actions resolve those saved IDs directly, including tabs hidden in inactive profiles, even when action input and output are redirected. Refresh the list after closing or moving tabs or restarting Dia. Row numbers are local to the terminal and shell that listed them.

`navigate` accepts only absolute HTTP(S) URLs. `exec` runs one quoted JavaScript argument in a tab using the logged-in Dia session. Dia may require its `--enable-applescript-javascript` startup flag for `exec`; this CLI does not change that browser setting.
