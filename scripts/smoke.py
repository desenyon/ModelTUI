#!/usr/bin/env python3
"""Offline command and real PTY smoke checks; no third-party Python packages."""
import errno
import json
import os
import pathlib
import pty
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time
import fcntl

binary = str(pathlib.Path(sys.argv[1] if len(sys.argv) > 1 else "bin/modeltui").resolve())


def run(*args, success=True):
    result = subprocess.run([binary, *args], capture_output=True, text=True, timeout=15)
    assert (result.returncode == 0) == success, (args, result.returncode, result.stderr)
    return result


with tempfile.TemporaryDirectory(prefix="modeltui-smoke-") as cache:
    flags = ["--offline", "--cache-dir", cache]
    assert "modeltui" in run("--help").stdout.lower()
    assert run("version").stdout.strip()
    result = json.loads(run("list", *flags, "--format", "json", "--limit", "3").stdout)
    assert result["source"] == "embedded snapshot"
    assert result["count"] == len(result["offerings"]) == 3
    provider = result["offerings"][0]["provider_id"]
    filtered = json.loads(run("list", *flags, "--provider", provider, "--format", "json").stdout)
    assert filtered["count"] > 0
    assert all(row["provider_id"] == provider for row in filtered["offerings"])
    empty = json.loads(run("list", *flags, "--query", "__modeltui_no_such_model__", "--format", "json").stdout)
    assert empty["offerings"] == [] and empty["count"] == 0
    table = run("list", *flags, "--limit", "1")
    assert "PROVIDER" in table.stdout and "Source: embedded snapshot" in table.stderr
    for args in [("--format", "xml"), ("--sort", "bad"), ("--capability", "bad"), ("--timeout", "0s")]:
        assert run("list", *flags, *args, success=False).stderr.strip()
    assert run("update", *flags, success=False).stderr.strip()
    assert not list(pathlib.Path(cache).iterdir()), "offline commands wrote cache files"
    print("PASS: help, version, offline JSON/table, provider filter, empty results, invalid options")

    pid, master = pty.fork()
    if pid == 0:
        os.environ["TERM"] = "xterm-256color"
        os.execv(binary, [binary, *flags])
    fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 36, 110, 0, 0))
    output = bytearray()
    quit_sent = False
    status = None
    deadline = time.monotonic() + 20
    try:
        while time.monotonic() < deadline:
            if select.select([master], [], [], 0.1)[0]:
                try:
                    chunk = os.read(master, 65536)
                    output.extend(chunk)
                except OSError as exc:
                    if exc.errno != errno.EIO:
                        raise
                if b"embedded snapshot" in output and not quit_sent:
                    # Navigate to offerings, then exit. Unit tests also verify
                    # capability intersection and navigation state transitions.
                    os.write(master, b"3q")
                    quit_sent = True
            done, status_value = os.waitpid(pid, os.WNOHANG)
            if done:
                status = status_value
                break
        assert quit_sent, "TUI never rendered the offline catalog"
        assert status is not None and os.waitstatus_to_exitcode(status) == 0, f"TUI failed: {status}"
        print("PASS: PTY startup, offline catalog render, keyboard navigation, clean quit")
    finally:
        if status is None:
            os.kill(pid, signal.SIGTERM)
            os.waitpid(pid, 0)
        os.close(master)
