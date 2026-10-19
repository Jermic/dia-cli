#!/usr/bin/env python3
import os
import re
import subprocess


def should_test(event, ref, sha, query):
    if event != "push" or ref != "refs/heads/main":
        return True
    try:
        lines = query().splitlines()
    except (OSError, UnicodeError, subprocess.SubprocessError):
        return True
    tags = {}
    peeled = {}
    for line in lines:
        fields = line.split()
        if len(fields) != 2:
            continue
        oid, name = fields
        if re.fullmatch(r"refs/tags/v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", name):
            tags[name] = oid
        elif name.endswith("^{}"):
            peeled[name[:-3]] = oid
    return not any(peeled.get(name, oid) == sha for name, oid in tags.items())


if __name__ == "__main__":
    run = should_test(
        os.environ["GITHUB_EVENT_NAME"],
        os.environ["GITHUB_REF"],
        os.environ["GITHUB_SHA"],
        lambda: subprocess.check_output(
            ["git", "ls-remote", "--tags", "origin", "v*"], text=True, timeout=30
        ),
    )
    with open(os.environ["GITHUB_OUTPUT"], "a") as output:
        output.write(f"run={str(run).lower()}\n")
