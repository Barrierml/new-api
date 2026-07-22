# 02 — 获取模型列表

## 端点

{{BRAND}} 同时提供 OpenAI 风格和 Google 风格的模型列表端点。

| 端点 | 风格 | 鉴权 |
|---|---|---|
| `GET /v1/models` | OpenAI(带 `x-api-key` + `anthropic-version` 时按 Anthropic 风格返回) | 需要 Key;不带 Key 时只返回公开列表 |
| `GET /v1beta/models` | Google AI | `x-goog-api-key` 或 `Authorization: Bearer` |

## OpenAI 风格

```bash
curl "{{BASE_URL}}/v1/models" \
  -H "Authorization: Bearer cr_your_key"
```

返回:

```json
{
  "object": "list",
  "data": [
    {
      "id": "gpt-5.4",
      "object": "model",
      "created": 1730000000,
      "owned_by": "openai"
    },
    {
      "id": "claude-sonnet-4-5",
      "object": "model",
      "created": 1730000000,
      "owned_by": "anthropic"
    }
  ]
}
```

## Google 风格

```bash
curl "{{BASE_URL}}/v1beta/models" \
  -H "x-goog-api-key: cr_your_key"
```

返回:

```json
{
  "models": [
    {
      "name": "models/gemini-3-pro",
      "supportedGenerationMethods": ["generateContent", "streamGenerateContent"]
    }
  ]
}
```

## SDK 示例

### Python(OpenAI SDK)

```python
from openai import OpenAI
client = OpenAI(api_key="cr_...", base_url="{{BASE_URL}}/v1")
for m in client.models.list().data:
    print(m.id)
```

### Node.js

```javascript
import OpenAI from "openai";
const client = new OpenAI({ apiKey: "cr_...", baseURL: "{{BASE_URL}}/v1" });
const list = await client.models.list();
for (const m of list.data) console.log(m.id);
```

## 模型 ID 命名规则

模型 ID 沿用各家族官方命名,**不**强制加 vendor 前缀:

| 模型家族 | 形式举例 |
|---|---|
| OpenAI | `gpt-5.4`、`gpt-4.1`、`o1-pro` |
| Anthropic | `claude-sonnet-4-5`、`claude-opus-4-7`、`claude-haiku-4-5` |
| Google | `gemini-3-pro`、`gemini-2.5-flash` |

## 路由规则速记

- `claude-*` → 可通过 OpenAI Chat Completions(`POST /v1/chat/completions`)或 Anthropic Messages(`POST /v1/messages`)调用
- `gpt-*`、`o1-*` → OpenAI Chat Completions 或 Anthropic Messages
- `gemini-*` → 走 Gemini 协议(**只能**通过 `/v1beta/...` 调用)

## 当前可用模型

实时清单以控制台「模型广场」(`/pricing`)或上面的 `GET /v1/models` 为准。本目录出现的具体模型 ID 只是示例。

## Tips

- **列表 ≠ 调用权限**:`/v1/models` 返回该 Key 可见的模型;若某模型在列表里但调用 403,说明当前分组未开通该模型
- **缓存友好**:`/v1/models` 响应可在客户端缓存几分钟
- **不要**通过模型列表去推断价格,价格请看控制台「模型广场」
