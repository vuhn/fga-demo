"""The Ollama client (copy of the Engine-era client): thinking budgets, JSON retry, the LLM boundary."""

import json

import httpx
import pytest
from fake_language_model import FakeLanguageModel

from fga_agent.domain.errors import LlmUnavailable
from fga_agent.infrastructure.ollama import OllamaLanguageModel

SCHEMA = {"type": "object", "properties": {"ok": {"type": "boolean"}}}


class Recorder:
    def __init__(self):
        self.events = []

    def step(self, stage, message):
        self.events.append(("step", stage))

    def thinking(self, text):
        self.events.append(("thinking", text))

    def event(self, name, data):
        self.events.append((name, data))


def _client(handler, **env) -> OllamaLanguageModel:
    c = OllamaLanguageModel(base_url="http://ollama.test")
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

    rep = Recorder()
    out = _client(handler, thinking_max_chars=500).answer_json("sys", "user", SCHEMA, progress=rep)
    assert out == {"ok": True}
    assert [c["think"] for c in calls] == [True, False]
    assert any(e[0] == "step" and e[1] == "retry" for e in rep.events)


def test_thinking_within_budget_is_used_and_shown():
    rep = Recorder()

    def handler(req):
        return _ndjson([{"message": {"thinking": "short plan"}}, {"message": {"content": '{"ok": true}'}}])

    assert _client(handler).answer_json("sys", "user", SCHEMA, progress=rep) == {"ok": True}
    assert any(e[0] == "thinking" and "short plan" in e[1] for e in rep.events)


def test_nobody_listening_means_no_thinking_stream():
    bodies = []

    def handler(req):
        bodies.append(json.loads(req.content))
        return httpx.Response(200, json={"message": {"content": '{"ok": true}'}})

    _client(handler).answer_json("sys", "user", SCHEMA)
    assert [b["think"] for b in bodies] == [False]


def test_unparseable_json_is_retried_once_with_the_error_then_fails():
    seen = []

    def handler(req):
        seen.append(json.loads(req.content)["messages"])
        return httpx.Response(200, json={"message": {"content": "not json"}})

    with pytest.raises(LlmUnavailable, match="invalid JSON twice"):
        _client(handler).answer_json("sys", "user", SCHEMA)
    assert len(seen) == 2 and "invalid" in seen[1][-1]["content"]


def test_an_unreachable_ollama_is_an_llm_error():
    def handler(req):
        raise httpx.ConnectError("refused")

    with pytest.raises(LlmUnavailable, match="cannot reach Ollama"):
        _client(handler).answer_json("sys", "user", SCHEMA)


def test_fake_language_model_scripts_answers_and_text():
    llm = FakeLanguageModel([{"a": 1}, LlmUnavailable("down")], texts=["hello world"])
    assert llm.answer_json("s", "u", {}) == {"a": 1}
    with pytest.raises(LlmUnavailable):
        llm.answer_json("s", "u", {})
    assert "".join(llm.answer_text("s", "u")) == "hello world"
