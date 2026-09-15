#!/usr/bin/env python3
"""Exercise the built CLI, including real process exit codes and checked-in exports."""
import json
from pathlib import Path
import subprocess
import tempfile

ROOT = Path(__file__).resolve().parents[1]
BINARY = ROOT / "bin" / "walker"


def run(*args, input=None, code=0):
    result = subprocess.run(
        [str(BINARY), *args], input=input, text=True,
        capture_output=True, cwd=ROOT, timeout=20,
    )
    assert result.returncode == code, (args, result.returncode, result.stderr)
    return result


def main():
    exported = run("extract", "--demo", "--config", "example/postman-options.json")
    assert not exported.stderr
    assert exported.stdout == (ROOT / "example/collection.json").read_text()
    manifest = run("extract", "--demo", "--format", "json")
    assert manifest.stdout == (ROOT / "example/routes.json").read_text()
    assert len(json.loads(manifest.stdout)) == 4
    report = run("diff", "example/diff/previous.json", "example/diff/current.json")
    assert report.stdout == "ADDED    POST /users\nREMOVED  GET /users/{id}\n"
    failure = run("diff", "example/diff/previous.json", "example/diff/current.json",
                  "--fail-on-removed", "--format", "json", code=2)
    assert json.loads(failure.stdout)["removed"] == [{"method": "GET", "path": "/users/{id}"}]
    assert "endpoint removal policy failed" in failure.stderr
    unchanged = run("diff", "example/routes.json", "-", "--fail-on-removed",
                    input=manifest.stdout)
    assert unchanged.stdout == "No endpoint changes.\n"
    malformed = run("extract", "--input", "-", input="[] []", code=1)
    assert not malformed.stdout and malformed.stderr
    with tempfile.TemporaryDirectory() as directory:
        before = Path(directory) / "previous.json"
        before.write_text("[]")
        additions = run("diff", str(before), "-", "--fail-on-removed",
                        input='[{"method":"GET","path":"/new"}]')
        assert additions.stdout == "ADDED    GET /new\n"
    print("CLI smoke checks passed: examples, stdin, streams, and exit codes 0/1/2.")


if __name__ == "__main__":
    main()
