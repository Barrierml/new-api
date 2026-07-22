# 通用 OpenAI 兼容工具(Cursor / Cline 等)

任何支持「自定义 API 端点」的 OpenAI 兼容客户端,都可以一行配置接入 {{BRAND}}。已知可用:Cursor、Cline、Continue、Open WebUI、ChatBox、Roo Code、LobeChat 等。

## 通用配置三件套

| 字段 | 值 |
|---|---|
| **API Base URL** | `{{BASE_URL}}/v1` |
| **API Key** | 控制台生成的 `cr_...` |
| **Model** | 任意支持的 OpenAI / Anthropic 模型 ID(Gemini 不行) |

> **Gemini 模型不能通过这条路走**,因为这些工具都打 `/v1/chat/completions`。Gemini 用专用的 [Gemini CLI 集成](./gemini-cli.md)。

## 各客户端配置位置

### Cursor

`Settings` → `Models` → 滑到底 `OpenAI API Key` 区:
- `OpenAI API Key`:填你的 Key
- `Override OpenAI Base URL`:勾上,填 `{{BASE_URL}}/v1`
- `Verify` 按钮测试通

模型选项在上方 `Model Names` 里,可手动添加 `claude-sonnet-4-5`、`gpt-5.4` 等。

### Cline(VS Code 插件)

设置面板:
- `API Provider`:选 `OpenAI Compatible`
- `Base URL`:`{{BASE_URL}}/v1`
- `API Key`:你的 Key
- `Model ID`:手填,如 `claude-sonnet-4-5`

### Continue

`config.json`:

```json
{
  "models": [
    {
      "title": "my-claude",
      "provider": "openai",
      "model": "claude-sonnet-4-5",
      "apiBase": "{{BASE_URL}}/v1",
      "apiKey": "cr_your_key"
    }
  ]
}
```

### Open WebUI / ChatBox / LobeChat

通用步骤:设置里找 `OpenAI API` 配置,把 Base URL 改成 `{{BASE_URL}}/v1`,填 Key,刷新模型列表。

## 用 cURL 自测

接入前先确认 Key + 模型可用:

```bash
curl "{{BASE_URL}}/v1/chat/completions" \
  -H "Authorization: Bearer cr_your_key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "messages": [{"role":"user","content":"hi"}]
  }'
```

返回正常的 `chat.completion` 对象就说明网关这一侧没问题,后续是客户端配置问题。

## 跨协议提醒

通过 `/v1/chat/completions` 调用 Claude 模型时,网关自动适配为 Claude 模型协议,大多数字段兼容,但需注意:

- Claude 模型 `max_tokens` 必填(网关会自动填默认值,但建议显式给)
- Claude 模型不支持 OpenAI 的 `logprobs`、`logit_bias`、`n>1`
- 工具调用语义有差异,但网关已做映射,客户端无感

## 故障排查

| 现象 | 处理 |
|---|---|
| 模型列表拉不到 | 客户端「刷新模型」按钮;或检查 Base URL 末尾是否多了斜杠 |
| `401` | Key 拼错,或客户端把 Key 发到别处了(检查抓包) |
| `404 model not found` | 模型 ID 拼错,或该 Key 没开通该模型 |
| Claude 模型回复内容被截断 | 显式设 `max_tokens` ≥ 2048 |
| 流式没流起来 | 部分客户端默认不开 `stream`,设置里手动开 |

## 相关文档

- [00 — 快速开始](../00-quickstart.md) — Chat Completions 最小调用
- [01-authentication.md](../01-authentication.md) — Key 管理
- [09-errors.md](../09-errors.md) — 错误码
