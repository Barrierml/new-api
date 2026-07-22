# Claude Code 接入 {{BRAND}}

## 推荐:用 tako-cli 一键启动

不要自己改环境变量。**装一次 [tako-cli](../cli/00-quickstart.md)**,然后:

```bash
tako --claude
```

tako 会:
- 自动选好 {{BRAND}} 网关供应商和 Key
- 写入 `ANTHROPIC_BASE_URL` 和 `ANTHROPIC_AUTH_TOKEN`
- 启动 Claude Code 并同步 `~/.claude/settings.json`
- 启动后在状态栏显示 token 计费、当前模型

> 多机器之间想保持一致配置,只需在每台机器上 `tako install claude` + `tako --claude`,不需要复制 dotfile。

---

## 手动配置(不装 tako-cli 时)

远程纯 ssh 环境、CI/CD 里,可以自己设环境变量。

### 前置条件

- 已安装 Claude Code(`claude --version` 能看到版本)
- 已在 {{BRAND}} 控制台拿到 API Key

### 环境变量

```bash
export ANTHROPIC_BASE_URL="{{BASE_URL}}"
export ANTHROPIC_AUTH_TOKEN="cr_your_key"
```

> **为什么 `ANTHROPIC_BASE_URL` 给根域名,不加 `/v1` 或 `/api`?**
> Claude Code 内部会自动拼 `/v1/messages`,所以 `ANTHROPIC_BASE_URL` 直接给到站点根域名 `{{BASE_URL}}` 即可,实际请求会打到 `/v1/messages` 端点。

写入 shell 配置持久化:

```bash
# zsh
echo 'export ANTHROPIC_BASE_URL="{{BASE_URL}}"' >> ~/.zshrc
echo 'export ANTHROPIC_AUTH_TOKEN="cr_your_key"' >> ~/.zshrc

# bash
echo 'export ANTHROPIC_BASE_URL="{{BASE_URL}}"' >> ~/.bashrc
echo 'export ANTHROPIC_AUTH_TOKEN="cr_your_key"' >> ~/.bashrc
```

新开终端或 `source ~/.zshrc` 后生效。

### 验证

```bash
claude "hi"
/status              # 在 claude 交互界面里查看当前 endpoint
```

`/status` 应该显示 `{{BASE_URL}}`(或你自部署的域名)。

### settings.json 方式(可选)

不想改环境变量,也可以写到 `~/.claude/settings.json`:

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "{{BASE_URL}}",
    "ANTHROPIC_AUTH_TOKEN": "cr_your_key"
  }
}
```

### VS Code 插件

在 VS Code 的 `settings.json` 中添加:

```json
"claudeCode.environmentVariables": [
    {
        "name": "ANTHROPIC_BASE_URL",
        "value": "{{BASE_URL}}"
    },
    {
        "name": "ANTHROPIC_AUTH_TOKEN",
        "value": "cr_your_key"
    }
]
```

打开方式: `Cmd+Shift+P` → `Preferences: Open User Settings (JSON)`。

### 指定模型

在 `~/.claude/settings.json` 里指定要用的模型(例如 `claude-sonnet-4-5`):

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "{{BASE_URL}}",
    "ANTHROPIC_AUTH_TOKEN": "cr_your_key",
    "ANTHROPIC_MODEL": "claude-sonnet-4-5"
  }
}
```

可用模型见 [02-models.md](../02-models.md) 或控制台「模型广场」。

### 故障排查

| 现象 | 处理 |
|---|---|
| `401 Unauthorized` | `echo $ANTHROPIC_AUTH_TOKEN` 看是否拼对、是否被 IDE 覆盖了 |
| `404` 或 `Cannot POST /messages` | `ANTHROPIC_BASE_URL` 末尾别加 `/v1` 或 `/api`,直接给根域名 |
| 模型 ID 不存在 | 控制台确认该 Key 有该模型权限,或换一个 |
| 流式中途断 | 检查本地网络 / 代理 keep-alive |

## 相关文档

- [tako-cli 快速开始](../cli/00-quickstart.md) — 推荐路径
- [01-authentication.md](../01-authentication.md) — API Key 与 header 说明
- [09-errors.md](../09-errors.md) — 错误码
