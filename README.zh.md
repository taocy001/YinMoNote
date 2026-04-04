# YinMoNote（隐墨笔记）

中文 | [English](./README.md)

[![License: MIT](https://img.shields.io/badge/License-MIT-orange?style=for-the-badge)](./LICENSE)
![Built with Claude Code](https://img.shields.io/badge/Built%20with-Claude%20Code-blue?style=for-the-badge)

YinMoNote 是一款**基于浏览器的文档笔记软件**，目标是在你自己的服务器上实现飞书文档的使用体验——实时富文本编辑、嵌套文档、版本历史、一键分享——同时保持数据完全自主可控。每个元素在输入时即时渲染，无需在编辑与预览模式之间切换。

![Demo](docs/images/demo.gif)

> 本项目所有代码和文档均由 [Claude Code](https://claude.ai/code) 和 [Gemini](https://gemini.google.com/) 生成。

## Markdown 支持

编辑器基于 [Tiptap](https://tiptap.dev/) 构建，所有元素实时渲染：

| 类别 | 语法 |
|---|---|
| 标题 | H1 – H6 |
| 行内格式 | 粗体、斜体、删除线、下划线、行内代码、高亮、上标、下标 |
| 块级元素 | 引用块、有序/无序/任务列表、分隔线 |
| 代码块 | 围栏代码块，支持语法高亮（Go、Python、TypeScript、SQL、YAML、Rust 等） |
| 数学公式 | KaTeX — 块级公式与行内公式（`$E=mc^2$`） |
| 图表 | Mermaid（流程图、时序图、类图等），自动适配暗黑模式 |
| 富文本块 | Callout（信息/警告/提示/危险），Toggle（可折叠内容块） |
| 表格 | 完整表格编辑，可调列宽，飞书风格行列选择器，手机端长按菜单 |
| 图片 | 图片上传，视口懒加载 |

智能 Markdown 粘贴：直接粘贴 Markdown 原文，自动解析渲染，无需手动转换。

斜杠命令（`/`）可不离开键盘插入任意块级元素；选中文本时弹出气泡菜单，快速应用行内格式。

## 功能特性

- **版本历史**：内置 Git 存储引擎，空闲感知自动提交。可浏览历史、字符级内联 diff 对比，一键回滚。
- **命令面板**（`Cmd+K` / `Ctrl+K`）：快速搜索笔记和执行命令——输入 `>` 可切换命令模式（新建、设置、锁定、主题、回收站）。
- **主题**：自动/浅色/深色三态——默认跟随系统偏好（`prefers-color-scheme`），也可在设置中手动固定。
- **多格式导出**：一键导出为 `.md`、带样式 HTML 或 PDF。支持从文件、文件夹或 ZIP 批量导入。
- **全文搜索**：标题与内容防抖搜索，覆盖整个笔记库。
- **多页签编辑**：VS Code 风格预览/固定页签，支持拖拽排序。
- **侧边栏**：拖拽排序树形结构、嵌套文件夹、标签筛选、虚拟滚动（支持大型笔记库）。
- **跨标签同步**：一个浏览器标签页的编辑会通过 BroadcastChannel 同步到其他标签页。
- **移动端适配**：格式工具栏固定于键盘上方，响应式布局。
- **WebDAV**：通过 `/dav/` 端点连接 Obsidian、iA Writer 等任意 WebDAV 客户端。
- **AI 访问（MCP）**：内置 [Model Context Protocol](https://modelcontextprotocol.io/) 服务，让 Claude 等 AI 助手以细粒度权限读写笔记。
- **端到端加密**：AES-256-GCM + PBKDF2（10 万次迭代）或 WebAuthn PRF 硬件密钥派生。启用后服务端强制拒绝未加密笔记。
- **无障碍**：完整键盘导航（可见焦点环）、ARIA 规范的命令面板和开关组件、`prefers-reduced-motion` 动画减弱支持。

## 快速开始

### Docker（推荐）

```bash
make docker           # 构建 Docker 镜像 → dist/yinmonote-<version>-docker-<arch>.tar
make install-docker   # 构建 + 交互式安装（数据目录、端口、访问方式）
```

`make install-docker` 已包含构建步骤，会自动将 `.tar` 包加载到 Docker 并启动容器。

### 原生二进制 — macOS

```bash
make build       # 编译 → dist/yinmonote-darwin-arm64
make install     # 交互式安装：数据目录 + 端口 + LaunchAgent
```

或从 Release 页面下载 `.dmg`，双击 `Install.command` 按提示安装。

### 原生二进制 — Linux

```bash
make build       # 编译 → dist/yinmonote-linux-amd64
make install     # 交互式安装：数据目录 + 端口 + systemd 服务
```

TLS 配置、交叉编译、`.deb` 打包等更多构建选项，参见 [build/README.zh.md](build/README.zh.md)。

## WebDAV

通过 WebDAV 连接 Obsidian Remotely Save、iA Writer 等客户端，将笔记作为可读 Markdown 文件同步。

| 设置项 | 值 |
|---|---|
| 服务器地址 | `https://<host>:7281/dav/` |
| 用户名 | `yinmonote` |
| 密码 | WebDAV 令牌——在 **设置 → 安全 → WebDAV 令牌** 中一次性生成 |

**令牌获取**：打开设置 → 安全，点击"WebDAV 令牌"下的**生成**，复制令牌（仅展示一次），粘贴到 WebDAV 客户端的密码栏。

**远程基目录（Remote Base Directory）**：填任意名称均可。服务端会自动忽略 vault 名这一级目录前缀，将所有请求映射到笔记根目录，无需与服务端目录名匹配。

**文件名显示**：WebDAV 客户端看到的是人类可读的笔记标题（如 `我的笔记.md`），而非内部随机 ID。

**服务端加密**：启用 serverEncrypt 后，磁盘上的笔记以 `ENC1:` 密文存储，第三方 WebDAV 客户端无法读取。使用 WebDAV 客户端前请先关闭服务端加密。

> **无密钥模式**：若服务端未设置密码，WebDAV 无需认证即可访问。

## AI 访问（MCP）

在 **设置 → AI 访问** 中开启并生成令牌，然后将端点配置到 AI 客户端：

```json
{
  "mcpServers": {
    "yinmonote": {
      "url": "https://<your-server>/mcp/sse",
      "headers": { "Authorization": "Bearer <your-token>" }
    }
  }
}
```

访问规则支持按标签、笔记 ID、标题通配符或子树精细控制。

## 项目结构

```
build/      Dockerfile、Compose、构建/安装脚本    → build/README.zh.md
docs/       架构设计文档
tests/      单元测试 + E2E 测试套件               → tests/README.zh.md
backend/    Go 后端源代码
frontend/   Vue 3 前端源代码
```

## 开源协议

[MIT](./LICENSE) — Copyright (c) 2026 YinMoNote Contributors

本项目的源代码和文档由 AI 工具（Claude Code 与 Gemini）辅助生成，著作权人负责设计、架构及对 AI 的指导工作。

## 免责声明

YinMoNote 按**"现状"**提供，不作任何形式的明示或暗示担保。作者及贡献者不对因使用本软件导致的数据丢失、损坏或其他损害承担任何责任。**数据备份是您自己的责任。**请定期备份笔记目录，并确认备份可正常恢复后再依赖其作为唯一数据来源。
