#!/usr/bin/env zsh
emulate -L zsh
set -eu
source "${0:A:h:h}/dia-cli.plugin.zsh"

fake=$(mktemp)
trap 'rm -f "$fake"' EXIT
printf '%s\n' '#!/usr/bin/env zsh' 'for arg in "$@"; do print -r -- "$arg"; done' > "$fake"
chmod 700 "$fake"
_DIA_CLI_BINARY="$fake"
[[ "$(dia f 'o t' -D '*.example.com')" == $'f\no t\n-D\n*.example.com' ]]
[[ "$(dia e 2 $'\"quoted\";\n2+2')" == $'e\n2\n"quoted";\n2+2' ]]
_DIA_CLI_BINARY="$fake.missing"
if dia help >/dev/null 2>&1; then
  print -u2 'Missing Go binary must fail'
  exit 1
fi
print 'Session tests passed (plugin forwards exact arguments and reports missing build).'
