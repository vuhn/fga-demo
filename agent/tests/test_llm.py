"""The Ollama client (copy of the Engine-era client): thinking budgets, JSON retry, the LLM boundary."""

import json

import httpx
import pytest
from fake_llm import FakeLLM

from fga_agent.domain import progress
from fga_agent.domain.errors import LLMError
from fga_agent.infrastructure.ollama import OllamaClient

SCHEMA = {"type": "object", "properties": {"ok": {"type": "boolean"}}}


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

    def handler(req):
        body = json.loads(req.content)
        calls.append(body)
        if body["think"]:
            return _ndjson([{"message": {"thinking": "hmm, wait, let me think again. "}} for _ in range(5000)])
        return httpx.Response(200, json={"message": {"content": '{"ok": true}'}})

    events = []
    with progress.sink(events.append):
        out = _client(handler, thinking_max_chars=500).structured("sys", "user", SCHEMA)
    assert out == {"ok": True}
    assert [c["think"] for c in calls] == [True, False]
    assert any(e.get("stage") == "retry" for e in events)


def test_thinking_within_budget_is_used_and_shown():
    events = []

    def handler(req):
        return _ndjson([{"message": {"thinking": "short plan"}}, {"message": {"content": '{"ok": true}'}}])

    with progress.sink(events.append):
        assert _client(handler).structured("sys", "user", SCHEMA) == {"ok": True}
    assert any(e["kind"] == "thinking" and "short plan" in e["message"] for e in events)


def test_unparseable_json_is_retried_once_with_the_error_then_fails():
    seen = []

    def handler(req):
        seen.append(json.loads(req.content)["messages"])
        return httpx.Response(200, json={"message": {"content": "not json"}})

    with pytest.raises(LLMError, match="invalid JSON twice"):
        _client(handler).structured("sys", "user", SCHEMA)
    assert len(seen) == 2 and "invalid" in seen[1][-1]["content"]


def test_an_unreachable_ollama_is_an_llm_error():
    def handler(req):
        raise httpx.ConnectError("refused")

    with pytest.raises(LLMError, match="cannot reach Ollama"):
        _client(handler).structured("sys", "user", SCHEMA)


def test_fake_llm_scripts_answers_and_text():
    llm = FakeLLM([{"a": 1}, LLMError("down")], texts=["hello world"])
    assert llm.structured("s", "u", {}) == {"a": 1}
    with pytest.raises(LLMError):
        llm.structured("s", "u", {})
    assert "".join(llm.stream_text("s", "u")) == "hello world"
