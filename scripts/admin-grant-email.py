#!/usr/bin/env python3
"""Grant the admin role to one existing learner.42 account through Compose."""

from __future__ import annotations

import os
import re
import shlex
import subprocess
import sys


EMAIL_RE = re.compile(r"^[^\s@]+@learner\.42\.tech$", re.IGNORECASE)


def main() -> int:
    if len(sys.argv) != 2 or not EMAIL_RE.fullmatch(sys.argv[1].strip()):
        print("Usage: python scripts/admin-grant-email.py you@learner.42.tech", file=sys.stderr)
        return 1
    email = sys.argv[1].strip().lower()
    compose = shlex.split(os.environ.get("COMPOSE", "docker compose"))
    sql = """set -eu
apk add --no-cache sqlite >/dev/null
sqlite3 /data/transcendance.db <<SQL
.parameter init
.parameter set :email "$ADMIN_EMAIL"
.timeout 5000
BEGIN IMMEDIATE;
UPDATE users SET role = 'admin' WHERE lower(email) = lower(:email) AND deleted_at IS NULL;
SELECT changes();
COMMIT;
SQL
"""
    command = compose + ["run", "--rm", "--no-deps", "-e", f"ADMIN_EMAIL={email}", "db-cli", "sh", "-c", sql]
    result = subprocess.run(command, text=True, capture_output=True, check=False)
    if result.returncode != 0:
        print(result.stderr or "Could not update the database.", file=sys.stderr)
        return result.returncode
    changed = result.stdout.strip().splitlines()
    if not changed or changed[-1].strip() != "1":
        print(f"No active user found for {email}.", file=sys.stderr)
        return 1
    print(f"Admin role granted to {email}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
