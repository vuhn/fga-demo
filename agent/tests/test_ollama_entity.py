"""The Ollama wire entities."""

from fga_agent.infrastructure.entity.ollama_chat import ChatChunk, ChatMessage, ChatRequest


def test_request_body_matches_the_ollama_chat_api():
    body = ChatRequest("qwen3", [ChatMessage("system", "s"), ChatMessage("user", "u")], stream=True, think=True,
                       schema={"type": "object"}, max_tokens=50).body()
    assert body == {"model": "qwen3", "messages": [{"role": "system", "content": "s"}, {"role": "user", "content": "u"}],
                    "stream": True, "think": True, "format": {"type": "object"},
                    "options": {"temperature": 0, "num_ctx": 8192, "num_predict": 50}}


def test_no_schema_means_no_format_and_no_token_cap():
    body = ChatRequest("m", [], stream=False, think=False, temperature=0.3).body()
    assert "format" not in body and body["options"] == {"temperature": 0.3, "num_ctx": 8192}


def test_chunks_read_content_and_thinking_and_tolerate_missing_fields():
    assert ChatChunk.from_line('{"message": {"thinking": "hm", "content": ""}}') == ChatChunk("", "hm")
    assert ChatChunk.from_line(b'{"message": {"content": "{}"}}') == ChatChunk("{}", "")
    assert ChatChunk.from_json({"done": True}) == ChatChunk()
