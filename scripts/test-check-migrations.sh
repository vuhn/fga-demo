#!/usr/bin/env bash
# Tests for check-migrations.py: each case is a migration Up body; want = pass | fail.
cd "$(dirname "$0")/.."
fail=0; n=0
run() { # want, sql [, header]
  d=$(mktemp -d); n=$((n+1))
  printf -- '%s\n-- +goose Up\n%s\n-- +goose Down\nSELECT 1;\n' "${3:-}" "$2" > "$d/00099_t.sql"
  python3 scripts/check-migrations.py "$d" >/dev/null 2>&1 && got=pass || got=fail
  [ "$got" = "$1" ] || { echo "FAIL want $1 got $got: $2"; fail=1; }
  rm -rf "$d"
}
for s in "ALTER TABLE b DROP COLUMN x;" "DROP TABLE b;" "ALTER TABLE b RENAME TO c;" "ALTER TABLE b RENAME COLUMN x TO y;" \
  "ALTER TABLE b ALTER COLUMN x SET NOT NULL;" "ALTER TABLE b ALTER COLUMN x DROP NOT NULL;" "ALTER TYPE t ADD VALUE 'x';" \
  "ALTER TABLE b ALTER COLUMN name TYPE varchar(10);" "ALTER TABLE b ALTER COLUMN name TYPE text USING name;" \
  "ALTER TABLE b ALTER COLUMN name DROP DEFAULT;" "ALTER TABLE b ADD COLUMN q text NOT NULL;" \
  "ALTER TABLE b
  ADD COLUMN q text
  NOT NULL;" "ALTER TABLE b ADD COLUMN ok int, ADD COLUMN q text NOT NULL;" \
  "ALTER TABLE b ADD CONSTRAINT x CHECK (name <> '');" "ALTER TABLE b ADD CONSTRAINT u UNIQUE (name);" \
  "ALTER TABLE b ADD FOREIGN KEY (c) REFERENCES users (id);" "ALTER TABLE b DROP CONSTRAINT x;" "DROP INDEX i;" \
  "CREATE UNIQUE INDEX z ON b (name);" "CREATE INDEX z ON b (name);" "DELETE FROM b;" "TRUNCATE b;" "UPDATE b SET name = 'x';"; do
  run fail "$s"
done
run pass "ALTER TABLE b ADD COLUMN q text;"
run pass "ALTER TABLE b ADD COLUMN q text NOT NULL DEFAULT '';"
run pass "ALTER TABLE b ADD CONSTRAINT x CHECK (name <> '') NOT VALID;"
run pass "ALTER TABLE b VALIDATE CONSTRAINT x;"
run pass "ALTER TABLE b ADD CONSTRAINT u UNIQUE USING INDEX z;"
run pass "CREATE INDEX CONCURRENTLY z ON b (name);" "-- +goose NO TRANSACTION"
run fail "CREATE INDEX CONCURRENTLY z ON b (name);"
run pass "INSERT INTO b (id) VALUES (1);"
run pass "CREATE TABLE n (id uuid PRIMARY KEY, b uuid);
ALTER TABLE n ADD FOREIGN KEY (b) REFERENCES b (id);
CREATE UNIQUE INDEX n_b_key ON n (b);
UPDATE n SET b = NULL;"
run pass "-- force-allow-breaking: demo table, 3 rows
DELETE FROM b;"
run pass "DELETE FROM b; -- force-allow-breaking: demo table"
run fail "-- force-allow-breaking:
DELETE FROM b;"
run fail "-- force-allow-breaking: only the next statement
DELETE FROM b;
TRUNCATE c;"
run fail "-- breaking-ok: OLDNAME no longer works
DELETE FROM b;"
d=$(mktemp -d); printf -- '-- +goose Up\nSELECT 1;\n' > "$d/00099_t.sql"
python3 scripts/check-migrations.py "$d" >/dev/null 2>&1 && { echo "FAIL missing Down passed"; fail=1; }; rm -rf "$d"
python3 scripts/check-migrations.py >/dev/null || { echo "FAIL the real migrations do not pass"; fail=1; }
[ $fail = 0 ] && echo "check-migrations: $n cases ok"
exit $fail
