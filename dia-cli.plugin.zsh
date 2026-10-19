typeset -g _DIA_CLI_BINARY="${${(%):-%N}:A:h}/bin/dia"

dia() {
  emulate -L zsh
  if [[ ! -x "$_DIA_CLI_BINARY" ]]; then
    print -u2 'dia-cli is not built. Run: go build -o bin/dia .'
    return 1
  fi
  command "$_DIA_CLI_BINARY" "$@"
}
