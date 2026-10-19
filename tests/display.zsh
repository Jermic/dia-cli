#!/usr/bin/env zsh
emulate -L zsh
set -eu
root="${0:A:h:h}"
cd "$root"
go test -run 'TestGoRendererKeepsCurrentAndSafeLinks|TestRunFindAndCurrentShowURLs' .
print 'Display tests passed through the Go renderer and run entry point.'
