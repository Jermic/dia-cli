#!/usr/bin/env python3
import os
import pty
import select
import signal
import time
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parent.parent
ZSH = r'''
fpath=("$DIA_COMPLETION_DIR" $fpath)
autoload -Uz compinit
compinit -D
autoload -Uz _dia
compadd() {
  shift
  local item
  for item in "$@"; do print -rn -- "$item"$'\0'; done
}
words=("$@")
CURRENT=${#words}
_dia
'''

with tempfile.TemporaryDirectory() as tmp:
    tmp = Path(tmp)
    fake = tmp / 'dia'
    trace = tmp / 'trace'
    marker = tmp / 'injected'
    fake.write_text("#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"$DIA_TRACE\"\n"
                    "[ \"$1\" = profiles ] || exit 9\n"
                    "cat <<'PROFILES'\nWINDOW\tINDEX\tPROFILE\nw1\t1\tWork Notes\n"
                    "w1\t2\tBob's Work\nw1\t3\tPrice $(touch " + str(marker) + ")\n"
                    "w2\t1\tWork Notes\nw2\t2\tTeam:Ops\nw2\t3\t阅读\nPROFILES\n"
                    "[ \"$DIA_FAIL\" != 1 ]\n")
    fake.chmod(0o755)
    env = dict(os.environ, PATH=f'{tmp}:{os.environ["PATH"]}', DIA_TRACE=str(trace), DIA_COMPLETION_DIR=str(ROOT / 'completions'))

    def complete(*args):
        result = subprocess.run(['zsh', '-f', '-c', ZSH, '_', 'dia', *args], env=env, capture_output=True)
        assert result.returncode in (0, 1) and not result.stderr, (result.returncode, result.stderr)
        return result.stdout.decode().rstrip('\0').split('\0') if result.stdout else []

    commands = complete('')
    assert {'history', 'hs', 'bookmarks', 'bm', 'dedupe', 'dd', 'domains', 'ds', 'tabs', 't', 'move-domain', 'mv', 'md', 'help', 'h', 'version', 'v', '-v', '--version'} <= set(commands)
    assert not trace.exists(), 'command completion queried Dia'
    for local_command in ('history', 'hs', 'bookmarks', 'bm'):
        assert complete(local_command, '') == ['-h', '--help', '--json']
        assert complete(local_command, 'github', '') == ['-h', '--help', '--json']
        assert complete(local_command, '--json', '') == ['-h', '--help']
        assert complete(local_command, '--', '') == []
    assert not trace.exists(), 'local search completion queried Dia'
    for version in ('version', 'v', '-v', '--version'):
        assert complete(version, '') == ['-h', '--help']
    assert not trace.exists(), 'version completion queried Dia'
    profiles = complete('mv', '')
    assert profiles == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    trace.unlink()
    assert complete('mv', '-') == ['-h', '--help'], 'command help missing before required arguments'
    assert not trace.exists(), 'domain completion queried Dia'
    assert complete('mv', 'example.com', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    assert trace.read_text().splitlines() == ['profiles']
    assert not marker.exists(), 'Profile data executed as shell code'
    trace.unlink()
    assert complete('mv', '--', '-up', '') == profiles, 'literal selector was mistaken for a target-only flag'
    trace.unlink()
    assert complete('mv', 'example.com', 'Work', '') == ['-h', '--help', '-w', '--window', '-p', '--profile', '-D', '--domain', '-k', '--keyword', '-E', '--exact-domain', '--pinned', '--loading', '-up', '--unpinned', '-exd', '--exclude-domain', '-n', '--dry-run', '-a', '--include-pinned']
    for move_command in ('mv', 'md', 'move-domain'):
        assert {'-a', '--include-pinned'} <= set(complete(move_command, 'Read', '-exd', 'x.com', ''))
        assert '--exclude-domain' in complete(move_command, 'Read', '--include-pinned', '')
    for other_command in ('t', 'ds', 'dd', 'ps'):
        assert '--include-pinned' not in complete(other_command, '')
    assert {'--pinned', '--loading', '-up', '--unpinned'} <= set(complete('ps', '-c', ''))
    assert not trace.exists(), 'flag completion queried Dia'
    for domain_command in ('domains', 'ds'):
        assert {'-s', '--subdomains', '-w', '--window', '-p', '--profile', '-D', '--domain', '-k', '--keyword', '-E', '--exact-domain', '--pinned', '--loading'} <= set(complete(domain_command, ''))
        assert complete(domain_command, '-k', '') == []
    assert {'-up', '--unpinned', '-exd', '--exclude-domain'} <= set(complete('l', ''))
    assert '--exclude-domain' in complete('mv', 'Read', '-up', '')
    assert complete('mv', 'Read', '-exd', '') == []
    assert complete('l', '--exclude-domain', '') == []
    assert not trace.exists(), 'domains flags queried Dia'
    for dedupe_command in ('dedupe', 'dd'):
        assert {'-a', '--apply', '-n', '--dry-run', '-p', '--profile', '-D', '--domain', '-k', '--keyword', '-E', '--exact-domain', '--pinned', '--loading', '-w', '--window'} <= set(complete(dedupe_command, ''))
        assert complete(dedupe_command, '-k', '') == []
        assert complete(dedupe_command, '-D', '') == []
        assert complete(dedupe_command, '--', '') == []
    assert not trace.exists(), 'dedupe flag completion queried Dia'
    assert complete('dd', '-p', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    trace.unlink()
    assert complete('ds', '-p', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    trace.unlink()
    assert complete('mv', 'Read', '-p', '') == profiles
    trace.unlink()
    assert '--exclude-domain' in complete('mv', 'Read', '--exact-domain', '')
    assert not trace.exists(), 'exact flag mistaken for target'
    assert '--exact-domain' in complete('ps', '-c', '')
    assert '--exact-domain' not in complete('windows', '')
    assert complete('move', '1', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    assert complete('profile', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    assert complete('tabs', '-p', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    assert complete('layout', '') == ['sidebar', 'tabStrip']
    assert complete('open', '1', '') == ['-h', '--help']
    trace.unlink()
    assert complete('open', '1', '-p', '') == []
    assert complete('links', '-p', '-p', '') == ['-h', '--help', '-w', '--window', '-p', '--profile', '-D', '--domain', '-k', '--keyword', '-E', '--exact-domain', '--pinned', '--loading', '-up', '--unpinned', '-exd', '--exclude-domain']
    assert '--keyword' in complete('tabs', '-p', '--', '')
    assert complete('mv', 'youtube', '--h') == ['-h', '--help']
    assert complete('tabs', '-k', '') == []
    assert complete('mv', 'youtube', 'Watch', '-k', '') == []
    assert complete('mv', 'youtube', '-k', '') == []
    assert complete('mv', '-k', '') == []
    assert complete('mv', '--keyword', '--help') == []
    assert complete('mv', '-k', 'youtube.com/watch', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    assert '--keyword' in complete('mv', '-k', 'youtube.com/watch', 'Watch', '')
    assert complete('exec', '1', '--', '-h', '') == []
    assert complete('mv', 'youtube', '--', '') == ['Work Notes', "Bob's Work", f'Price $(touch {marker})', 'Team:Ops', '阅读']
    trace.unlink()
    assert complete('unknown', '') == []
    assert not trace.exists(), 'invalid option or command queried Dia'
    assert complete('help', '') == commands, 'help command candidates changed'
    assert not marker.exists(), 'Profile data executed as shell code'
    env['DIA_FAIL'] = '1'
    assert complete('profile', '') == [], 'failed Profile query offered partial candidates'
    del env['DIA_FAIL']

    pid, fd = pty.fork()
    if pid == 0:
        os.execve('/bin/zsh', ['zsh', '-f', '-i'], dict(env, TERM='xterm', PS1='PROMPT> '))
    def read_until(needle):
        output = b''
        deadline = time.monotonic() + 5
        while needle not in output and time.monotonic() < deadline:
            ready, _, _ = select.select([fd], [], [], 0.2)
            if ready:
                output += os.read(fd, 65536)
        assert needle in output, output
        return output
    try:
        read_until(b'PROMPT> ')
        os.write(fd, b"fpath=($DIA_COMPLETION_DIR $fpath); autoload -Uz compinit; compinit -D; bindkey '^I' complete-word\n")
        read_until(b'PROMPT> ')
        os.write(fd, b'dia profile Bob\t')
        read_until(b"Bob\\'s\\ Work")
        os.write(fd, b'\x15dia mv -k youtube.com/watch Bob\t')
        read_until(b"Bob\\'s\\ Work")
    finally:
        os.kill(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
        os.close(fd)
print('Zsh completion candidates and first Tab insertion passed.')
