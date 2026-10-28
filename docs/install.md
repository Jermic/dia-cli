# Install Dia CLI

[Back to README](../README.md)

Requires macOS. Tab commands require a running Dia with scripting support. Local history and bookmark searches work without a running browser. Homebrew selects the matching Apple Silicon or Intel archive automatically.

```zsh
brew install jermic/tap/dia-cli
dia --version
dia help
```

Homebrew installs Zsh completion for commands, aliases, valid flags, layouts, and Profile names. Open a new Zsh session after installation, or run `autoload -Uz compinit; compinit` in an existing session. Zsh must include Homebrew's `share/zsh/site-functions` directory in `fpath`. Completing a Profile name runs the read-only `dia profiles` command, so Dia must be running and macOS may ask for Automation permission. If Dia cannot be queried, completion offers no Profile names.

Upgrade with `brew upgrade jermic/tap/dia-cli`. Uninstall with `brew uninstall dia-cli`. Dia does not launch automatically. macOS may ask permission for your terminal to control Dia under **System Settings → Privacy & Security → Automation**.

If you previously used the Oh My Zsh plugin, remove only `dia-cli` from the existing `plugins=(...)` array in `~/.zshrc`, leaving its other entries in place. In an already open Zsh session, run `unfunction dia` if it loaded the old function, or open a new terminal. Check `command -v dia` and `dia --version` to confirm the Homebrew command is active. The old plugin directory can stay in place.

## Install a release archive manually

Download the archive for your Mac from the [latest GitHub Release](https://github.com/Jermic/dia-cli/releases/latest): `darwin_arm64` for Apple Silicon or `darwin_amd64` for Intel. Download `checksums.txt` from the same release, then verify and install the selected archive. For example, after downloading version 0.4.0 for Apple Silicon:

```zsh
grep '  dia-cli_0.4.0_darwin_arm64.tar.gz$' checksums.txt | shasum -a 256 -c - &&
tar -xzf dia-cli_0.4.0_darwin_arm64.tar.gz &&
mkdir -p "$HOME/.local/bin" &&
install -m 0755 dia "$HOME/.local/bin/dia"
```

Use the `darwin_amd64` filename on Intel, update the version for newer releases, and ensure `~/.local/bin` is on your `PATH`.

## Build from source

Requires Go 1.23 or newer. The generated `bin/dia` is ignored by Git.

```zsh
git clone https://github.com/Jermic/dia-cli.git
cd dia-cli
go build -o bin/dia .
./bin/dia help
```

Use `bin/dia` directly, or copy the built binary into a directory on your `PATH`. The binary contains the JXA script. Rebuild after updating the repository. To use completion from this checkout, add its `completions` directory to `fpath` before `compinit` and put `bin/dia` on `PATH`. The completion queries the `dia` binary for Profile names.

For Oh My Zsh, clone the repository into `${ZSH_CUSTOM:-$HOME/.oh-my-zsh/custom}/plugins/dia-cli`, build it there, add `dia-cli` to your existing `plugins=(...)` array, and reload Zsh. You can also source `dia-cli.plugin.zsh` directly. The plugin defines a `dia` function that forwards arguments to `bin/dia`; if the binary is missing it prints the build command. It does not change shell configuration automatically.
