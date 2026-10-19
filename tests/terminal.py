#!/usr/bin/env python3
import errno
import fcntl
import json
import os
import pty
import re
import struct
import subprocess
import tempfile
import termios
from pathlib import Path

root = Path(__file__).resolve().parent.parent
with tempfile.TemporaryDirectory() as directory:
    mock = Path(directory) / 'osascript'
    response = json.dumps({'output': 'WINDOW\tTAB\tTITLE\tURL\nw1\ta\tA long title for checking terminal width\thttps://example.com/path'})
    mock.write_text('#!/bin/sh\nprintf "%s\\n" ' + "'" + response + "'" + '\n')
    mock.chmod(0o700)
    env = os.environ.copy()
    env['PATH'] = directory + os.pathsep + env['PATH']
    env['TERM'] = 'xterm'
    env.pop('COLUMNS', None)
    positions = []
    for columns in (40, 120):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, columns, 0, 0))
        process = subprocess.Popen([str(root / 'bin/dia'), 'current'], stdin=slave, stdout=slave, stderr=slave, env=env)
        os.close(slave)
        chunks = []
        while True:
            try:
                chunk = os.read(master, 4096)
            except OSError as error:
                if error.errno == errno.EIO:
                    break
                raise
            if not chunk:
                break
            chunks.append(chunk)
        os.close(master)
        assert process.wait() == 0, b''.join(chunks)
        shown = b''.join(chunks).decode().replace('\r', '')
        assert '\x1b]8;;https://example.com/path\x1b\\' in shown, shown
        shown = re.sub(r'\x1b]8;;[^\x1b]*\x1b\\', '', shown)
        shown = re.sub(r'\x1b\[[0-9;]*m', '', shown)
        header = next(line for line in shown.splitlines() if "TITLE" in line)
        assert 'https://example.com/path' in shown and 'URL' in header, shown
        positions.append(header.index('URL'))
    assert positions[0] < positions[1], positions

    mock.write_text('#!' + __import__('sys').executable + '\n' + '''import json, os, sys
request = json.loads(sys.argv[-1])
if request.get('progress'):
    events = json.loads(os.environ.get('DIA_TEST_EVENTS', '[{"phase":"scan","completed":0,"total":0},{"phase":"preflight","window":"w1","profile":"Work","completed":1,"total":1},{"phase":"done","completed":2,"total":2}]'))
    for event in events:
        print(json.dumps({'progress': event}), flush=True)
print(os.environ['DIA_TEST_RESULT'])
''')

    def terminal_run(args, result, stdout_tty=True, stderr_tty=True, events=None, want_status=0):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 24, 120, 0, 0))
        process = subprocess.Popen([str(root / 'bin/dia'), *args],
                                   stdin=subprocess.DEVNULL,
                                   stdout=slave if stdout_tty else subprocess.PIPE,
                                   stderr=slave if stderr_tty else subprocess.PIPE,
                                   env=dict(env, DIA_TEST_RESULT=json.dumps(result), **({'DIA_TEST_EVENTS': json.dumps(events)} if events is not None else {})))
        os.close(slave)
        chunks = []
        while True:
            try:
                chunk = os.read(master, 4096)
            except OSError as error:
                if error.errno == errno.EIO:
                    break
                raise
            if not chunk:
                break
            chunks.append(chunk)
        os.close(master)
        out, err = process.communicate(timeout=5)
        terminal = b''.join(chunks).decode().replace('\r', '')
        assert process.returncode == want_status, (terminal, out, err)
        return terminal, (out or b'').decode(), (err or b'').decode()

    windows = {'output': 'WINDOW\tPROFILE\tLAYOUT\nw1\tA\tsidebar\nw2\tLong\ttabStrip'}
    shown, out, err = terminal_run(['windows'], windows)
    assert 'w1      A        sidebar\n' in shown and '\t' not in shown, shown
    shown, out, err = terminal_run(['windows'], windows, False, False)
    assert out == windows['output'] + '\n' and not err, (out, err)

    profiles = {'output': 'WINDOW\tINDEX\tPROFILE\tCOUNT\nw1\t1\tDay\t2\nw2\t12\t中文档\t785'}
    for args, result in ((['ps', '-c'], profiles),
                         (['ps'], {'output': 'WINDOW\tINDEX\tPROFILE\nw1\t1\tDay\nw2\t12\t中文档'})):
        shown, out, err = terminal_run(args, result)
        assert '│  INDEX  PROFILE' in shown and 'WINDOW' not in shown and 'w1' not in shown and 'w2' not in shown, shown
        assert '    1  Day' in shown and '   12  中文档' in shown, shown
        if args[-1] == '-c':
            assert 'Day          2\n' in shown and '中文档     785\n' in shown, shown
        shown, out, err = terminal_run(args, result, False, False)
        assert out == result['output'] + '\n' and not shown and not err, (shown, out, err)

    preview = {'output': 'ACTION\tWINDOW\tTAB\tSOURCE\tTARGET\tURL\nMOVE\tw1\ta\tPersonal\tWork\thttps://example.com',
               'move': {'matched': 2, 'moved': 0, 'skipped': 1, 'failed': 0, 'remaining': 1}}
    for stdout_tty, stderr_tty in ((True, True), (False, True), (True, False), (False, False)):
        shown, out, err = terminal_run(['mv', 'example.com', 'Work', '-n'], preview, stdout_tty, stderr_tty)
        assert ('Reading tabs' in shown) == stderr_tty, (shown, out, err)
        assert 'window' not in shown and 'w1' not in shown and 'WINDOW' not in shown, shown
        assert not err and 'Scanning' not in out, (shown, out, err)
        if stdout_tty:
            assert '┌  Move preview' in shown and '1 tab would move · 1 skipped · no changes made' in shown, shown
            assert 'ACTION  SOURCE    TARGET  URL\n' in shown and 'MOVE    Personal  Work    https://example.com\n' in shown, shown
            assert 'METRIC' not in shown and '\t' not in shown, shown
        else:
            assert out == preview['output'] + '\ndry-run matched=2 moved=0 skipped=1 failed=0 remaining=1\n', out
        assert 'Moving tabs' not in shown and 'Moved tabs' not in shown, shown

    for args, result in ((['links'], {'output': 'https://example.com\nhttps://a.example.com'}),
                         (['count'], {'output': '2'})):
        shown, out, err = terminal_run(args, result, False, True)
        assert out == result['output'] + '\n' and not shown and not err, (shown, out, err)


    events = [
        {'phase': 'scan', 'completed': 0, 'total': 2},
        {'phase': 'profile', 'window': 'w1', 'profile': 'Day', 'completed': 1, 'total': 4},
        {'phase': 'preflight', 'window': 'w1', 'completed': 1, 'total': 1},
        {'phase': 'profile', 'window': 'w2', 'profile': 'News', 'completed': 1, 'total': 4},
        {'phase': 'preflight', 'window': 'w2', 'completed': 1, 'total': 1},
        {'phase': 'plan', 'completed': 2, 'total': 2},
        {'phase': 'moving', 'completed': 0, 'total': 2},
        {'phase': 'moved', 'completed': 1, 'total': 2},
        {'phase': 'moving', 'completed': 1, 'total': 2},
        {'phase': 'moved', 'completed': 2, 'total': 2},
        {'phase': 'done', 'completed': 2, 'total': 2},
    ]
    moved = {'output': '', 'move': {'matched': 2, 'moved': 2, 'skipped': 0, 'failed': 0, 'remaining': 0}}
    shown, out, err = terminal_run(['mv', 'example.com', 'Work'], moved, events=events)
    assert shown.count('Reading tabs') == 1 and shown.count('Target ready') == 1, shown
    assert shown.count('Moving tabs · 1/2') == 1 and '\x1b[2K◇  Moved 2 tabs' in shown, shown
    assert '└  Done' in shown and 'COUNT' not in shown and 'w1' not in shown and 'w2' not in shown, shown
    stopped = {'output': '', 'move': {'matched': 2, 'moved': 1, 'skipped': 0, 'failed': 1, 'remaining': 1}, 'error': 'Original window w1 move error'}
    shown, out, err = terminal_run(['mv', 'example.com', 'Work'], stopped, events=events[:-2], want_status=1)
    assert '\x1b[2K▲  Move stopped' in shown and 'RESULT' in shown, shown
    assert 'Original window w1 move error' in shown and 'Incomplete · 1 tab moved' in shown and '└  Done' not in shown, shown

print('PTY global frames, live progress cleanup, partial failures, and pipe compatibility passed.')
