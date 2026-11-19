#!/usr/bin/env python3
"""Shape checks the import rules cannot see (AST based). Run by scripts/check-architecture-python.sh.

  agent-thin-handler  (Agent and Engine) a public method of a handler class (handler/grpc/*_handler.py) has at most MAX_HANDLER_STMTS
                      statements, no loops, and calls at most ONE use case method (`self._uc.<field>.<method>(...)`).
  agent-pure-mapper   a function in a mapper/*_mapper.py (handler or infrastructure) has at most MAX_MAPPER_STMTS
                      statements: mapping is conversion only.
  agent-model-data-only  domain/model/ holds business data only: every class is a dataclass (or an Enum) with fields and no
                      methods (dunder methods aside), no module-level functions and no aliases. A token, an exception,
                      a factory or a callback type is not a model.
  agent-usecase-interface  a file in usecase/ declares ONLY a Protocol named after the file (`brief_drafter.py` ->
                      `BriefDrafter`) with method signatures, no implementation.
  agent-service-implements  a file in service/ (and service/helper/) declares the class `<File>Service` that implements
                      the use case of the same name; service/prompts/ holds pure functions and no classes.
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


def camel(stem: str) -> str:
    return "".join(w.capitalize() for w in stem.split("_"))


def is_protocol(c: ast.ClassDef) -> bool:
    return any("Protocol" in ast.unparse(b) for b in c.bases)


def bodiless(fn: ast.AST) -> bool:
    """A Protocol method: only a docstring, `...` or pass."""
    return all(isinstance(s, ast.Pass) or (isinstance(s, ast.Expr) and isinstance(s.value, ast.Constant)) for s in fn.body)


def functions(tree: ast.AST):
    for n in ast.walk(tree):
        if isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)):
            yield n


# The Agent is checked in full. The Engine's delivery layer (handler/) follows the same shape rules now; its flat domain
# and legacy modules are refactored later (see docs/harness/progress.md), so nothing else of the Engine is judged here.
SOURCES = sorted((ROOT / "agent" / "src").glob("*")) + sorted((ROOT / "blueprint-engine" / "src").glob("*"))
for src in SOURCES:
    if not src.is_dir():
        continue
    for path in sorted(src.rglob("*.py")):
        if "__pycache__" in str(path):
            continue
        tree = ast.parse(path.read_text(), str(path))
        rel = path.relative_to(src).as_posix()
        if src.parent.parent.name != "agent" and not rel.startswith("handler/"):
            continue
        if rel.startswith("domain/model/") and path.name != "__init__.py":
            for node in tree.body:
                data = isinstance(node, ast.ClassDef) and (
                    any("dataclass" in ast.unparse(d) for d in node.decorator_list)
                    or any("Enum" in ast.unparse(b) for b in node.bases))
                if isinstance(node, ast.ClassDef) and not data:
                    report("agent-model-data-only", path, f"class {node.name} is not a dataclass",
                           "domain/model/ is business data; behavior belongs in a service, a signal in errors.py, a delivery mechanism in handler/",
                           "move it to the layer that uses it")
                if data and isinstance(node, ast.ClassDef):
                    for m in node.body:
                        if isinstance(m, (ast.FunctionDef, ast.AsyncFunctionDef)) and not m.name.startswith("__"):
                            report("agent-model-data-only", path, f"{node.name}.{m.name} is a method",
                                   "a model is data; a factory or rule belongs in a service (or the caller builds the value directly)",
                                   "remove the method and construct the model with its fields")
                if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Assign, ast.AnnAssign)):
                    report("agent-model-data-only", path, "a module-level function or alias",
                           "domain/model/ is business data only (a callback type is written inline where it is used)",
                           "move it to a service, or inline the type")
        if rel.startswith(("usecase/", "usecase/helper/")) and path.name != "__init__.py":
            want = camel(path.stem)
            classes = [n for n in tree.body if isinstance(n, ast.ClassDef)]
            if [c.name for c in classes] != [want] or not is_protocol(classes[0]):
                report("agent-usecase-interface", path, f"must declare exactly the Protocol {want}",
                       "a use case is one interface named after its file; the service implements it",
                       f"declare `class {want}(Protocol)` and nothing else")
            elif any(isinstance(m, (ast.FunctionDef, ast.AsyncFunctionDef)) and not bodiless(m) for m in classes[0].body) \
                    or any(isinstance(n, (ast.FunctionDef, ast.AsyncFunctionDef)) for n in tree.body):
                report("agent-usecase-interface", path, f"{want} has an implementation",
                       "usecase/ holds interfaces only", "move the logic to service/")
        if rel.startswith("service/") and path.name != "__init__.py":
            classes = [n.name for n in tree.body if isinstance(n, ast.ClassDef)]
            if rel.startswith("service/prompts/"):
                if classes:
                    report("agent-service-implements", path, f"prompts are pure functions, found class {classes[0]}",
                           "a prompt is inputs in, text out", "use a function")
            elif camel(path.stem) + "Service" not in classes:
                report("agent-service-implements", path, f"must declare the class {camel(path.stem)}Service",
                       "a service file implements the use case of the same name", f"add `class {camel(path.stem)}Service`")
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
