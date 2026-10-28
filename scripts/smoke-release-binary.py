#!/usr/bin/env python3
"""Boot an actual packaged server against a disposable database (#333)."""
import os
from pathlib import Path
import socket
import subprocess
import sys
import tempfile
import time
from urllib.request import urlopen

binary = Path(sys.argv[1]).resolve()
with tempfile.TemporaryDirectory(prefix="rekenraam-release-smoke-") as directory:
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        port = listener.getsockname()[1]
    # Explicit synthetic configuration; never inherit operator secrets or DB paths.
    env = {key: value for key, value in os.environ.items() if key in ("PATH", "LANG", "TZ")}
    env.update(APP_ENV="production", HTTP_ADDR=f"127.0.0.1:{port}",
               DATABASE_URL=f"file:{directory}/smoke.sqlite",
               SETUP_TOKEN="synthetic-release-smoke-token-0000000000000000")
    with open(Path(directory) / "app.log", "w+") as log:
        process = subprocess.Popen([str(binary)], cwd=directory, env=env, stdout=log, stderr=log)
        try:
            deadline = time.monotonic() + 30
            while True:
                try:
                    with urlopen(f"http://127.0.0.1:{port}/healthz", timeout=1) as response:
                        assert response.status == 204
                    break
                except OSError:
                    if process.poll() is not None or time.monotonic() > deadline:
                        log.seek(0)
                        raise RuntimeError(f"candidate did not start: {log.read()}")
                    time.sleep(0.1)
            with urlopen(f"http://127.0.0.1:{port}/", timeout=5) as response:
                html = response.read().decode()
                assert response.status == 200 and "_app/" in html
            print("Candidate health, fresh migration and embedded frontend smoke passed")
        finally:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
