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

Download the archive for your Mac from the [latest GitHub Release](https://github.com/Jermic/dia-cli/releases/latest): `darwin_arm64` for Apple Silicon or `darwin_amd64` for Intel. Download `checksums.txt` from the same release, then verify and install the selected archive. For example, after downloading version 0.2.0 for Apple Silicon:

```zsh
grep '  dia-cli_0.2.0_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c - &&
tar -xzf dia-cli_0.2.0_darwin_arm64.tar.gz &&
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

```zsh
dia tabs                         # All windows and profiles, sorted by title
dia tabs -d -sl                  # Show URLs and sort by URL
dia tabs --pinned --loading      # Both states required
dia find "github"                # Literal, case-insensitive title/URL search
dia count                        # Number of distinct window/tab pairs
dia links                        # Full URLs, one per line
dia current                      # Active tab of the front or selected window
dia windows                      # Window ID, active profile, and layout
dia profiles -c                  # Profile membership counts
dia profiles -c -D example.com  # Count matching host within each profile
dia profile "Work"               # Focus an exact profile name
dia focus <tab-id>               # Focus an exact tab ID
dia open 2                       # Focus row 2 from the last visible tabs/find list
dia close 2                      # Close saved row 2
dia move 2 "Work"                 # Move saved row 2 within its saved window
dia navigate 2 https://example.com
dia exec 2 'document.title'
dia layout sidebar              # Also accepts tabStrip
dia --version                   # Local version; no Dia connection needed
dia help                        # Root help
dia help tabs                   # Command-specific usage and flags
dia mv --help                   # Help also works through aliases
```

The tab and window commands have short forms: `t` tabs, `f` find, `n` count, `l` links, `c` current, `w` windows, `ps` profiles, `p` profile, `fo` focus, `o` open, `cl` close, `m` move, `nav` navigate, `e` exec, `la` layout, `h` help, and `mv` move-domain. The older `md` alias still works. Short forms accept the same arguments and flags. Use `dia help <command>` or `dia <command> -h` to see that command's flags and examples. `dia version` is equivalent to `dia --version`.

Use `--window ID` or `-w ID` to scope a query or explicit ID/name action. Saved row actions always use the window ID from the saved list and reject `-w`. `tabs` accepts `-d`/`-detail` for full URLs, `-s`/`-st` for title order, and `-sl` for URL order; URL order requires `-d`. The last sort flag wins. `--pinned` and `--loading` can filter tabs, find, count, links, and move-domain.

Use `--profile NAME` or `-p NAME` to select a source profile by exact, case-sensitive name. Use `--domain DOMAIN` or `-D DOMAIN` repeatedly for a union of HTTP(S) host matches. A plain domain matches that host and all subdomain levels. Add `--exact-domain` or `-E` to match plain domains exactly. The flag applies to all plain domains, regardless of flag order. `*.example.com` matches one or more subdomain levels, excluding `example.com` itself, even with `-E`. Domain matching ignores URL path and port and folds host case. Other filters intersect with the domain union. Domain arguments are DNS names, not URLs, paths, ports, or arbitrary globs. `profiles` accepts `-p`, `-w`, and `-D` with `-c`; domain filtering requires `-c`. `-E` requires a domain and is available on tabs, find, count, links, move-domain, and profiles with counts. A requested source profile that does not exist is an error.

## Batch moving

```zsh
dia move-domain example.com "Work" --profile Personal --dry-run
dia mv example.com "Work" -p Personal -w <window-id>
dia mv example.com "Work" -E -n   # Preview only the exact host
```

`move-domain` combines its first domain with every `-D`, then gathers matching tabs once across the selected windows and profiles. It validates that each involved window contains exactly one target profile before moving anything. It skips a tab already in that window's target profile, even when the tab also belongs to another profile. It never creates profiles or moves tabs between windows.

`--dry-run` or `-n` performs the same read and preflight with zero moves. The terminal preview lists each matching tab's source profile, target profile, URL, and `MOVE` or `SKIP` decision, hiding internal window and tab IDs. Piped TSV retains both IDs, and actionable errors preserve IDs needed to locate a failure. The terminal preview ends with how many tabs would move, how many would be skipped, and confirmation that no changes were made. An empty plan says `No matching tabs · no changes made`.

Terminal batch operations show the domains and target before the browser request starts. Scans and target checks share one updating `Reading tabs` line. Once every window has passed preflight, the display confirms the number of matches and target readiness. The move counter advances only after successful moves. Success reports moved tabs and any tabs skipped because they already belong to the target, with zero-valued metrics omitted. Partial failure shows all five counts (`matched`, `moved`, `skipped`, `failed`, and `remaining`) and the original error, then exits nonzero. `remaining` includes failed tabs and is not added to `failed`. Completed moves stay completed. Inspect the result before retrying.

Live progress uses terminal stderr, including when stdout is redirected, and is cleared before final results or errors. Redirected stderr omits live progress. Piped stdout retains window IDs, TSV plans, and one-line `key=value` counts. A backend or protocol failure exits nonzero without a fabricated summary or automatic retry.

## Output and saved rows

On an interactive terminal, queries use a shared title, optional filter context, aligned content, and a result-count footer. Empty queries show a message instead of an empty table. `tabs`, `find`, and `current` retain numbered titles and HTTP(S) OSC 8 links. `find` and `current` also show full URLs; `tabs -d` adds them to `tabs`. Profile footers count displayed profile rows, not distinct tabs. Ordinary listings and progress hide window IDs. Explicit `dia windows` still lists them, and actionable errors preserve their original diagnostics.

Simple actions use one success line. Errors use `✕` when stderr is a terminal. `links`, `count`, and `exec` keep their raw payloads even on a terminal; help, version, and shell completion stay undecorated. Redirected listings retain their existing TSV format without color, hyperlinks, or frames. `links` prints only nonempty URLs and retains duplicate URLs from different tabs.

The renderer reads the terminal's actual width, falling back to `COLUMNS` and then 80. Frames are included in the title-width budget, while full URLs remain visible. Chinese characters and combining marks are measured for terminal alignment; complex emoji widths can vary by terminal. Control characters from browser titles and URLs are flattened.

Each successful visible `tabs` or `find` replaces a row cache for that terminal and parent shell. The cache stores only window and tab IDs in a mode `0600` temporary file, written atomically; its key includes the parent shell's terminal, process ID, and start time. Empty visible results clear it. Failed listings, pipes, redirected output, `TERM=dumb`, `links`, and `current` do not replace it. Row actions resolve those saved IDs directly, including tabs hidden in inactive profiles, even when action input and output are redirected. Refresh the list after closing or moving tabs or restarting Dia. Row numbers are local to the terminal and shell that listed them.

`navigate` accepts only absolute HTTP(S) URLs. `exec` runs one quoted JavaScript argument in a tab using the logged-in Dia session. Dia may require its `--enable-applescript-javascript` startup flag for `exec`; this CLI does not change that browser setting.

## Development

```zsh
go test ./...
go vet ./...
go build -o bin/dia .
zsh tests/smoke.zsh
```

The smoke suite also uses Python 3 for PTY display checks. Tests use fake Dia objects and never write to real browser tabs. A live read-only check can use `dia count` or `dia profiles`. Listing reads properties in bulk but is not an atomic browser snapshot; a detected size change asks you to retry. No URLs or titles are saved to the row cache.

## Release maintenance

For each stable `vX.Y.Z` release, commit the source changes, tag that exact commit, and push the commit and tag. CI tests on both macOS architectures, then `go run ./cmd/release vX.Y.Z` builds the two archives, `checksums.txt`, and `dia-cli.rb` from the clean tagged source. The helper refuses a dirty source tree or an existing `dist/vX.Y.Z` directory. The tag workflow creates a draft GitHub Release with all four assets, then publishes it. An existing Release is not replaced.

After the Release is public, download all four assets with `gh release download vX.Y.Z --repo Jermic/dia-cli --dir release-files`, then run `cd release-files && shasum -a 256 -c checksums.txt` to verify both archives. In a checkout of the separate `Jermic/homebrew-tap` repository, replace `Formula/dia-cli.rb` with the downloaded `release-files/dia-cli.rb`, inspect the version, two architecture URLs, and their separate SHA-256 values, then commit and push that tap change. The tap is maintained separately; this repository's workflow does not write to it.

## License

MIT
