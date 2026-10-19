# Dia CLI

A standalone Go command for listing and controlling Dia tabs through macOS Apple Events. The Go binary embeds its JXA backend, so it does not need the source tree at runtime. An optional Zsh plugin calls the same binary. The Homebrew package contains prebuilt Apple Silicon and Intel binaries with no Go, Node.js, or Python runtime dependency.

## Install

Requires macOS and a running Dia with scripting support. Homebrew selects the matching Apple Silicon or Intel archive automatically.

```zsh
brew install jermic/tap/dia-cli
dia --version
dia help
```

Homebrew installs Zsh completion for commands, aliases, valid flags, layouts, and Profile names. Open a new Zsh session after installation, or run `autoload -Uz compinit; compinit` in an existing session. Zsh must include Homebrew's `share/zsh/site-functions` directory in `fpath`. Completing a Profile name runs the read-only `dia profiles` command, so Dia must be running and macOS may ask for Automation permission. If Dia cannot be queried, completion offers no Profile names.

Upgrade with `brew upgrade jermic/tap/dia-cli`. Uninstall with `brew uninstall dia-cli`. Dia does not launch automatically. macOS may ask permission for your terminal to control Dia under **System Settings → Privacy & Security → Automation**.

If you previously used the Oh My Zsh plugin, remove only `dia-cli` from the existing `plugins=(...)` array in `~/.zshrc`, leaving its other entries in place. In an already open Zsh session, run `unfunction dia` if it loaded the old function, or open a new terminal. Check `command -v dia` and `dia --version` to confirm the Homebrew command is active. The old plugin directory can stay in place.

### Install a release archive manually

Download the archive for your Mac from the [latest GitHub Release](https://github.com/Jermic/dia-cli/releases/latest): `darwin_arm64` for Apple Silicon or `darwin_amd64` for Intel. Download `checksums.txt` from the same release, then verify and install the selected archive. For example, after downloading version 0.3.0 for Apple Silicon:

```zsh
grep '  dia-cli_0.3.0_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c - &&
tar -xzf dia-cli_0.3.0_darwin_arm64.tar.gz &&
mkdir -p "$HOME/.local/bin" &&
install -m 0755 dia "$HOME/.local/bin/dia"
```

Use the `darwin_amd64` filename on Intel, update the version for newer releases, and ensure `~/.local/bin` is on your `PATH`.

### Build from source

Requires Go 1.23 or newer. The generated `bin/dia` is ignored by Git.

```zsh
git clone https://github.com/Jermic/dia-cli.git
cd dia-cli
go build -o bin/dia .
./bin/dia help
```

Use `bin/dia` directly, or copy the built binary into a directory on your `PATH`. The binary contains the JXA script. Rebuild after updating the repository. To use completion from this checkout, add its `completions` directory to `fpath` before `compinit` and put `bin/dia` on `PATH`. The completion queries the `dia` binary for Profile names.

For Oh My Zsh, clone the repository into `${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/dia-cli`, build it there, add `dia-cli` to your existing `plugins=(...)` array, and reload Zsh. You can also source `dia-cli.plugin.zsh` directly. The plugin defines a `dia` function that forwards arguments to `bin/dia`; if the binary is missing it prints the build command. It does not change shell configuration automatically.

## Commands

Every command has a short form. Both forms accept the same arguments and flags.

| Command | Short form | Purpose |
| --- | --- | --- |
| `tabs` | `t` | List tabs across windows and profiles. |
| `find` | `f` | Search tab titles and URLs. |
| `current` | `c` | Show the active tab. |
| `count` | `n` | Count distinct window and tab pairs. |
| `domains` | `ds` | Count tabs by root domain, or by full hostname with `-s`. |
| `links` | `l` | Print full URLs, one per line. |
| `windows` | `w` | List window IDs, active profiles, and layouts. |
| `profiles` | `ps` | List profiles, with optional tab counts. |
| `profile` | `p` | Focus a profile by its exact name. |
| `focus` | `fo` | Focus a tab by its ID. |
| `open` | `o` | Focus a saved row. |
| `close` | `cl` | Close a saved row. |
| `move` | `m` | Move a saved row to a profile. |
| `move-domain` | `mv` | Move tabs selected by domains or URL keywords. |
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
dia help mv                     # Show batch move usage and flags
```

Use `dia help <command>` or `dia <command> -h` for command help. `-h` and `--help` before `--` show help, even when a positional argument is missing. A flag that takes a value consumes that value literally, so `-p --help` selects a profile named `--help`. To pass a literal positional argument named `-h` or `--help`, put it after `--`, as in `dia find -- -h`. JavaScript and quoted text that contain those strings stay literal.

Use `--window ID` or `-w ID` to scope a query or explicit ID/name action. Saved row actions always use the window ID from the saved list and reject `-w`. `tabs` accepts `-d`/`-detail` for full URLs, `-s`/`-st` for title order, and `-sl` for URL order; URL order requires `-d`. The last sort flag wins. `--pinned` and `--loading` can filter tabs, find, count, links, domains, and move-domain.

Use `--profile NAME` or `-p NAME` to select a source profile by its exact, case-sensitive name. A requested profile that does not exist is an error. Queries skip other profiles before reading their tabs. Batch moves still inspect profile membership to detect tabs that already belong to the target.

Use `--domain DOMAIN` or `-D DOMAIN` repeatedly to match HTTP(S) hosts. A plain domain matches the host and every subdomain level. Add `--exact-domain` or `-E` to match plain domains exactly, regardless of flag order. `*.example.com` matches subdomains but excludes `example.com`, even with `-E`. Domain matching ignores the URL path and port and folds host case. Domain arguments are DNS names, not URLs, paths, ports, or arbitrary globs.

Use `--keyword TEXT` or `-k TEXT` repeatedly to match text anywhere in a full URL. Keywords are case-insensitive literal text, not title searches, globs, or regular expressions. A keyword can match the host, path, or query. For example, `-k youtube` also matches `https://example.com/?next=youtube`.

Domains and keywords form one union. Profile, state, and `find` filters intersect with that union. These filters are available on `tabs`, `find`, `count`, `links`, `domains`, `move-domain`, and `profiles -c`. Domain and keyword filters on `profiles` require `-c`. `-E` requires at least one domain and has no effect on keywords.

### Domain counts

`dia domains` (or `dia ds`) counts tabs by registrable root domain. `news.bbc.co.uk` groups under `bbc.co.uk`. Private Public Suffix List rules keep `alice.github.io` and `bob.github.io` separate. The list is bundled in the binary and needs no runtime network access. Root grouping uses ASCII/punycode hostnames; raw Unicode hostnames are not IDNA-normalized. `-s` or `--subdomains` groups by full hostname instead.

```bash
dia ds
dia ds -s
dia ds -p Work -D github.com
dia ds -D github.com -s
dia ds -k youtube
```

Output has `DOMAIN` and `COUNT` columns, sorted by count descending and then domain alphabetically. Each distinct window and tab pair counts once, even if several tabs have the same URL. Hostnames ignore case, ports, and a trailing dot. IP addresses and `localhost` stay separate. Empty, invalid, and hostless URLs group under `(no domain)`, so totals match `dia count` with the same filters. Domain counts do not replace saved rows. Redirected output is TSV.

## Batch moving

```zsh
dia move-domain example.com Work -p Personal -n
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

Each successful visible `tabs` or `find` replaces a row cache for that terminal and parent shell. The cache stores only window and tab IDs in a mode `0600` temporary file, written atomically; its key includes the parent shell's terminal, process ID, and start time. Empty visible results clear it. Failed listings, pipes, redirected output, `TERM=dumb`, `links`, `current`, and `domains` do not replace it. Row actions resolve those saved IDs directly, including tabs hidden in inactive profiles, even when action input and output are redirected. Refresh the list after closing or moving tabs or restarting Dia. Row numbers are local to the terminal and shell that listed them.

`navigate` accepts only absolute HTTP(S) URLs. `exec` runs one quoted JavaScript argument in a tab using the logged-in Dia session. Dia may require its `--enable-applescript-javascript` startup flag for `exec`; this CLI does not change that browser setting.

## Development

```zsh
go test ./...
go vet ./...
go build -o bin/dia .
zsh tests/smoke.zsh
```

The smoke suite also uses Python 3 for PTY display checks. Tests use fake Dia objects and never write to real browser tabs. A live read-only check can use `dia count` or `dia profiles`. Listing reads properties in bulk and checks tab ID order before and after those reads. A detected change asks you to retry. The reads are not an atomic browser snapshot. No URLs or titles are saved to the row cache.

## Release maintenance

For each stable `vX.Y.Z` release, commit the source changes, tag that exact commit, and push the commit and tag. CI caches Go dependencies and build outputs using `go.mod`. A main-branch run skips its duplicate test matrix only when a remote stable version tag points to that same commit. Tag runs always test natively on Apple Silicon and Intel.

Each tag test job builds a versioned `bin/dia`, runs the smoke suite against that binary, checks its version, and uploads it. The release job downloads both binaries from that same workflow run and packages them without rebuilding. `go run ./cmd/release --binaries-dir dist/binaries vX.Y.Z` reads `dist/binaries/arm64/dia` and `dist/binaries/amd64/dia`, validates their Mach-O architectures, and produces the two archives, `checksums.txt`, and `dia-cli.rb`. To build both binaries locally instead, use `go run ./cmd/release vX.Y.Z`.

The release helper requires a clean tagged source tree and refuses an existing `dist/vX.Y.Z` directory. The tag workflow creates a draft GitHub Release with all four assets, then publishes it. An existing Release is not replaced.

After the Release is public, download all four assets with `gh release download vX.Y.Z --repo Jermic/dia-cli --dir release-files`, then run `cd release-files && shasum -a 256 -c checksums.txt` to verify both archives. In a checkout of the separate `Jermic/homebrew-tap` repository, replace `Formula/dia-cli.rb` with the downloaded `release-files/dia-cli.rb`, inspect the version, two architecture URLs, and their separate SHA-256 values, then commit and push that tap change. The tap is maintained separately; this repository's workflow does not write to it.

## License

MIT
