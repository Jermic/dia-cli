# macOS provides osascript and JavaScript for Automation (JXA).
typeset -g _DIA_CLI_SCRIPT="${${(%):-%N}:A:h}/scripts/dia.js"
typeset -ga _DIA_CLI_TAB_IDS _DIA_CLI_WINDOW_IDS

_dia_cli_remember_tabs() {
  emulate -L zsh
  local -a lines fields
  local line
  lines=("${(@f)1}")
  _DIA_CLI_TAB_IDS=()
  _DIA_CLI_WINDOW_IDS=()
  for line in "${lines[@]:1}"; do
    fields=("${(@ps:\t:)line}")
    _DIA_CLI_WINDOW_IDS+=("${fields[1]}")
    _DIA_CLI_TAB_IDS+=("${fields[3]}")
  done
}

# Set REPLY to a padded cell, measuring terminal columns instead of characters.
_dia_cli_cell() {
  local text="$1" char
  local -i width=$2 used=0 char_width
  REPLY="$text"
  if (( ${(m)#text} > width )); then
    REPLY=''
    for char in "${(@s::)text}"; do
      char_width=${(m)#char}
      (( used + char_width >= width )) && break
      REPLY+="$char"
      (( used += char_width ))
    done
    REPLY+='…'
  fi
  REPLY="${(mr:width:)REPLY}"
}

_dia_cli_render_tabs() {
  emulate -L zsh
  setopt multibyte
  local -a lines fields
  local line title url REPLY
  lines=("${(@f)1}")
  local -i count=$(( $#lines - 1 )) number=0 columns=$2 detail=${3:-0} linked
  local -i number_width=${#count} title_width
  (( columns < 16 )) && columns=16
  title_width=$(( columns - number_width - 3 ))
  if (( detail )); then
    title_width=$(( (columns - number_width - 5) * 2 / 5 ))
    (( title_width > 48 )) && title_width=48
  fi
  _dia_cli_cell TITLE "$title_width"
  printf '\e[1;33m%*s\e[0m  \e[1;36m%s\e[0m' "$number_width" '#' "$REPLY"
  (( detail )) && printf '  \e[1;34mURL\e[0m'
  print
  for line in "${lines[@]:1}"; do
    fields=("${(@ps:\t:)line}")
    title="${fields[-2]}"
    url="${fields[-1]}"
    [[ -n "$title" ]] || title='(untitled)'
    (( ++number ))
    _dia_cli_cell "$title" "$title_width"
    printf '\e[33m%*d\e[0m  \e[1m' "$number_width" "$number"
    # Link only web URLs; never allow page text to inject OSC control sequences.
    linked=0
    [[ "$url" == http(s|)://?* && "$url" != *[[:space:][:cntrl:]]* ]] && linked=1
    (( linked )) && printf '\e]8;;%s\e\\' "$url"
    printf '%s' "$REPLY"
    (( linked )) && printf '\e]8;;\e\\'
    printf '\e[0m'
    if (( detail )); then
      printf '  \e[2;34m'
      (( linked )) && printf '\e]8;;%s\e\\' "$url"
      printf '%s' "$url"
      (( linked )) && printf '\e]8;;\e\\'
      printf '\e[0m'
    fi
    print
  done
}

dia() {
  emulate -L zsh
  local -A short_commands=(t tabs n count l links c current f find o open fo focus ps profiles p profile h help w windows cl close m move nav navigate la layout e exec)
  if (( $# )) && [[ -n "${short_commands[$1]:-}" ]]; then
    set -- "${short_commands[$1]}" "${@:2}"
  fi
  if [[ "$OSTYPE" != darwin* ]]; then
    print -u2 'dia-cli requires macOS.'
    return 1
  fi
  if [[ "${1:-}" == (open|close|move|navigate|exec) ]]; then
    local row_command="$1"
    local -i expected=2
    [[ "$row_command" == (move|navigate|exec) ]] && expected=3
    if (( $# != expected )) || [[ ! "$2" =~ '^[1-9][0-9]*$' ]]; then
      print -u2 "Usage: dia $row_command <positive-row-number>${${expected:#2}:+ <argument>}"
      return 2
    fi
    local count=${#_DIA_CLI_TAB_IDS}
    if (( count == 0 )); then
      print -u2 'No saved tab list. Run dia tabs or dia find in this terminal first.'
      return 1
    fi
    # Bound the digit count before arithmetic to reject overflowing integers.
    if (( ${#2} > ${#count} )) || (( $2 > count )); then
      print -u2 'Row is outside the saved list. Refresh with dia tabs or dia find.'
      return 1
    fi
    command osascript -l JavaScript "$_DIA_CLI_SCRIPT" "_$row_command" \
      "${_DIA_CLI_WINDOW_IDS[$2]}" "${_DIA_CLI_TAB_IDS[$2]}" "${@:3}"
    return $?
  fi
  local output
  local -i detail=1 skip_window=0
  local arg
  if [[ "${1:-}" == tabs ]]; then
    detail=0
    for arg in "${@:2}"; do
      if (( skip_window )); then
        skip_window=0
      elif [[ "$arg" == --window ]]; then
        skip_window=1
      elif [[ "$arg" == (-d|-detail) ]]; then
        detail=1
      fi
    done
  fi
  if [[ -t 1 && "${TERM:-dumb}" != dumb && "${1:-}" == (tabs|find|current) ]]; then
    output=$(command osascript -l JavaScript "$_DIA_CLI_SCRIPT" "$@") || return $?
    if [[ "$1" == (tabs|find) ]]; then
      _dia_cli_remember_tabs "$output"
    fi
    _dia_cli_render_tabs "$output" "${COLUMNS:-80}" "$detail"
  else
    command osascript -l JavaScript "$_DIA_CLI_SCRIPT" "$@"
  fi
}
