# {{BRAND}} 使用文档

{{BRAND}} 是一个统一 AI 模型 API 网关,通过单一域名访问 OpenAI / Anthropic Claude / Google Gemini 三大家族的模型。本目录是面向**调用方**(开发者、AI 编码工具、第三方应用)的使用文档。

> 部署/运维相关请联系管理员。本文只讲「怎么调用」。

## BASE_URL 约定

全文示例用 `{{BASE_URL}}` 占位,表示你访问本网关的根域名,例如:

- `https://api.example.com`(线上公开实例)
- `http://localhost:3000`(本地开发)

页面渲染时会自动替换成当前站点的真实地址。把它代入下面的示例即可。

## 三种协议入口一览

| 协议 | 路径前缀 | 认证 header | 适用客户端 |
|---|---|---|---|
| OpenAI 兼容 | `/v1/...` | `Authorization: Bearer <key>` | Codex、Cursor、Cline、所有 OpenAI SDK |
| Anthropic 原生 | `/v1/messages` | `x-api-key: <key>` 或 `Authorization: Bearer <key>` | Claude Code、Anthropic SDK |
| Google 原生 | `/v1beta/models/...` | `x-goog-api-key: <key>` 或 `Authorization: Bearer <key>` | Gemini CLI、Google AI SDK |

同一个 API Key 可同时调用三种协议的端点(具体取决于 token 分组配置,见 [01-authentication.md](./01-authentication.md))。

## 文档目录

### 快速开始
- [00 — 快速开始](./00-quickstart.md)

### tako-cli(推荐)
- [tako-cli 快速开始](./cli/00-quickstart.md) — 装 `tako` 一键启动 Claude Code / Codex / Gemini CLI

### API 参考
- [01 — 身份认证](./01-authentication.md)
- [02 — 获取模型列表](./02-models.md)
- [09 — 错误处理](./09-errors.md)

### 客户端集成
- [Claude Code](./integrations/claude-code.md)
- [OpenAI Codex](./integrations/codex.md)
- [Gemini CLI](./integrations/gemini-cli.md)
- [OpenAI 兼容工具(Cursor / Cline 等)](./integrations/openai-compat.md)

## 模型列表

可用模型以控制台「模型广场」(`/pricing`)或 `GET /v1/models` 实时返回为准。本目录出现的具体模型 ID(如 `claude-sonnet-4-5`、`gpt-5.4`、`gemini-3-pro`)只是示例。
