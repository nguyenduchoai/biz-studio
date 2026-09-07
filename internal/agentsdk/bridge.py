"""Private stdin/NDJSON adapter. No API calls until explicitly invoked by a user."""
import asyncio
import dataclasses
import json
import os
import sys

BLOCK_TYPES = {
    "TextBlock": "text", "ThinkingBlock": "thinking", "ToolUseBlock": "tool_use",
    "ToolResultBlock": "tool_result",
}


def block_dict(block):
    if isinstance(block, dict):
        return block
    result = dataclasses.asdict(block)
    result["type"] = BLOCK_TYPES.get(type(block).__name__, "unknown")
    return result


def event_dict(message):
    name = type(message).__name__
    if name == "SystemMessage":
        return {**message.data, "type": "system", "subtype": message.subtype}
    if name in ("AssistantMessage", "UserMessage"):
        content = message.content
        if isinstance(content, list):
            content = [block_dict(item) for item in content]
        return {"type": "assistant" if name == "AssistantMessage" else "user",
                "message": {"content": content}}
    if name == "ResultMessage":
        result = dataclasses.asdict(message)
        result["type"] = "result"
        if message.is_error and result.get("subtype") == "success":
            result["subtype"] = "error"
        return result
    return None


async def run(request, emit):
    from claude_agent_sdk import ClaudeAgentOptions, query

    # Separate SDK session storage and disable user/project settings and hooks.
    # Model is deliberately omitted: the provider chooses its current default.
    os.environ["CLAUDE_CONFIG_DIR"] = request["config_dir"]
    options = ClaudeAgentOptions(
        cwd=request["cwd"],
        resume=request.get("resume") or None,
        setting_sources=[],
        permission_mode="dontAsk",
        allowed_tools=["Read", "Write", "Edit", "Glob", "Grep", "Bash(ffmpeg *)",
                       "Bash(ffprobe *)", "Bash(mkdir *)", "Bash(cp *)", "Bash(mv *)"],
        disallowed_tools=["WebFetch", "WebSearch"],
        extra_args={"safe-mode": None},
        stderr=lambda _line: None,
        max_turns=24,
        max_budget_usd=request["budget"],
        env={"ANTHROPIC_API_KEY": request["api_key"],
             "CLAUDE_CONFIG_DIR": request["config_dir"]},
    )
    got_result = False
    async for message in query(prompt=request["prompt"], options=options):
        event = event_dict(message)
        if event is not None:
            emit(event)
            got_result = got_result or event["type"] == "result"
    if not got_result:
        raise RuntimeError("missing result")


def main():
    secret = ""
    try:
        request = json.load(sys.stdin)
        secret = request["api_key"].strip()
        if not secret:
            raise ValueError("missing API key")

        def emit(event):
            text = json.dumps(event, ensure_ascii=False)
            # A tool/provider error must not echo the credential into stored logs.
            print(text.replace(secret, "[REDACTED]"), flush=True)

        asyncio.run(run(request, emit))
    except Exception as exc:
        # SDK exceptions may contain request headers: never persist their text.
        print(json.dumps({"type": "result", "subtype": "error", "is_error": True,
                          "result": "Agent SDK thất bại (" + type(exc).__name__ +
                          "). Kiểm tra API key, hạn mức, mạng hoặc cài lại SDK."}), flush=True)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
