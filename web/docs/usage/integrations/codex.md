# OpenAI Codex 接入 {{BRAND}}

## 推荐:用 tako-cli 一键启动

不要自己手写 `~/.codex/config.toml`。**装一次 [tako-cli](../cli/00-quickstart.md)**,然后:

```bash
tako --codex
```

tako 会:
- 自动选好 {{BRAND}} 网关供应商和 Key
- **增量更新** `~/.codex/config.toml`(不会破坏你已有的其它 provider 配置)
- 解决 Windows 上 Codex TUI 输入失灵的问题

> Codex 内部走的是 `/v1/responses` 端点(不是 `/v1/chat/completions`),网关已支持。

---

## 手动配置(不装 tako-cli 时)

CI/CD 或远程纯 ssh 环境下,只需编辑一个配置文件 `~/.codex/config.toml`。

### 前置条件

- 已通过 `npm i -g @openai/codex` 等方式安装 Codex CLI
- 已在 {{BRAND}} 控制台拿到 API Key

### 配置文件:`~/.codex/config.toml`

```toml
model_provider = "tako"
model = "gpt-5.4"

[model_providers.tako]
name = "tako"
base_url = "{{BASE_URL}}/v1"
requires_openai_auth = true
experimental_bearer_token = "cr_your_key"
```

> Windows 路径:`%USERPROFILE%\.codex\config.toml`

`model` 字段可换成任意可用的文本模型 ID,例如 `gpt-5.4`、`deepseek-v4-pro`、`kimi-k2.6`、`glm-5.1` 等;完整列表见控制台「模型广场」或 `GET /v1/models`。Gemini 模型仍走 Google 原生端点,不放在 Codex 的 Responses 配置里。

### 验证

```bash
codex --version
codex "用 Python 实现快排"
```

### 多 provider 共存

想保留官方 OpenAI 入口,在 `config.toml` 里加多个 provider:

```toml
[model_providers.openai]
name = "openai"
base_url = "https://api.openai.com/v1"

[model_providers.tako]
name = "tako"
base_url = "{{BASE_URL}}/v1"
requires_openai_auth = true
experimental_bearer_token = "cr_your_key"
```

启动时用 `--provider tako` 切换。

> 这种多 provider 场景**强烈建议**用 tako-cli 管理,避免手动改环境变量。

### 故障排查

| 现象 | 处理 |
|---|---|
| `401` | `config.toml` 里 `experimental_bearer_token` 拼错,或本机还残留旧的 Codex 环境变量/配置 |
| `model_not_found` | 该 Key 没开通该模型,或 `config.toml` 里 `model` 拼错 |
| `connection refused` | `base_url` 末尾必须有 `/v1`,且不能带尾斜杠之外的多余路径 |
| TOML 解析错 | 看是不是缺了引号或括号匹配 |

## 相关文档

- [tako-cli 快速开始](../cli/00-quickstart.md) — 推荐路径
- [01-authentication.md](../01-authentication.md) — Key 管理
- [09-errors.md](../09-errors.md) — 错误码
