"""A thinking stream that never ends (a repetition loop) is cut by the budget and the call is redone
without thinking; a stream within budget is returned as before."""

import json

import httpx
from pydantic import BaseModel

from fga_engine import progress
from fga_engine.llm import OllamaClient


class Out(BaseModel):
    ok: bool


def _client(handler, **env) -> OllamaClient:
    c = OllamaClient(base_url="http://ollama.test")
    c.http = httpx.Client(transport=httpx.MockTransport(handler))
    for k, v in env.items():
        setattr(c, k, v)
    return c


def _ndjson(chunks):
    return httpx.Response(200, content="".join(json.dumps(c) + "\n" for c in chunks).encode())


def test_endless_thinking_is_cut_and_redone_without_thinking():
    calls = []

    def handler(req: httpx.Request) -> httpx.Response:
        body = json.loads(req.content)
        calls.append(body)
        if body["think"]:  # an endless loop: far more reasoning than the budget, never an answer
            return _ndjson([{"message": {"thinking": "hmm, wait, let me think again. "}} for _ in range(5000)])
        return httpx.Response(200, json={"message": {"content": '{"ok": true}'}})

    events = []
    with progress.sink(events.append):
        out = _client(handler, thinking_max_chars=500).structured("sys", "user", Out)
    assert out.ok is True
    assert [c["think"] for c in calls] == [True, False]
    assert any(e.get("stage") == "retry" for e in events)


def test_thinking_within_budget_is_used_and_not_redone():
    calls = []

    def handler(req: httpx.Request) -> httpx.Response:
        calls.append(json.loads(req.content))
        return _ndjson([{"message": {"thinking": "short plan"}}, {"message": {"content": '{"ok": true}'}}])

    with progress.sink(lambda e: None):
        out = _client(handler).structured("sys", "user", Out)
    assert out.ok is True and len(calls) == 1


def test_time_budget_also_cuts_the_stream():
    def handler(req: httpx.Request) -> httpx.Response:
        if json.loads(req.content)["think"]:
            return _ndjson([{"message": {"thinking": "x"}}] * 10)
        return httpx.Response(200, json={"message": {"content": '{"ok": true}'}})

    with progress.sink(lambda e: None):
        assert _client(handler, thinking_max_seconds=-1).structured("sys", "user", Out).ok
