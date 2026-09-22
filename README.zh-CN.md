<div align="center">

# WhichRepo

**描述任务，找到仓库，展示证据。**

面向微服务、polyrepo、monorepo 和编码 Agent 的语言无关、本地优先任务路由器。

[30 秒上手](#30-秒上手) · [安装](#安装) · [Agent 接入](#编码-agent-接入) · [语言支持](#语言支持) · [隐私](#隐私)

</div>

![WhichRepo Signal Map Dashboard](assets/whichrepo-dashboard.png)

编码 Agent 找到正确上下文后通常很擅长写代码。但在拥有几十个微服务的工作区中，第一个昂贵问题往往是：**这个需求应该修改哪些仓库？**

WhichRepo 在本地建立索引，找出主项目和关联项目，跟踪跨服务依赖，并展示每个判断的文件、符号和依赖证据。运行不需要账号、API Key、网络、Go、Node.js、Python、Java 或数据库服务。

```text
“为支付回调增加重试上限”
              │
              ▼
       主项目：payments-api
     关联项目：webhook-worker
         证据：internal/webhooks/retry.go
```

## 安装

macOS / Linux：

```bash
curl -fsSL https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.sh | sh
```

Windows PowerShell：

```powershell
irm https://raw.githubusercontent.com/smallshellctw/whichrepo/main/scripts/install.ps1 | iex
```

安装脚本下载与操作系统、CPU 匹配的预编译二进制，并校验 SHA-256。普通用户不需要安装 Go 或其他语言运行时，也可以直接从 [GitHub Releases](https://github.com/smallshellctw/whichrepo/releases) 下载。

只有参与源码开发时才需要 Go 和 Node.js。完整说明见 [安装文档](docs/install.md)。

## 30 秒上手

让 WhichRepo 扫描包含所有微服务的上层目录：

```bash
whichrepo init ~/code/commerce-platform
cd ~/code/commerce-platform
whichrepo "为支付回调增加重试上限"
```

打开本地可视化页面：

```bash
whichrepo dashboard --open
```

仓库或依赖发生较大变化后刷新索引：

```bash
whichrepo index
```

SQLite 已嵌入二进制，索引自动保存在操作系统缓存目录，无需安装或启动数据库。

## 核心能力

- **理解微服务关系**：同时给出主项目和关联服务，而不只是一个模糊候选。
- **语言无关**：分析 package manifest、workspace、imports 和语法符号。
- **提供证据**：展示命中的路径、符号、aliases、package identity 和依赖。
- **默认完全本地**：SQLite + BM25，无遥测、无源码上传、无需 API Key。
- **Agent 原生**：支持 Codex、Claude Code、Cursor、OpenCode 和 Gemini CLI 的 MCP 接入。
- **单二进制**：CLI、MCP、语言分析器、SQLite 和 React Dashboard 一起分发。
- **Jev 可选增强**：支持 Vercel、TypeSafe 和 OpenRouter，失败自动降级到本地结果。

## 语言支持

| 生态 | 项目发现 | 依赖提取 | 语法证据 |
| --- | --- | --- | --- |
| Go | `go.mod`、`go.work` | module 与 imports | Tree-sitter |
| JavaScript / TypeScript | `package.json`、npm/pnpm/yarn workspace | packages 与 imports | Tree-sitter |
| Python | `pyproject.toml`、`requirements.txt` | distributions 与 imports | Tree-sitter |
| Java / Kotlin | Maven、Gradle | modules、dependencies、imports | Java Tree-sitter |
| Rust | `Cargo.toml`、Cargo workspace | crates 与 `use` | Tree-sitter |
| .NET | `.sln`、`.csproj`、`.fsproj`、`.vbproj` | project/package references | C# Tree-sitter |
| PHP | `composer.json` | Composer packages 与 namespaces | Tree-sitter |
| Ruby | `Gemfile`、gemspec | gems 与 `require` | Tree-sitter |

Tree-sitter 使用纯 Go runtime，发布产物继续保持 `CGO_ENABLED=0`、单文件和跨平台。详见 [语言支持](docs/languages.md) 与 [分析器扩展接口](docs/analyzers.md)。

## 编码 Agent 接入

WhichRepo 提供四个 MCP 工具：

- `route_task`
- `refresh_index`
- `list_projects`
- `workspace_status`

Codex：

```bash
codex mcp add whichrepo -- whichrepo mcp --workspace /path/to/workspace
```

Claude Code：

```bash
claude mcp add whichrepo --scope user -- whichrepo mcp --workspace /path/to/workspace
```

Cursor 的 `~/.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "whichrepo": {
      "command": "/absolute/path/to/whichrepo",
      "args": ["mcp", "--workspace", "/path/to/workspace"]
    }
  }
}
```

保存后重启客户端。其他客户端示例见 [`docs/integrations`](docs/integrations/)。

## Dashboard

Dashboard 只监听 `127.0.0.1`，包含：

- **Router**：需求到仓库的 signal map 与证据。
- **Repositories**：语言、package identity、依赖和索引覆盖。
- **Evaluation**：可复现的 Top-1 / Top-3 路由评测。
- **Settings & Privacy**：Provider 状态和远程 payload 边界。

CLI、Dashboard 和 MCP 使用完全相同的路由引擎。

## 配置

团队共享配置 `.whichrepo.yaml` 可以提交到 Git：

```yaml
version: 1
workspace:
  name: commerce-platform
  exclude: [archive-*, tmp-*]
projects:
  payments-api:
    path: services/payments-api
    description: 支付、收银台与 webhook 接口
    aliases: [支付服务, checkout, webhook]
```

个人或敏感 aliases 放进不提交的 `.whichrepo.local.yaml`。

## 可选 Jev 判断

默认使用本地检索。需要 Jev 二次判断时：

```bash
export WHICHREPO_PROVIDER=jev-vercel
export AI_GATEWAY_API_KEY="..."
whichrepo "为支付回调增加重试上限"
```

Provider 只接收任务文本、候选摘要与脱敏证据，不接收完整源码。鉴权、余额、限流、超时或服务异常都会自动降级到本地结果。

## 可复现评测

示例工作区包含 Go、TypeScript、Python、Java、Rust、C#、PHP 和 Ruby 微服务。当前 100 条任务的本地基线为：

- Top-1：79%
- Top-3：95%

运行：

```bash
whichrepo index --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-demo.db

whichrepo eval --workspace ./examples/polyrepo \
  --config ./examples/polyrepo/.whichrepo.yaml \
  --db /tmp/whichrepo-demo.db \
  --dataset benchmarks/starter.jsonl
```

## 隐私

- 源码与 SQLite 索引留在本机。
- 默认排除凭证、环境文件、私钥、生成代码和依赖目录。
- 不收集默认遥测。
- 路由结果只是证据，不代表授权 Agent 修改、提交、部署或访问生产环境。

## 开发

仅源码贡献者需要开发环境：

```bash
make test
make web
make build
```

项目采用 MIT License。
