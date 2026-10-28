# Dia CLI

Search and organize Dia tabs from your terminal. Move matching tabs between Profiles, preview duplicate cleanup, and count tabs by domain.

Runs on macOS with prebuilt Apple Silicon and Intel binaries. No Go, Node.js, or Python installation is needed to use a release package.

[Install](#install) · [Browse tabs](#browse-and-count-tabs) · [Move tabs](#move-tabs-between-profiles) · [Pinned tabs](#pinned-tabs) · [All commands](docs/reference.md#commands)

## Install

```zsh
brew install jermic/tap/dia-cli
dia --version
```

Open Dia before using tab commands. When macOS prompts, allow your terminal to control Dia in **System Settings → Privacy & Security → Automation**.

Homebrew installs Zsh completion for commands, flags, and Profile names. Open a new terminal after installation. Profile completion also needs a running Dia and Automation permission.

To update, run `brew upgrade jermic/tap/dia-cli`. See the [installation guide](docs/install.md) for shell setup, old plugin migration, manual packages, source builds, and removal.

## Browse and count tabs

```zsh
dia ps -c                 # Profiles and tab counts
dia t -p Day              # Tabs in Day
dia f github              # Search titles and URLs
dia l -p GitHub            # Copyable URLs from GitHub
```

Profile names are exact and case-sensitive. Replace `Day` and `GitHub` with names shown by `dia ps`.

Count domains by root, or keep subdomains separate:

```zsh
dia ds                    # Group docs.example.com under example.com
dia ds -s                 # Count each full hostname separately
dia ps -c -up             # Count only unpinned tabs in each Profile
```

## Move tabs between Profiles

Preview before moving. `-n` means `--dry-run` and makes no changes:

```zsh
dia mv github.com GitHub -n
```

This selects `github.com` and its subdomains across Profiles, excludes pinned tabs, and skips tabs already in `GitHub`. The destination must already exist. Tabs stay in their current windows.

After checking the preview, remove `-n` to apply:

```zsh
dia mv github.com GitHub
```

Use `-p` to limit the source Profile. The positional Profile argument is always the destination:

```zsh
dia mv github.com GitHub -p Day -n
```

Other common selections:

```zsh
dia mv Watch -k bilibili -k youtube -n  # Match either word in the URL
dia mv Read -p Day -exd x.com -n        # Unpinned Day tabs outside x.com
dia mv github.com GitHub -E -n         # Exact host, without subdomains
```

A destination-only command needs an explicit tab filter. `dia mv Read -p Day -n` is rejected because `-p` alone does not select tabs. See the [batch move reference](docs/reference.md#batch-moving) for selection rules and partial failures.

## Pinned tabs

| Operation | Default behavior |
| --- | --- |
| List, search, or count tabs | Include pinned tabs. Add `-up` to exclude them. |
| Batch move with `mv` | Exclude pinned tabs. Add `-a` to include them. |
| Duplicate cleanup with `dd` | Protect every pinned tab from closing. |
| Act on a saved row with `m` or `cl` | Act on that explicit tab, even if pinned. |

To include pinned tabs in a move preview, use `dia mv github.com GitHub -a -n`. To select only pinned tabs, use `dia mv github.com GitHub --pinned -n`.

`-a` is short for `--include-pinned` on `mv`. It preserves the domain, Profile, and other filters, and cannot combine with `--pinned` or `-up`.

## Preview and remove duplicate tabs

Preview duplicate groups without closing anything:

```zsh
dia dd -p Day
```

After reviewing the plan, add `-a` to close safe duplicate copies:

```zsh
dia dd -p Day -a
```

On `dd`, `-a` means `--apply`, not include pinned tabs. Pinned, active, and shared tabs remain protected. URLs must match exactly, and duplicate groups stay within one window and one Profile. See [duplicate protection](docs/reference.md#duplicate-tabs) for the full rules.

## Search history and bookmarks

```zsh
dia hs github
dia bm "reading list" --json
```

Both commands search titles and URLs without a running browser. They read the last-used disk profile, which is distinct from sidebar Profiles such as `Day`. They do not support `-p` or other tab filters. See [history and bookmark sources](docs/reference.md#search-history-and-bookmarks) for limits and JSON fields.

## Command and filter reference

Short and long command names accept the same arguments. Use `dia mv -h` or `dia help mv` for command help.

| Task | Short commands | Long commands |
| --- | --- | --- |
| Browse tabs | `t`, `f`, `c`, `l` | `tabs`, `find`, `current`, `links` |
| Count and summarize | `n`, `ds`, `ps` | `count`, `domains`, `profiles` |
| Move and clean up | `mv`, `dd`, `m`, `cl` | `move-domain`, `dedupe`, `move`, `close` |
| Focus and navigate | `o`, `fo`, `p`, `nav` | `open`, `focus`, `profile`, `navigate` |
| Search local data | `hs`, `bm` | `history`, `bookmarks` |
| Browser controls | `w`, `la`, `e` | `windows`, `layout`, `exec` |
| Help and version | `h`, `v` | `help`, `version` |

`md` remains an alias for `mv`. `dia -v` and `dia --version` also show the version. Row actions use the last visible `tabs` or `find` list in the same terminal and shell. See [all command arguments](docs/reference.md#commands) and [saved-row behavior](docs/reference.md#output-and-saved-rows).

For `t`, `f`, `n`, `l`, `ds`, `mv`, `dd`, and `ps -c`, combine `-p` for the source Profile, `-D` for domains, `-k` for URL keywords, `-exd` for excluded domains, and `-up` for unpinned tabs.

Domains and keywords match alternatives. Exclusions remove matches; Profile and state filters narrow the selection. Plain domains include subdomains. Quote `'*.github.com'` for subdomains only, or add `-E` to match a plain domain exactly. See [all filters and their long forms](docs/reference.md#filters-and-arguments).

## Documentation

- [Command reference](docs/reference.md) covers command arguments, filters, output, and safety checks.
- [Installation guide](docs/install.md) covers Homebrew, shell completion, and source builds.
- [Development and releases](docs/development.md) covers tests, architecture builds, release assets, and Homebrew Tap updates.

Dia CLI is a Go binary with an embedded JXA backend that communicates through macOS Apple Events. Browser reads and writes are not atomic. An operation can stop after partial success if tabs change or Dia reports an error. Completed actions remain completed.

## License

[MIT](LICENSE)
