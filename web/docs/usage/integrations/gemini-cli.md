# Gemini CLI 接入 {{BRAND}}

[Gemini CLI](https://github.com/google-gemini/gemini-cli) 是 Google 官方的命令行 Gemini 客户端,通过环境变量切换到 {{BRAND}} 网关。

## 推荐:用 tako-cli 一键启动

```bash
tako --gemini
```

会自动写好 `GEMINI_API_KEY` 和 `GOOGLE_GEMINI_BASE_URL` 并启动 Gemini CLI。

---

## 手动配置(不装 tako-cli 时)

### 前置条件

- 已安装 Gemini CLI(`gemini --version` 能看到版本)
- 已在 {{BRAND}} 控制台拿到 API Key

### 环境变量配置

```bash
export GEMINI_API_KEY="cr_your_key"
export GOOGLE_GEMINI_BASE_URL="{{BASE_URL}}"
```

> **注意 `GOOGLE_GEMINI_BASE_URL` 不带 `/v1beta/`**——Gemini CLI 会自动拼 `/v1beta/models/...:generateContent`,只要给根域名即可。

写到 shell 配置里持久化:

```bash
echo 'export GEMINI_API_KEY="cr_your_key"' >> ~/.zshrc
echo 'export GOOGLE_GEMINI_BASE_URL="{{BASE_URL}}"' >> ~/.zshrc
```

### 验证

```bash
gemini --help
gemini "用一句话介绍你自己"
```

### 切换模型

```bash
gemini --model gemini-3-pro "..."
gemini --model gemini-2.5-flash "..."
```

完整可用模型见控制台或:

```bash
curl "{{BASE_URL}}/v1beta/models" \
  -H "x-goog-api-key: $GEMINI_API_KEY" | jq '.models[].name'
```

## 故障排查

| 现象 | 处理 |
|---|---|
| `401` | `echo $GEMINI_API_KEY` 看是否拼对 |
| `404 model not found` | 该 Key 没开通该 Gemini 模型,控制台核对 |
| 流式断流 | 多为本地网络/代理问题 |

## 相关文档

- [tako-cli 快速开始](../cli/00-quickstart.md) — 推荐路径
- [01-authentication.md](../01-authentication.md) — Key 管理
