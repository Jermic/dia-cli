#!/usr/bin/env python3
import importlib.util
from pathlib import Path
import subprocess
import unittest
import sys

sys.dont_write_bytecode = True

spec = importlib.util.spec_from_file_location("guard", Path(__file__).with_name("should-test.py"))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class ShouldTest(unittest.TestCase):
    def test_events_and_remote_tags(self):
        for event, ref, lines, expected in [
            ("push", "refs/heads/main", "abc refs/tags/v1.2.3", False),
            ("push", "refs/heads/main", "tag refs/tags/v1.2.3\nabc refs/tags/v1.2.3^{}", False),
            ("push", "refs/heads/main", "abc refs/tags/v1.2.3\nother refs/tags/v1.2.3^{}", True),
            ("push", "refs/heads/main", "other refs/tags/v1.2.3", True),
            ("push", "refs/heads/main", "abc refs/tags/v1.2.3-rc.1", True),
            ("push", "refs/heads/main", "abc refs/tags/v01.2.3", True),
            ("push", "refs/heads/main", "", True),
            ("push", "refs/heads/main", "malformed", True),
            ("push", "refs/tags/v1.2.3", "abc refs/tags/v1.2.3", True),
            ("pull_request", "refs/pull/1/merge", "abc refs/tags/v1.2.3", True),
            ("push", "refs/heads/other", "abc refs/tags/v1.2.3", True),
        ]:
            with self.subTest(event=event, ref=ref, lines=lines):
                self.assertEqual(guard.should_test(event, ref, "abc", lambda: lines), expected)

    def test_remote_failure_keeps_tests(self):
        for error in (OSError("offline"), UnicodeError("invalid ref encoding"), subprocess.CalledProcessError(1, "git"), subprocess.TimeoutExpired("git", 30)):
            def query():
                raise error
            self.assertTrue(guard.should_test("push", "refs/heads/main", "abc", query))

    def test_tag_and_pull_request_do_not_query(self):
        def query():
            self.fail("queried tags for a mandatory test event")
        self.assertTrue(guard.should_test("push", "refs/tags/v1.2.3", "abc", query))
        self.assertTrue(guard.should_test("pull_request", "refs/pull/1/merge", "abc", query))


if __name__ == "__main__":
    unittest.main()
