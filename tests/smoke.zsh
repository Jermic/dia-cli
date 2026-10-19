#!/usr/bin/env zsh
emulate -L zsh
set -eu

root="${0:A:h:h}"
zsh -n "$root/dia-cli.plugin.zsh"
source "$root/dia-cli.plugin.zsh"
[[ "$(dia help)" == *'dia <command>'* ]]
if dia unknown >/dev/null 2>&1; then
  print -u2 'Unknown commands must fail'
  exit 1
fi
osascript -l JavaScript "$root/tests/behavior.js" "$root/scripts/dia.js"
osascript -l JavaScript "$root/tests/batch.js" "$root/scripts/dia.js"
zsh "$root/tests/display.zsh"
zsh "$root/tests/session.zsh"
python3 "$root/tests/terminal.py"
python3 "$root/tests/completion.py"
print 'Smoke tests passed (no live browser changes).'
