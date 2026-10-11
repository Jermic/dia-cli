# Dia CLI

A lightweight Zsh plugin for finding and focusing Dia browser tabs through macOS Apple Events. Inspired by [worktree-jump](https://github.com/Jermic/worktree-jump).

## Requirements

- macOS, Zsh, and Dia with scripting support.
- No Node.js, Python, browser extension, or third-party runtime is required.
- Allow your terminal to control Dia when macOS asks. Permissions can be reviewed in **System Settings → Privacy & Security → Automation**.
- Dia must already be running. The plugin does not launch it automatically.

## Installation

Clone into the Oh My Zsh custom plugins directory:

```zsh
git clone https://github.com/Jermic/dia-cli.git "${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/dia-cli"
```

Add `dia-cli` to the existing `plugins=(...)` array in `~/.zshrc`, then reload your shell. Keep your other plugins.

Without Oh My Zsh, source the plugin directly:

```zsh
source /path/to/dia-cli/dia-cli.plugin.zsh
```

The plugin defines a `dia` shell function. Check `whence -a dia` before installation if you already use another command with that name. No shell configuration is modified automatically.

## Usage

```zsh
dia tabs                    # All windows and profiles, sorted by title
dia tabs -d                 # Detailed view: linked title and full linked URL
dia tabs -detail            # Same as -d
dia tabs -s                 # Explicit title sort (also -st)
dia tabs -d -sl             # Detailed view sorted by URL (also -sl -d)
dia count                   # Number of listed tabs
dia links                   # Full URLs only, one per line, sorted by title
dia current                 # Front window's active tab
dia find "github"           # Literal, case-insensitive title/URL search
dia open 2                  # Focus row 2 from this shell's last tabs/find list
dia focus <tab-id>           # Use the TAB value from the list
dia profiles                # Window ID, profile index, profile name
dia profile "Work"          # Exact, case-sensitive profile name
dia help
```

Every public subcommand has a short form. Short forms accept exactly the same arguments and options; they are subcommands, not dash-prefixed options.

| Short | Full | Example |
| --- | --- | --- |
| `t` | `tabs` | `dia t -d -sl` |
| `n` | `count` | `dia n` |
| `l` | `links` | `dia l` |
| `c` | `current` | `dia c` |
| `f` | `find` | `dia f github` |
| `o` | `open` | `dia o 2` |
| `fo` | `focus` | `dia fo <tab-id>` |
| `ps` | `profiles` | `dia ps` |
| `p` | `profile` | `dia p Work` |
| `h` | `help` | `dia h` |

Append `--window <window-id>` to limit browser queries or explicit-ID/name commands to one window (`open` uses its saved window ID):

```zsh
dia find "github" --window <window-id>
dia focus <tab-id> --window <window-id>
dia profile "Work" --window <window-id>
```

In an interactive terminal, `dia tabs` displays two aligned columns: `#` and a clickable `TITLE` linked to the full URL. `dia tabs -d` (or `-detail`) adds a third column with the full, clickable URL. Flags after `tabs` may appear in any order, including around `--window <window-id>`. `find` and `current` retain the three-column view. Styling follows worktree-jump: yellow numbers, cyan title headers, bold titles, and subdued blue URLs. Both title and URL use the same OSC 8 hyperlink syntax as `wt -p`.

`tabs` sorts by title by default; `-s` and `-st` explicitly select title sorting. `-sl` sorts by URL and requires `-d` or `-detail`; without it the command fails before reading Dia. If multiple sorting flags are supplied, the last one wins. Sorting is ascending, case-insensitive, and uses Unicode string order rather than Chinese pinyin. URL sort keys omit the initial `http://` or `https://` without changing the displayed or linked address. Equal keys retain their original order; empty values sort first. Display row numbers are assigned after sorting. Sorting also applies to piped output; `find` keeps its original order.

The title column adapts to the terminal (minimum 16 columns); long titles are shortened with an ellipsis and empty titles display `(untitled)`. In detailed view, URLs are displayed in full, including the scheme, path, and query string, and can naturally wrap past the terminal width. Only HTTP(S) targets become hyperlinks. Use your terminal's link-opening gesture (often Command-click). Link support depends on the terminal; opening a URL uses the default browser and does not focus an existing Dia tab. Use `dia open <number>` to focus an existing tab from a numbered `tabs` or `find` list instead; `focus` still takes a tab ID.

Each successful `tabs` or `find` list rendered in a normal terminal replaces an in-memory mapping of row numbers to window/tab IDs in that Zsh session. `dia open 2` sends a direct focus request for those saved IDs without searching, sorting, or enumerating tabs again. A closed tab or window produces a refresh-list message; other automation errors remain visible. Empty results clear the mapping; failed listings leave the previous mapping intact. `links`, `current`, pipes, redirected output, and `TERM=dumb` do not update it. Run lists directly in the same shell, not inside command substitutions or subshells. No URLs are saved to disk, and no bare-number aliases are installed, so worktree-jump shortcuts remain unchanged. IDs are not guaranteed to survive a Dia restart; refresh the list after restarting.

Chinese and combining-character widths use Zsh's native display-width support with your UTF-8 locale. Complex emoji sequences can still vary in width between terminals. No extra width library is required.

`dia links` always prints plain, full URLs, one per line, with no header, numbers, colors, or OSC hyperlinks. It follows the default title ordering of `tabs`, accepts `--window <window-id>`, skips empty URLs, and preserves duplicate URLs belonging to different tabs. For example, `dia links | pbcopy` copies the URLs to the macOS clipboard. Browser control characters are still sanitized as in other output.

Pipes, redirected output, and `TERM=dumb` retain full tab-separated data with a header (except `count`) and no hyperlinks. For example, `dia tabs | cat` shows the original `WINDOW`, `PROFILE`, `TAB`, `TITLE`, and `URL` columns, including the IDs used by `focus`. `profiles` also retains its original output. Control characters are replaced with spaces so a page cannot inject terminal escape sequences. Search with no matches succeeds with a header only. Invalid arguments, missing or ambiguous targets, and automation errors return a nonzero exit status.

The plugin reads each profile's favorite, pinned, and window tabs as exposed by Dia's scripting dictionary. It deduplicates the same tab ID within a window, keeping the first profile attribution, but preserves occurrences across different windows. Counts represent these window/tab pairs, not unique URLs. This does not depend on which tabs a Chromium extension can see.

`focus` and `profile` bring the target window forward. Ambiguous targets are rejected; specify a window ID. Profile names must be unique within that window. Tab/window IDs come from Dia and should not be treated as permanent bookmarks.

## Scope and limitations

- No tab closing, URL changes, tab creation, JavaScript execution, history, bookmarks, or shell numeric aliases.
- Listing does not intentionally activate or load tabs. Actual visibility is determined by Dia's Apple Events implementation.
- Tab properties are bulk-read for speed, not as an atomic snapshot. Avoid modifying tabs while listing; detected count changes produce a retry error, but same-size reorders cannot be detected reliably.
- Browser titles and URLs may be private. The plugin prints them locally and does not upload, cache, or log them.

## Updating

```zsh
git -C "${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/dia-cli" pull --ff-only
```

## Development

The plugin entry point invokes `scripts/dia.js` using macOS JavaScript for Automation (JXA). Arguments are passed as data, never interpolated into executable source.

```zsh
zsh tests/smoke.zsh
```

Tests use fake Dia objects and do not change browser state. They cover multiple windows/profiles, duplicate tab IDs, ambiguous targets, literal search, terminal control characters, invalid arguments, and unavailable browser state. For a live read-only check, source the plugin and run `dia count`, `dia profiles`, and `dia current`.

## License

MIT
