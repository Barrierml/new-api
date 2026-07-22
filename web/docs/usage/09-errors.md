# 09 — 错误处理

{{BRAND}} 错误响应**保持你所用协议的原始格式**(OpenAI 路径返回 OpenAI 风格,Anthropic 路径返回 Anthropic 风格,Gemini 路径返回 Google 风格),便于现有 SDK 直接识别。

## HTTP 状态码

| 状态 | 含义 | 常见原因 |
|---|---|---|
| `400 Bad Request` | 请求参数有误 | JSON 解析失败、必填字段缺失、模型 ID 拼错 |
| `401 Unauthorized` | 未认证 | API Key 缺失 / 拼错 / 已禁用 |
| `402 Payment Required` | 余额不足 | 当前 Key 额度耗尽,无法预扣 |
| `403 Forbidden` | 已认证但无权限 | Key 所属分组没开通该模型 / 该端点 |
| `404 Not Found` | 资源不存在 | URL 拼错、模型 ID 不在你能用的列表 |
| `408 Request Timeout` | 请求超时 | 模型响应过久 |
| `413 Payload Too Large` | 请求体太大 | 上传图过大 / messages 超长 |
| `429 Too Many Requests` | 限流 | 触发模型 RPM 上限,或网关额度节流 |
| `500 Internal Server Error` | 服务异常 | 联系管理员 |
| `502 Bad Gateway` | 网关异常 | 上游模型服务暂时不可达或返回非法响应 |
| `503 Service Unavailable` | 服务过载 | 重试 |
| `504 Gateway Timeout` | 网关超时 | 重试 + 缩短上下文 |

## 网关错误格式

网关层产生的错误(鉴权、额度、路由失败等)按你调用的协议风格返回:

### OpenAI 风格(`/v1/chat/completions`、`/v1/responses` 等)

```json
{
  "error": {
    "message": "invalid token: API key not found or disabled",
    "type": "invalid_request_error",
    "param": null,
    "code": null
  }
}
```

### Anthropic 风格(`/v1/messages`)

```json
{
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "max_tokens: must be greater than 0"
  }
}
```

### Google 风格(`/v1beta/...`)

```json
{
  "error": {
    "code": 400,
    "message": "...",
    "status": "INVALID_ARGUMENT"
  }
}
```

## 上游模型服务错误

若错误来自具体上游模型服务(OpenAI / Anthropic / Google),网关会**按你所用协议的原始格式**透传,这样现有官方 SDK 能直接识别错误类型。

## 重试策略

推荐对幂等请求做指数退避:

| 错误 | 重试? |
|---|---|
| `429` | ✅ 退避后重试,看 `Retry-After` header |
| `500` / `502` / `503` / `504` | ✅ 最多 3 次,间隔 1s/3s/9s |
| `408`(超时) | ✅ 但缩短上下文 |
| `400` / `401` / `403` / `404` / `413` | ❌ 立即失败,改请求 |
| `402` | ❌ 充值后再试 |

## 流式响应中的错误

SSE 流中途出错时:

### OpenAI 路径
最后一条事件可能是 `data: {"error":{...}}`,然后 `data: [DONE]`。客户端要在每个 chunk 上检查 `error` 字段。

### Anthropic 路径
事件类型 `error`:

```
event: error
data: {"type":"error","error":{"type":"...","message":"..."}}
```

### Gemini 路径
`:streamGenerateContent` 中途出错时,流会立即关闭并返回非 200 HTTP 状态,或在最后一个 chunk 写入 `error` 对象。

## 调试 tips

1. **看 `message` 字段**:错误体的 `error.message` 通常已说明原因
2. **本地复现 401**:确认环境变量没拼错(常见:`NEW_API_KEY` vs `OPENAI_API_KEY` 混用)
3. **403 vs 404**:403 表示模型存在但你没权限(去申请开通/换分组);404 是模型 ID 拼错或模型已下线
4. **`429` 频发**:控制台看你的 RPM 配额,或在客户端做 token bucket 限流
5. **`502/504` 频发**:稍后重试 / 联系管理员 / 减小 `max_tokens`
