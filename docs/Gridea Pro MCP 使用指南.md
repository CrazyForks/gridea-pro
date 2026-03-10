# Gridea Pro MCP 使用指南

> 让 AI 成为你的博客助手 —— 用自然语言管理你的整个博客

---

## 什么是 MCP？

MCP（Model Context Protocol，模型上下文协议）是由 Anthropic 提出的开放标准，它定义了 AI 应用与外部工具之间的通信方式。你可以把它理解为一个"翻译层"：AI 通过 MCP 协议，能够直接读取和操作你的本地应用数据，而不仅仅是回答问题。

Gridea Pro 实现了完整的 MCP 服务，这意味着你可以在 Claude Desktop、Cursor、Claude Code 等支持 MCP 的 AI 客户端中，通过对话的方式完成博客的所有操作 —— 从写文章、管理标签，到渲染站点、一键部署。

---

## 核心能力一览

Gridea Pro MCP 提供了 **25 个工具**、**3 个资源**和 **5 个提示词模板**，覆盖了博客管理的方方面面。

### 工具（Tools）

AI 可以调用的操作指令，每个工具对应一个具体的博客管理动作。

| 类别 | 工具 | 功能说明 |
|------|------|---------|
| **文章管理** | `list_posts` | 获取所有文章列表（标题、日期、标签、发布状态） |
| | `get_post` | 获取某篇文章的完整内容 |
| | `create_post` | 创建新文章（支持标题、正文、标签、分类、发布日期等） |
| | `update_post` | 更新已有文章的内容或属性 |
| | `delete_post` | 删除文章（需确认） |
| **闪念管理** | `list_memos` | 获取所有闪念记录 |
| | `create_memo` | 创建新闪念（自动提取 #标签） |
| | `update_memo` | 更新闪念内容 |
| | `delete_memo` | 删除闪念（需确认） |
| | `get_memo_stats` | 获取闪念统计数据（热力图、标签分布等） |
| **标签管理** | `list_tags` | 查看所有标签 |
| | `create_tag` | 创建新标签（名称、slug、颜色） |
| | `delete_tag` | 删除标签（需确认） |
| **分类管理** | `list_categories` | 查看所有分类 |
| | `create_category` | 创建新分类 |
| | `delete_category` | 删除分类（需确认） |
| **友链管理** | `list_links` | 查看所有友情链接 |
| | `create_link` | 添加友链（站名、URL、头像、描述） |
| | `delete_link` | 删除友链（需确认） |
| **菜单管理** | `list_menus` | 查看导航菜单 |
| | `create_menu` | 添加菜单项 |
| | `delete_menu` | 删除菜单项（需确认） |
| **主题与配置** | `list_themes` | 查看已安装的主题 |
| | `get_theme_config` | 获取当前主题配置（站名、作者、描述等） |
| | `update_theme_config` | 更新主题配置（需确认，会显示变更预览） |
| | `get_site_settings` | 获取站点部署设置（敏感信息已脱敏） |
| | `update_site_settings` | 更新部署设置 |
| **评论管理** | `list_comments` | 获取最近的评论（支持分页） |
| | `reply_comment` | 回复评论 |
| | `delete_comment` | 删除评论（需确认） |
| **站点操作** | `render_site` | 渲染生成静态站点 |
| | `deploy_site` | 部署到远程平台（需确认，需开启配置） |

### 资源（Resources）

AI 可以主动读取的上下文信息，帮助它更好地理解你的博客现状。

| 资源 URI | 说明 |
|----------|------|
| `gridea://site/info` | 站点基本信息（站名、描述、作者、域名、主题） |
| `gridea://posts/summary` | 所有文章概要（标题、日期、标签、发布状态） |
| `gridea://memos/recent` | 最近 20 条闪念 |

### 提示词模板（Prompts）

预设的工作流模板，AI 会按照既定步骤引导你完成复杂任务。

| 模板名称 | 说明 |
|----------|------|
| `blog_writing_assistant` | **博客写作助手** —— 输入主题，AI 自动查阅现有标签和分类，拟定大纲，撰写全文，一键发布 |
| `memo_to_post` | **闪念整理器** —— 将散落的闪念按主题归类，组织成结构化的博客文章 |
| `content_review` | **内容审查** —— 检查所有文章的标题、标签、分类完整性，给出优化建议 |
| `site_health_check` | **站点健康检查** —— 全面诊断站点问题（空标签、缺失配置、无效链接等），按严重程度分级报告 |
| `translate_post` | **文章翻译** —— 将指定文章翻译成目标语言，保留格式，自动创建新文章 |

---

## 安全设计

MCP 服务在安全性上做了充分考虑，确保 AI 不会在未经确认的情况下执行破坏性操作。

### 两步确认机制

所有危险操作（删除、部署、修改配置）都采用 **"预览 → 确认"** 的两步模式：

1. **第一次调用**（不带确认参数）：AI 仅返回操作预览，展示即将发生的变化
2. **第二次调用**（带 `confirm=true`）：确认后才真正执行操作

例如，当 AI 要删除一篇文章时：

```
AI: ⚠️ 确认删除文章《Go 并发编程指南》？
    请再次调用 delete_post 并设置 confirm=true 以确认。

你: 确认删除。

AI: 文章已删除：Go 并发编程指南
```

### 配置变更预览

修改主题配置时，AI 会先展示完整的变更 diff：

```
⚠️ 以下配置将被修改：
  siteName: 'My Blog' → 'Eric's Tech Blog'
  siteDescription: '' → '记录技术与生活'

请带 confirm=true 重新调用以应用变更。
```

### 敏感信息脱敏

通过 `get_site_settings` 获取部署配置时，所有敏感字段（token、password、privateKey 等）会自动替换为 `***`，防止泄露。

### 部署功能默认关闭

部署功能（`deploy_site`）默认不启用。只有在配置中显式设置 `DEPLOY_ENABLED=true` 后，该工具才会注册到 MCP 服务中。未开启时，AI 完全看不到这个工具，无法调用。

---

## 安装与配置

### 前置条件

- 已安装 Gridea Pro 桌面应用，并有一个站点数据目录（通常在 `~/Documents/Gridea Pro`）
- 已安装 Go 1.22+（用于编译 MCP 服务）
- 一个支持 MCP 的 AI 客户端（Claude Desktop、Cursor、Claude Code 等）

### 第一步：编译 MCP 服务

在 Gridea Pro 项目根目录下执行：

```bash
make build-mcp
```

编译完成后，二进制文件位于 `build/bin/gridea-pro-mcp`。

你也可以手动编译：

```bash
go build -o build/bin/gridea-pro-mcp ./backend/cmd/mcp
```

### 第二步：配置 AI 客户端

根据你使用的客户端，选择对应的配置方式。

#### Claude Desktop

编辑配置文件 `~/Library/Application Support/Claude/claude_desktop_config.json`（macOS）：

```json
{
  "mcpServers": {
    "gridea-pro": {
      "command": "/path/to/gridea-pro-mcp",
      "env": {
        "SOURCE_DIR": "/Users/你的用户名/Documents/Gridea Pro"
      }
    }
  }
}
```

#### Cursor

在项目根目录创建 `.cursor/mcp.json`：

```json
{
  "mcpServers": {
    "gridea-pro": {
      "command": "/path/to/gridea-pro-mcp",
      "env": {
        "SOURCE_DIR": "/Users/你的用户名/Documents/Gridea Pro"
      }
    }
  }
}
```

#### Claude Code

在项目根目录创建 `.mcp.json`：

```json
{
  "mcpServers": {
    "gridea-pro": {
      "command": "/path/to/gridea-pro-mcp",
      "env": {
        "SOURCE_DIR": "/Users/你的用户名/Documents/Gridea Pro"
      }
    }
  }
}
```

### 环境变量说明

| 变量名 | 必填 | 说明 | 默认值 |
|--------|------|------|--------|
| `SOURCE_DIR` | 否 | Gridea Pro 站点数据目录的绝对路径 | `~/Documents/Gridea Pro` |
| `DEPLOY_ENABLED` | 否 | 设为 `true` 开启部署功能 | 不开启 |

### 第三步：验证连接

重启 AI 客户端后，尝试发送以下消息：

```
列出我博客的所有文章
```

如果 AI 成功调用 `list_posts` 并返回了你的文章列表，说明 MCP 服务已正常工作。

---

## 使用场景

以下是一些典型的使用场景，展示 AI + Gridea Pro MCP 的实际价值。

### 场景一：AI 辅助写作

```
你: 帮我写一篇关于 Go 语言 Context 的技术博客，专业但易懂的风格

AI: 让我先了解一下你博客现有的标签和分类体系...
    [调用 list_tags, list_categories]

    你目前有标签 "Go", "后端", "编程" 和分类 "技术"。
    我来为你写这篇文章。

    [撰写完整文章]

    文章已写好，是否发布？

你: 发布吧

AI: [调用 create_post，自动匹配标签 "Go"、分类 "技术"]
    文章《深入理解 Go Context：从原理到实践》已发布！
    需要渲染站点吗？
```

### 场景二：闪念整理成文

```
你: 把我最近关于"产品设计"的闪念整理成一篇文章

AI: [调用 list_memos]
    我找到了 8 条相关闪念，按主题分为三组：
    1. 用户体验原则（3 条）
    2. 设计工具对比（2 条）
    3. 案例分析（3 条）

    建议以《我的产品设计方法论》为题，按这三个主题组织。可以吗？

你: 好的，第三部分的案例分析可以展开写

AI: [撰写并调用 create_post]
    文章已发布！要不要把这些已整理的闪念标记或清理一下？
```

### 场景三：批量内容优化

```
你: 检查我的博客有什么问题

AI: [调用 site_health_check 模板]
    [依次调用 list_posts, list_tags, list_categories, list_links,
     get_theme_config, get_site_settings]

    📊 站点健康检查报告

    🔴 严重问题（2 项）
    - 3 篇文章没有分配分类
    - 站点描述为空

    🟡 建议优化（4 项）
    - 标签 "test" 和 "temp" 没有任何文章使用
    - 2 篇文章内容少于 100 字
    - 友链 "示例站点" 缺少描述
    - 1 个菜单项链接指向 "#"

    🟢 正常（3 项）
    - 所有文章都有标签 ✓
    - 部署配置完整 ✓
    - 主题配置正常 ✓

    需要我帮你修复这些问题吗？
```

### 场景四：多语言发布

```
你: 把《Go Context 详解》翻译成英文

AI: [调用 get_post 获取原文]
    [翻译全文]

    翻译完成，标题为 "Deep Dive into Go Context: From Theory to Practice"。
    请检查翻译质量，确认后我将创建英文版本。

你: 翻译不错，发布吧

AI: [调用 create_post，文件名自动添加 -en 后缀]
    英文版已发布！需要渲染站点吗？
```

### 场景五：一键渲染部署

```
你: 文章都改好了，帮我部署上线

AI: [调用 deploy_site]
    ⚠️ 确认部署到平台 'github'（域名: blog.example.com）？

你: 确认

AI: 正在渲染站点...
    正在推送到 GitHub...
    ✅ 站点已成功部署到 blog.example.com
```

---

## 支持的部署平台

开启 `DEPLOY_ENABLED=true` 后，MCP 支持以下部署平台：

| 平台 | 说明 |
|------|------|
| **GitHub Pages** | 通过 Git 推送到 GitHub 仓库的指定分支 |
| **Gitee Pages** | 通过 Git 推送到 Gitee 仓库 |
| **Vercel** | 通过 Vercel API 直接部署，支持增量上传 |

部署前会自动执行站点渲染，确保发布的是最新内容。部署平台和凭证信息在 Gridea Pro 桌面应用中配置，MCP 服务读取同一份配置，无需重复设置。

---

## 技术架构

```
┌─────────────────┐     stdio      ┌──────────────┐
│  AI 客户端       │ ◄──────────► │  gridea-pro-mcp  │
│  (Claude Desktop │     MCP       │  (独立进程)   │
│   Cursor 等)     │    Protocol   │              │
└─────────────────┘               └──────┬───────┘
                                         │
                                         │ 复用
                                         ▼
                              ┌─────────────────────┐
                              │  Gridea Pro 后端     │
                              │                     │
                              │  Service 层（业务逻辑）│
                              │       ↓             │
                              │  Repository 层（数据）│
                              │       ↓             │
                              │  JSON / Markdown 文件│
                              └─────────────────────┘
```

**关键设计决策：**

- **独立二进制，共享代码**：`gridea-pro-mcp` 是一个不包含 GUI 的轻量级进程，与 Gridea Pro 桌面应用共享相同的 Service 和 Repository 代码，确保数据操作逻辑完全一致。
- **Stdio 传输**：AI 客户端直接启动 `gridea-pro-mcp` 进程，通过标准输入输出通信，零网络开销，安全可靠。
- **无数据库**：所有数据以 JSON 和 Markdown 文件形式存储在本地，MCP 服务直接读写这些文件，与桌面应用使用完全相同的数据源。

---

## 常见问题

### MCP 服务会和 Gridea Pro 桌面应用冲突吗？

不建议同时运行。两者操作同一份数据文件，同时写入可能导致数据不一致。建议在使用 MCP 时关闭桌面应用，反之亦然。

### 为什么 AI 看不到 deploy_site 工具？

需要在配置中添加 `"DEPLOY_ENABLED": "true"` 环境变量。该功能默认关闭，需要显式开启。

### SOURCE_DIR 应该指向哪个目录？

指向 Gridea Pro 的站点数据目录，通常是 `~/Documents/Gridea Pro`。这个目录下包含 `posts/`、`themes/`、`config/` 等子目录。如果不确定，打开 Gridea Pro 桌面应用，在设置中可以看到数据目录的路径。

### 支持哪些 AI 客户端？

任何实现了 MCP 协议 stdio 传输方式的客户端都可以使用，包括但不限于：

- [Claude Desktop](https://claude.ai/download) — Anthropic 官方桌面客户端
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — Anthropic 官方 CLI 工具
- [Cursor](https://cursor.com) — AI 代码编辑器
- 其他支持 MCP 的第三方客户端

### 如何更新 MCP 服务？

当 Gridea Pro 项目代码更新后，重新执行 `make build-mcp` 编译即可。MCP 服务的版本始终与项目代码保持同步。
