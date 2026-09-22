# WhichRepo

> 描述任务，找到仓库，查看证据。

WhichRepo 是面向多仓库、微服务和 monorepo 工作区的本地优先任务路由器。它帮助开发者和编码 Agent 在修改代码前确定主仓库、关联仓库及判断依据。

## 核心特点

- 默认完全本地运行，无需账号和 API Key。
- 自动发现 Git polyrepo 及 Go、Node、Python、Rust、Maven、Gradle 项目。
- 使用 SQLite + BM25 检索候选仓库，并展示命中的路径和证据。
- 可选接入 Jev，通过 Vercel、TypeSafe 或 OpenRouter 增强判断。
- 提供 CLI、MCP、Codex Skill 和本地可视化 Dashboard。
- 不上传完整源码，不收集遥测。

## 快速开始

```bash
make build
./bin/whichrepo init /path/to/workspace
./bin/whichrepo "修复支付回调重试问题"
./bin/whichrepo dashboard --open
```

## Agent 接入

```bash
codex mcp add whichrepo -- /absolute/path/to/whichrepo mcp --workspace /path/to/workspace
claude mcp add whichrepo --scope user -- /absolute/path/to/whichrepo mcp --workspace /path/to/workspace
```

完整配置、Provider、隐私和开发说明请查看英文 [README](README.md)。
