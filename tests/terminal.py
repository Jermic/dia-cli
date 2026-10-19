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
        shown = re.sub(r'\x1b]8;;[^\x1b]*\x1b\\', '', shown)
        shown = re.sub(r'\x1b\[[0-9;]*m', '', shown)
        header = shown.splitlines()[0]
        assert 'https://example.com/path' in shown and 'URL' in header, shown
        positions.append(header.index('URL'))
    assert positions[0] < positions[1], positions
print('PTY binary display passed at 40 and 120 columns without COLUMNS.')
