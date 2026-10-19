#!/usr/bin/env zsh
emulate -L zsh
set -eu

root="${0:A:h:h}"
zsh -n "$root/dia-cli.plugin.zsh"
source "$root/dia-cli.plugin.zsh"
[[ "$(dia help)" == *'Usage: dia'* ]]
if dia unknown >/dev/null 2>&1; then
  print -u2 'Unknown commands must fail'
  exit 1
fi
osascript -l JavaScript "$root/tests/behavior.js" "$root/scripts/dia.js"
zsh "$root/tests/display.zsh"
zsh "$root/tests/session.zsh"
_DIA_CLI_SCRIPT="$root/tests/arguments.js"
[[ "$(dia tabs)" == '["tabs"]' ]]
[[ "$(dia tabs -d --window 'a b')" == '["tabs","-d","--window","a b"]' ]]
[[ "$(dia tabs -detail)" == '["tabs","-detail"]' ]]
[[ "$(dia tabs -sl --window 'a b' -d)" == '["tabs","-sl","--window","a b","-d"]' ]]
[[ "$(dia find '-d')" == '["find","-d"]' ]]
[[ "$(dia tabs --bad)" == '["tabs","--bad"]' ]]
[[ "$(dia t -d -sl)" == '["tabs","-d","-sl"]' ]]
[[ "$(dia n)" == '["count"]' ]]
[[ "$(dia l)" == '["links"]' ]]
[[ "$(dia c)" == '["current"]' ]]
[[ "$(dia f 'o t')" == '["find","o t"]' ]]
[[ "$(dia fo 'tab-id')" == '["focus","tab-id"]' ]]
[[ "$(dia ps)" == '["profiles"]' ]]
[[ "$(dia p 'Work Profile')" == '["profile","Work Profile"]' ]]
[[ "$(dia h)" == '["help"]' ]]
[[ "$(dia w --window w1)" == '["windows","--window","w1"]' ]]
[[ "$(dia la sidebar)" == '["layout","sidebar"]' ]]
[[ "$(dia t --pinned --loading -d)" == '["tabs","--pinned","--loading","-d"]' ]]
print 'Smoke tests passed (no live browser changes).'
