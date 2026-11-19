#!/usr/bin/env python3
"""Shape checks the import rules cannot see (AST based). Run by scripts/check-architecture-python.sh.

  agent-thin-handler  a public method of a handler class (handler/grpc/*_handler.py) has at most MAX_HANDLER_STMTS
                      statements, no loops, and calls at most ONE use case method (`self._uc.<field>.<method>(...)`).
  agent-pure-mapper   a function in a mapper/*_mapper.py (handler or infrastructure) has at most MAX_MAPPER_STMTS
                      statements: mapping is conversion only.
  agent-small-function  any function in the Agent has at most MAX_FUNCTION_STMTS statements (read it in one screen).

ROOT (env) is the repository root; output matches the shell checks (FAIL / why / fix) and the exit code is 1 on a violation.
"""

from __future__ import annotations

import ast
import os
import sys
from pathlib import Path

MAX_HANDLER_STMTS = 15
MAX_MAPPER_STMTS = 25
MAX_FUNCTION_STMTS = 60

ROOT = Path(os.environ.get("ROOT", Path(__file__).resolve().parent.parent))
failed = False


def report(rule: str, path: Path, what: str, why: str, fix: str) -> None:
    global failed
    failed = True
    print(f"\nFAIL: [{rule}] {path.relative_to(ROOT)}: {what}\n  why: {why}\n  fix: {fix}")


def stmts(node: ast.AST) -> int:
    return sum(isinstance(n, ast.stmt) for n in ast.walk(node)) - 1  # minus the def itself


def usecase_calls(fn: ast.AST) -> set[str]:
    """Distinct `<field>.<method>` of calls rooted at `self._uc`."""
    found = set()
    for n in ast.walk(fn):
        if not (isinstance(n, ast.Call) and isinstance(n.func, ast.Attribute)):
            continue
        chain, cur = [], n.func
        while isinstance(cur, ast.Attribute):
            chain.append(cur.attr)
            cur = cur.value
        if isinstance(cur, ast.Name) and cur.id == "self" and chain and chain[-1] == "_uc":
            found.add(".".join(reversed(chain[:-1])))
    return found


def functions(tree: ast.AST):
    for n in ast.walk(tree):
        if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)):
            yield n


for src in sorted((ROOT / "agent" / "src").glob("*")):
    if not src.is_dir():
        continue
    for path in sorted(src.rglob("*.py")):
        if "/gen/" in str(path) or "__pycache__" in str(path):
            continue
        tree = ast.parse(path.read_text(), str(path))
        rel = path.relative_to(src).as_posix()
        for fn in functions(tree):
            n = stmts(fn)
            if n > MAX_FUNCTION_STMTS:
                report("agent-small-function", path, f"{fn.name} has {n} statements (max {MAX_FUNCTION_STMTS})",
                       "a function that does not fit one screen hides what it does", "split it into named steps")
            if "/mapper/" in "/" + rel and path.name.endswith("_mapper.py") and n > MAX_MAPPER_STMTS:
                report("agent-pure-mapper", path, f"{fn.name} has {n} statements (max {MAX_MAPPER_STMTS})",
                       "a mapper only converts between a DTO and a model", "move the logic to a service")
        if rel.startswith("handler/grpc/") and path.name.endswith("_handler.py"):
            for cls in (c for c in ast.walk(tree) if isinstance(c, ast.ClassDef)):
                for m in cls.body:
                    if not isinstance(m, (ast.FunctionDef, ast.AsyncFunctionDef)) or m.name.startswith("_"):
                        continue
                    n = stmts(m)
                    if n > MAX_HANDLER_STMTS:
                        report("agent-thin-handler", path, f"{cls.name}.{m.name} has {n} statements (max {MAX_HANDLER_STMTS})",
                               "a handler decodes, calls one use case and encodes", "move the logic to a service")
                    if any(isinstance(x, (ast.For, ast.While)) for x in ast.walk(m)):
                        report("agent-thin-handler", path, f"{cls.name}.{m.name} loops",
                               "iteration over business data belongs in a service or a mapper", "move the loop out")
                    calls = usecase_calls(m)
                    if len(calls) > 1:
                        report("agent-thin-handler", path, f"{cls.name}.{m.name} calls {len(calls)} use cases ({', '.join(sorted(calls))})",
                               "a handler calls exactly one use case; orchestration belongs in a service",
                               "add a use case that does the orchestration")

sys.exit(1 if failed else 0)
