#!/usr/bin/env zsh
emulate -L zsh
set -eu
setopt multibyte
export LC_ALL=en_US.UTF-8
source "${0:A:h:h}/dia-cli.plugin.zsh"

fixture=$'WINDOW\tPROFILE\tTAB\tTITLE\tURL\nw1\tPersonal\ta\t中文标题\thttps://example.com/very/long/path?q=1\nw1\tPersonal\tb\tASCII title with a long suffix\thttps://example.org\nw1\t\tc\t\tjavascript:alert(1)'
compact=$(_dia_cli_render_tabs "$fixture" 40 0)
[[ "$compact" == *$'\e]8;;https://example.com/very/long/path?q=1\e\\中文标题'* ]]
compact_plain=$(print -r -- "$compact" | sed $'s/\033]8;;[^\033]*\033\\\\//g; s/\033\\[[0-9;]*m//g')
[[ "$compact_plain" != *https://* && "$compact_plain" != *URL* ]]
[[ "$compact" == *$'\e[33m'* && "$compact" == *$'\e[1;36m'* ]]
rendered=$(_dia_cli_render_tabs "$fixture" 40 1)
[[ "$rendered" != *WINDOW* && "$rendered" != *PROFILE* && "$rendered" != *$'\t'* ]]
[[ "$rendered" == *$'\e]8;;https://example.com/very/long/path?q=1\e\\'* ]]
[[ "$rendered" != *$'\e]8;;javascript:'* ]]
# Strip OSC 8 envelopes, leaving exactly the text the terminal displays.
plain=$(print -r -- "$rendered" | sed $'s/\033]8;;[^\033]*\033\\\\//g; s/\033\\[[0-9;]*m//g')
lines=("${(@f)plain}")
[[ "$#lines" == 4 ]]
[[ "${lines[2]}" == '1  中文标题'* ]]
[[ "${lines[3]}" == *'…'* ]]
[[ "${lines[2]}" == *'https://example.com/very/long/path?q=1' ]]
[[ "${lines[3]}" == *'https://example.org' ]]
# URL columns start at the same display column, including after Chinese titles.
first_prefix="${lines[2]%%https://*}"
second_prefix="${lines[3]%%https://*}"
[[ ${(m)#first_prefix} == ${(m)#second_prefix} ]]
[[ "$plain" != *$'\e'* ]]

local REPLY
_dia_cli_cell '中文中文中文' 9
[[ "$REPLY" == '中文中文…' && ${(m)#REPLY} == 9 ]]
_dia_cli_cell 'é cafe' 10
[[ ${(m)#REPLY} == 10 ]]
_dia_cli_cell '%F{red}$(echo bad)' 24
[[ "$REPLY" == '%F{red}$(echo bad)'* ]]

for width in 20 80 160; do
  rendered=$(_dia_cli_render_tabs "$fixture" "$width" 1)
  plain=$(print -r -- "$rendered" | sed $'s/\033]8;;[^\033]*\033\\\\//g; s/\033\\[[0-9;]*m//g')
  [[ "$plain" == *'https://example.com/very/long/path?q=1'* ]]
  [[ "$plain" == *'https://example.org'* ]]
done
unsafe=$'WINDOW\tTAB\tTITLE\tURL\nw\ta\tTest\thttps://example.org/\e]8;;evil'
[[ "$(_dia_cli_render_tabs "$unsafe" 40)" != *$'\e]8;;https://'* ]]
print 'Display tests passed (CJK widths, clipping, hyperlinks, unsafe URLs).'
