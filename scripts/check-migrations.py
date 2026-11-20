#!/usr/bin/env python3
"""Backward-compatibility lint for goose migrations (see .claude/skills/db-design).

Reads whole statements of the Up section (not lines) of every migration numbered above BASELINE. A statement
that would break the previous build while it still runs is rejected unless a comment on it, or on the line
before it, says `-- force-allow-breaking: <reason>` (the reason is required and belongs in
specs/<feature>/db-design.md). Tables created by the same migration are new, so changing them is free.

usage: check-migrations.py [migrations-dir]
"""
import re
import sys
from pathlib import Path

BASELINE = 11
FLAG = re.compile(r"--\s*force-allow-breaking:\s*(\S.*)")
FLAG_EMPTY = re.compile(r"--\s*force-allow-breaking\s*:?\s*$")


def statements(up: str):
    """Yield (code, comments, first_line) for each statement; a goose StatementBegin..End block is one."""
    code, comments, first, block = [], [], None, False
    for no, raw in enumerate(up.splitlines(), 1):
        s = raw.strip()
        if s.startswith("-- +goose StatementBegin"):
            block = True
            continue
        if s.startswith("-- +goose StatementEnd"):
            block = False
            if code:
                yield " ".join(code), comments, first
                code, comments, first = [], [], None
            continue
        body, _, comment = raw.partition("--")
        if comment:
            comments.append("--" + comment)
        if body.strip():
            first = first or no
            code.append(body.strip())
        if not block and body.strip().endswith(";") and code:
            yield " ".join(code), comments, first
            code, comments, first = [], [], None
    if code:
        yield " ".join(code), comments, first


def rules(sql: str, new_tables: set, concurrently_ok: bool):
    """Return (why, fix) pairs for one statement (upper-cased, whitespace collapsed)."""
    u = re.sub(r"\s+", " ", sql.upper()).strip().rstrip(";")
    out = []
    add = lambda why, fix: out.append((why, fix))
    m = re.match(r'(?:ALTER TABLE|CREATE (?:UNIQUE )?INDEX(?: CONCURRENTLY)?(?: IF NOT EXISTS)?(?: \S+)? ON)(?: ONLY)?(?: IF EXISTS)? "?([A-Z0-9_.]+)"?', u)
    table = m.group(1).lower() if m else None
    if table and table.lower() in new_tables:
        return out
    if re.search(r"\bDROP (COLUMN|TABLE)\b", u) or u.startswith("DROP TABLE"):
        add("dropping something old code may still read breaks the running release", "expand first, drop in a later release once nothing reads it")
    if re.search(r"\bRENAME\b", u):
        add("renaming breaks old code during rollout", "add the new name, backfill, drop the old one in a later release")
    if re.search(r"\bSET NOT NULL\b", u):
        add("existing or in-flight rows may be NULL", "backfill, add CHECK (col IS NOT NULL) NOT VALID, VALIDATE it, then SET NOT NULL in a later release")
    if re.search(r"\bDROP NOT NULL\b", u):
        add("NOT NULL -> NULL breaks readers that assume a value", "keep NOT NULL, or add a new column")
    if re.search(r"\bALTER TYPE\b|\bADD VALUE\b|\bDROP TYPE\b", u) and "ALTER COLUMN" not in u:
        add("changing enum/type values breaks old code", "use a new nullable column or lookup table")
    if re.search(r"\bALTER COLUMN \S+ (SET DATA )?TYPE\b", u):
        add("changing a column type rewrites the table and can narrow values old code writes", "add a new column of the new type, backfill in batches, switch, drop later")
    if re.search(r"\bALTER COLUMN \S+ DROP DEFAULT\b", u):
        add("old inserts that rely on the default start failing", "keep the default until no build relies on it")
    for clause in re.split(r",\s*(?=ADD COLUMN)", u):
        if re.search(r"\bADD COLUMN\b", clause) and "NOT NULL" in clause and "DEFAULT" not in clause:
            add("new NOT NULL column without DEFAULT breaks inserts from old code", "add a DEFAULT, or add nullable then backfill")
    if re.search(r"\bDROP CONSTRAINT\b", u):
        add("dropping a constraint removes a guard the running build relies on", "drop it in a later release once nothing depends on it")
    if u.startswith("DROP INDEX"):
        add("dropping an index the running build's queries use can make them slow or time out", "drop it in a later release once no query needs it")
    if re.search(r"\bADD (CONSTRAINT \S+ )?(CHECK|FOREIGN KEY|UNIQUE|PRIMARY KEY|EXCLUDE)\b|\bADD CONSTRAINT\b", u) and "NOT VALID" not in u and "USING INDEX" not in u:
        add("a new constraint is checked against existing rows and locks the table while it scans", "ADD CONSTRAINT ... NOT VALID, then VALIDATE CONSTRAINT in a later step (UNIQUE: CREATE UNIQUE INDEX CONCURRENTLY, then ADD CONSTRAINT ... USING INDEX)")
    if re.match(r"CREATE (UNIQUE )?INDEX", u):
        if " CONCURRENTLY" not in u:
            add("CREATE INDEX blocks writes to the table while it builds", "CREATE INDEX CONCURRENTLY with `-- +goose NO TRANSACTION`")
        elif not concurrently_ok:
            add("CREATE INDEX CONCURRENTLY fails inside a transaction", "add `-- +goose NO TRANSACTION` at the top of the file")
        if u.startswith("CREATE UNIQUE") and " CONCURRENTLY" not in u:
            add("a UNIQUE index fails on existing duplicates and rejects old-build writes", "check the data first and create it CONCURRENTLY")
    if re.match(r"(DELETE FROM|TRUNCATE|UPDATE) ", u):
        t = re.match(r"(?:DELETE FROM|TRUNCATE(?: TABLE)?|UPDATE)(?: ONLY)? \"?([A-Z0-9_.]+)", u)
        if not (t and t.group(1).lower() in new_tables):
            add("a bulk data change in the schema transaction locks rows and may destroy data the running build uses", "backfill in a separate batched step; never delete data in an Up")
    if re.match(r"DROP (SCHEMA|VIEW|FUNCTION|TRIGGER|SEQUENCE)\b", u) or re.search(r"\bSET SCHEMA\b", u):
        add("dropping or moving an object the running build may use breaks it", "expand first, remove in a later release")
    return out


def lint(path: Path) -> list:
    text = path.read_text()
    up, sep, _down = text.partition("-- +goose Down")
    problems = []
    if not sep:
        problems.append(("no '-- +goose Down' section", "migrations must be reversible or say why not",
                         "add a Down section (an explicit no-op with a comment if irreversible)"))
    no_tx = "+goose NO TRANSACTION" in up
    new_tables = {m.group(1).lower() for m in re.finditer(r'(?im)^\s*CREATE TABLE(?: IF NOT EXISTS)? "?([a-z0-9_.]+)', up)}
    carry = []
    for code, comments, line in statements(up):
        comments = carry + comments
        carry = []
        flagged = any(FLAG.search(c) for c in comments)
        if any(FLAG_EMPTY.search(c) for c in comments) and not flagged:
            problems.append((f"line {line}: force-allow-breaking has no reason", "the flag is a justification",
                             "write `-- force-allow-breaking: <reason>` and justify it in db-design.md"))
        if flagged:
            continue
        for why, fix in rules(code, new_tables, no_tx):
            problems.append((f"line {line}: {' '.join(code.split())[:110]}", why,
                             f"{fix}, or mark it `-- force-allow-breaking: <reason>` and justify it in db-design.md"))
    return problems


def main(argv):
    root = Path(argv[1]) if len(argv) > 1 else Path(__file__).resolve().parent.parent / "api" / "migrations"
    bad = 0
    for f in sorted(root.glob("[0-9]*.sql")):
        if int(f.name.split("_")[0]) <= BASELINE:
            continue
        for what, why, fix in lint(f):
            print(f"\nFAIL: {f}: {what}\n  why: {why}\n  fix: {fix}")
            bad = 1
    if not bad:
        print("migrations: ok")
    return bad


if __name__ == "__main__":
    sys.exit(main(sys.argv))
