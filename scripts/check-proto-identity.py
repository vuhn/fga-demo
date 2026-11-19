#!/usr/bin/env python3
"""No tenant, person, project or actor identity in the service contracts (docs/service-contracts.md).

Reads the compiled descriptors (`buf build`) and fails when any message, field, enum type, service or rpc name
contains one of the tokens below as a *whole word* (snake_case and CamelCase are split into words, so
`user_message` is flagged and `username_hint` is not). Enum value names are out of scope: `FACT_SOURCE_USER`
names who stated a fact, not a person.

    check-proto-identity.py [proto_dir]      default: proto
"""
import json
import re
import subprocess
import sys

DENY = {"tenant", "tenants", "user", "users", "project", "projects", "actor", "actors"}


def words(name: str) -> set[str]:
    spaced = re.sub(r"(?<=[a-z0-9])(?=[A-Z])|_", " ", name)
    return {w.lower() for w in spaced.split() if w}


def names(msg: dict, path: str):
    yield path + msg["name"], msg["name"]
    for f in msg.get("field", []):
        yield f"{path}{msg['name']}.{f['name']}", f["name"]
    for e in msg.get("enumType", []):
        yield f"{path}{msg['name']}.{e['name']}", e["name"]
    for m in msg.get("nestedType", []):
        yield from names(m, f"{path}{msg['name']}.")


def main() -> int:
    d = sys.argv[1] if len(sys.argv) > 1 else "proto"
    out = subprocess.run(["buf", "build", d, "--as-file-descriptor-set", "-o", "-#format=json"],
                         capture_output=True, text=True)
    if out.returncode:
        print(out.stderr, file=sys.stderr)
        return 2
    bad = []
    for f in json.loads(out.stdout)["file"]:
        for m in f.get("messageType", []):
            bad += [(f["name"], p) for p, n in names(m, "") if words(n) & DENY]
        for e in f.get("enumType", []):
            if words(e["name"]) & DENY:
                bad.append((f["name"], e["name"]))
        for s in f.get("service", []):
            if words(s["name"]) & DENY:
                bad.append((f["name"], s["name"]))
            bad += [(f["name"], f"{s['name']}.{m['name']}") for m in s.get("method", []) if words(m["name"]) & DENY]
    for file, name in bad:
        print(f"FAIL: {file}: `{name}` carries an identity word ({', '.join(sorted(DENY))}); "
              "contracts never carry tenant, person, project or actor identity")
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
