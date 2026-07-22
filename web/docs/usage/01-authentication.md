# 01 — 身份认证

{{BRAND}} 网关上所有业务 API 请求都需要携带 API Key。

## 获取 API Key

1. 登录 {{BRAND}} 控制台(`{{BASE_URL}}`)
2. 进入「API Keys」(或「密钥管理」)页面
3. 点「新建」,填写名称(可选)、有效期、额度
4. **立即复制** Key 字符串,关闭弹窗后无法再次查看
5. Key 格式形如 `cr_` 开头

> **安全提醒**:Key 等同密码,泄露后请立即在控制台禁用并新建。不要写入任何前端 / 移动端 / 提交到 git 的代码。建议存到环境变量或秘密管理服务。

## 三种认证 Header

{{BRAND}} 同时支持三种主流 SDK 的鉴权习惯,**同一个 Key 在三种 header 下都成立**:

| Header | 主要使用方 | 端点 |
|---|---|---|
| `Authorization: Bearer <key>` | OpenAI SDK、通用 HTTP 客户端 | 全部 |
| `x-api-key: <key>` | Anthropic 官方 SDK 默认 | `/v1/messages` 等 Anthropic 端点 |
| `x-goog-api-key: <key>` | Google AI SDK 默认 | `/v1beta/...` 等 Google 端点 |

请求到达网关后,网关会:
1. 从上述任一 header 中提取 Key
2. 校验 Key 有效性(额度、模型/分组权限)
3. 在网关层完成鉴权后处理请求

> 你的 Key 仅在网关层校验,**不会**以原样透传给第三方模型服务。

## 示例

### OpenAI 路径(Bearer)

```bash
curl "{{BASE_URL}}/v1/chat/completions" \
  -H "Authorization: Bearer cr_your_key" \
  -H "Content-Type: application/json" \
  -d '{"model":"gpt-5.4","messages":[{"role":"user","content":"hi"}]}'
```

### Anthropic 路径(x-api-key)

```bash
curl "{{BASE_URL}}/v1/messages" \
  -H "x-api-key: cr_your_key" \
  -H "anthropic-version: 2023-06-01" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-sonnet-4-5",
    "max_tokens": 1024,
    "messages": [{"role":"user","content":"hi"}]
  }'
```

### Google 路径(x-goog-api-key)

```bash
curl "{{BASE_URL}}/v1beta/models/gemini-3-pro:generateContent" \
  -H "x-goog-api-key: cr_your_key" \
  -H "Content-Type: application/json" \
  -d '{
    "contents": [{"role":"user","parts":[{"text":"hi"}]}]
  }'
```

## 多 Key 与配额

控制台支持:
- **多 Key**:每个用户可建多把 Key,分别给不同应用/同事
- **独立配额**:每把 Key 可设独立的额度上限和过期时间
- **用量统计**:控制台「日志」/「用量」页可看每把 Key 的调用次数、token 消耗、成本明细

## 常见错误

| HTTP | 含义 | 处理 |
|---|---|---|
| `401 Unauthorized` | Key 缺失、拼错、已禁用、已过期 | 控制台核对 Key 状态 |
| `403 Forbidden` | Key 有效但**当前模型/端点**没权限(分组限制) | 检查 Key 所属分组与模型权限 |
| `429 Too Many Requests` | 触发限流或配额耗尽 | 等待 / 升级套餐 / 分配更多额度 |

详见 [09-errors.md](./09-errors.md)。

## 健康检查 / 站点状态

下面这个端点**不**需要 Key,可用于外部探活:

```bash
curl "{{BASE_URL}}/api/status"
# {"success":true,"data":{...}}
```

但**任何业务端点**(`/v1/...`、`/v1beta/...`)都必须带 Key,否则直接 401。
