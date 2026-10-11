#!/usr/bin/env zsh
emulate -L zsh
set -eu
source "${0:A:h:h}/dia-cli.plugin.zsh"
_DIA_CLI_SCRIPT="${0:A:h}/arguments.js"
expect_failure() {
  if dia open "$@" >/dev/null 2>&1; then
    print -u2 "Expected rejection: $*"
    exit 1
  fi
}
expect_failure 1
_dia_cli_remember_tabs $'WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw2\tp\tb\tAlpha\thttps://a.test\nw1\tp\ta\tBeta\thttps://b.test'
[[ "$(dia open 2)" == '["_open","w1","a"]' ]]
[[ "$(dia o 2)" == '["_open","w1","a"]' ]]
[[ "$(dia open 1)" == '["_open","w2","b"]' ]]
for invalid in 0 -1 01 1.5 '1+1' abc 3 999999999999999999999999; do
  expect_failure "$invalid"
done
expect_failure
expect_failure 1 extra
dia links >/dev/null
dia tabs >/dev/null
dia find query >/dev/null
[[ "$(dia open 2)" == '["_open","w1","a"]' ]]
_dia_cli_remember_tabs $'WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw3\tp\tc\tNew result\thttps://c.test'
[[ "$(dia open 1)" == '["_open","w3","c"]' ]]
expect_failure 2
_dia_cli_remember_tabs $'WINDOW\tPROFILE\tTAB\tTITLE\tURL'
expect_failure 1
print 'Session tests passed (cached IDs, invalid rows, replacement, empty results, redirected output).'
