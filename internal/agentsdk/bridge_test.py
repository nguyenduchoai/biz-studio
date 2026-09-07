"""Offline contracts: do not import or invoke the real SDK/API."""
import asyncio
import dataclasses
import pathlib
import contextlib
import io
import json
import runpy
import sys
import types
import unittest
from unittest.mock import patch

bridge = runpy.run_path(str(pathlib.Path(__file__).with_name("bridge.py")))


@dataclasses.dataclass
class TextBlock:
    text: str


@dataclasses.dataclass
class AssistantMessage:
    content: list


@dataclasses.dataclass
class SystemMessage:
    subtype: str
    data: dict


@dataclasses.dataclass
class ResultMessage:
    subtype: str = "success"
    is_error: bool = False
    num_turns: int = 3
    total_cost_usd: float = 0.01
    session_id: str = "test-session"
    result: str = "done"


class BridgeTests(unittest.TestCase):
    def test_normalization(self):
        result = bridge["event_dict"](AssistantMessage([TextBlock("Tiếng Việt")]))
        self.assertEqual(result["message"]["content"], [{"type": "text", "text": "Tiếng Việt"}])
        self.assertEqual(bridge["event_dict"](SystemMessage("init", {"session_id": "a"}))["session_id"], "a")
        self.assertEqual(bridge["event_dict"](ResultMessage(is_error=True))["subtype"], "error")

    def test_options_and_resume_without_network(self):
        captured = {}

        def options(**kwargs):
            captured.update(kwargs)
            return kwargs

        async def query(**kwargs):
            self.assertEqual(kwargs["prompt"], "test only")
            yield ResultMessage()

        sdk = types.SimpleNamespace(ClaudeAgentOptions=options, query=query)
        request = {"prompt": "test only", "cwd": "/project", "resume": "previous",
                   "api_key": "fake-test-key", "budget": 2, "config_dir": "/private-sdk-config"}
        events = []
        with patch.dict(sys.modules, {"claude_agent_sdk": sdk}), patch.dict("os.environ", {}, clear=True):
            asyncio.run(bridge["run"](request, events.append))
        self.assertEqual(captured["resume"], "previous")
        self.assertEqual(captured["setting_sources"], [])
        self.assertEqual(captured["permission_mode"], "dontAsk")
        self.assertEqual(captured["extra_args"], {"safe-mode": None})
        self.assertNotIn("model", captured)
        self.assertEqual(captured["max_turns"], 24)
        self.assertEqual(captured["max_budget_usd"], 2)
        self.assertEqual(events[0]["type"], "result")

    def test_key_is_redacted_and_sdk_stderr_is_discarded(self):
        secret = "fake-test-key-not-a-credential"
        request = {"prompt": "test", "cwd": "/project", "api_key": secret,
                   "budget": 2, "config_dir": "/private-sdk-config"}

        async def query(**kwargs):
            kwargs["options"]["stderr"]("request header: " + secret)
            yield AssistantMessage([TextBlock("do not log " + secret)])
            raise RuntimeError("request failed with " + secret)

        sdk = types.SimpleNamespace(ClaudeAgentOptions=lambda **kw: kw, query=query)
        out, err = io.StringIO(), io.StringIO()
        with patch.dict(sys.modules, {"claude_agent_sdk": sdk}), patch.dict("os.environ", {}, clear=True), \
             patch("sys.stdin", io.StringIO(json.dumps(request))), \
             contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
            self.assertEqual(bridge["main"](), 1)
        self.assertNotIn(secret, out.getvalue())
        self.assertNotIn(secret, err.getvalue())
        self.assertIn("[REDACTED]", out.getvalue())


if __name__ == "__main__":
    unittest.main()
