# 00 — 快速开始

5 分钟内跑通第一次调用。

## 前置条件

1. 拿到一个 API Key:登录 {{BRAND}} 控制台 → 「API Keys」/「密钥」→ 新建。Key 形如 `cr_xxxxxxxx...`,**仅显示一次**,务必复制保存。
2. 选一个**模型 ID**:常用如 `claude-sonnet-4-5`、`gpt-5.4`、`gemini-3-pro`,完整列表见 [02-models.md](./02-models.md) 或控制台「模型广场」。

## 一次最小调用(OpenAI 兼容)

### cURL

```bash
export NEW_API_BASE_URL="{{BASE_URL}}"
export NEW_API_KEY="cr_your_key"

curl "${NEW_API_BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${NEW_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-5.4",
    "messages": [
      {"role": "user", "content": "你好,用一句话介绍你自己"}
    ]
  }'
```

### Python

```python
import os
from openai import OpenAI

client = OpenAI(
    api_key=os.environ["NEW_API_KEY"],
    base_url=os.environ.get("NEW_API_BASE_URL", "{{BASE_URL}}") + "/v1",
)

resp = client.chat.completions.create(
    model="gpt-5.4",
    messages=[{"role": "user", "content": "你好,用一句话介绍你自己"}],
)
print(resp.choices[0].message.content)
```

### Node.js

```javascript
import OpenAI from "openai";

const client = new OpenAI({
  apiKey: process.env.NEW_API_KEY,
  baseURL: (process.env.NEW_API_BASE_URL ?? "{{BASE_URL}}") + "/v1",
});

const resp = await client.chat.completions.create({
  model: "gpt-5.4",
  messages: [{ role: "user", content: "你好,用一句话介绍你自己" }],
});

console.log(resp.choices[0].message.content);
```

## 切换模型家族

把上面 `model` 字段换成不同模型 ID,网关会自动选择对应协议:

| 模型 ID | 模型家族 | 说明 |
|---|---|---|
| `gpt-5.4` | OpenAI | OpenAI 兼容格式直接调用 |
| `claude-sonnet-4-5` | Anthropic | OpenAI 兼容格式直接调用 |
| `gemini-3-pro` | Google | **不**支持通过 `/v1/chat/completions` 调用,见下条 |

**例外**:Google Gemini 模型当前**不支持**通过 OpenAI 兼容路径访问,必须走 `/v1beta/...` 协议(见 [Gemini CLI 集成](./integrations/gemini-cli.md))。

## 流式响应

加一个 `stream: true` 即可:

```bash
curl "${NEW_API_BASE_URL}/v1/chat/completions" \
  -H "Authorization: Bearer ${NEW_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role": "user", "content": "讲个 5 句话的故事"}],
    "stream": true
  }'
```

返回的是 `text/event-stream`,每个 chunk 形如 `data: {...}\n\n`,以 `data: [DONE]\n\n` 结束。

## 下一步

- 想用 Claude Code / Codex / Gemini CLI 等工具直连?跳到 [客户端集成](./README.md#客户端集成) 或 [tako-cli 一键启动](./cli/00-quickstart.md)(推荐)
- 想用 Anthropic 原生协议?见 [Claude Code 集成](./integrations/claude-code.md)
- 报错了?见 [09-errors.md](./09-errors.md)
