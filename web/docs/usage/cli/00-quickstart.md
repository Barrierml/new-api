# tako-cli — 快速开始

`tako` 是官方推荐的命令行工具,**一键启动 Claude Code / OpenAI Codex / Gemini CLI 并自动接管网关认证**——你无需自己配 `ANTHROPIC_BASE_URL`、`OPENAI_API_KEY`、`GOOGLE_GEMINI_BASE_URL` 等环境变量,也不用维护 `~/.codex/config.toml`。

## 安装

### macOS / Linux

```bash
curl -fsSL https://cdn.jsdelivr.net/npm/tako-cli@latest/install.sh | bash
```

### Windows

```powershell
powershell -c "irm https://cdn.jsdelivr.net/npm/tako-cli/install.ps1 | iex"
```

国内网络若 jsDelivr 慢,改用 Gitee 镜像:

```powershell
powershell -c "irm https://gitee.com/SHIR0HA/tako-cli/raw/master/install.ps1 | iex"
```

安装脚本会:
1. 下载 `tako` 二进制到本地(优先原生二进制,Node 兜底)
2. 写入到 PATH
3. 不需要预装 Node.js / Bun

### 验证

```bash
tako --version
```

## 第一次启动

直接敲:

```bash
tako
```

会进入主菜单(终端内的全屏交互界面),引导你:

1. **选客户端**:Claude Code / OpenAI Codex / Gemini CLI
2. **选供应商**:{{BRAND}} 网关(默认推荐)/ Anthropic 官方 / Codex 订阅 / DeepSeek / 自定义
3. **选启动选项**:模型、跳过权限、worktree 隔离 等
4. tako 自动写好对应客户端的环境变量 / 配置文件,并启动子进程

整个流程**不需要手动改任何环境变量或配置文件**。

## 快捷启动

跳过菜单、直接启动某个客户端(适合 alias 进 shell 配置):

```bash
tako --claude       # 启动 Claude Code
tako --codex        # 启动 OpenAI Codex
tako --gemini       # 启动 Gemini CLI
```

首次使用会自动选 {{BRAND}} 网关 + 默认模型。

## 安装客户端

如果你**还没装** Claude Code / Codex / Gemini CLI,tako 也能帮你装:

```bash
tako install claude     # 装 Claude Code
tako install codex      # 装 OpenAI Codex
tako install gemini     # 装 Gemini CLI
```

## 自更新

```bash
tako --version             # 看当前版本
curl -fsSL https://cdn.jsdelivr.net/npm/tako-cli@latest/install.sh | bash
```

## 帮助

```bash
tako --help
```

## 下一步

- 想看各客户端手动配置(不装 tako-cli 时):[Claude Code](../integrations/claude-code.md) / [Codex](../integrations/codex.md) / [Gemini CLI](../integrations/gemini-cli.md)
