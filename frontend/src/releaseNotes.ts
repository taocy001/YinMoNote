/**
 * 发布说明内容，在首次初始化或版本升级后自动写入文档库。
 *
 * 版本号从构建时注入的 __APP_VERSION__ 常量读取，该常量由 vite.config.ts
 * 在构建期从仓库根目录的 VERSION 文件中读取，是版本的唯一来源。
 *
 * 内容格式为 Markdown，与编辑器的存储格式一致。
 * 根据用户语言偏好（i18n lang）选择对应版本显示。
 */

export const RELEASE_NOTE_VERSION: string = import.meta.env.VITE_APP_VERSION

export const RELEASE_NOTE_ZH = `# YinMoNote v${import.meta.env.VITE_APP_VERSION} 发布说明

欢迎使用 **YinMoNote** —— 一款自托管笔记应用，将飞书文档、Notion 级别的 Markdown 写作体验带到你自己的服务器。

> 读完后可以删除此笔记。

## 编辑器

每个元素在输入时即时渲染，无需在编辑与预览模式之间切换。

**支持的语法：**

| 类别 | 语法 |
|---|---|
| 标题 | H1 – H6 |
| 行内格式 | 粗体、斜体、删除线、下划线、行内代码、高亮、上标、下标 |
| 块级元素 | 引用块、有序/无序/任务列表、分隔线 |
| 代码块 | 围栏代码块，支持语法高亮（Python、Go、TypeScript、SQL、Rust 等） |
| 数学公式 | KaTeX —— 行内公式 \`$E=mc^2$\` 与块级公式 |
| 图表 | Mermaid 流程图、时序图、类图，自动适配暗黑模式 |
| 富文本块 | Callout（信息/警告/提示/危险），Toggle（可折叠内容块） |
| 表格 | 完整表格编辑，支持行列操作 |
| 图片 | 图片上传，视口懒加载 |

**键盘操作：**

- 输入 \`/\` 可不离开键盘插入任意块级元素
- 选中文字弹出气泡菜单，快速应用行内格式
- 直接粘贴 Markdown 原文，自动解析渲染

## 版本历史

内置 Git 存储引擎，每 30 秒自动提交。点击工具栏时钟图标，可浏览历史版本、进行**字符级内联 diff 对比**，并一键回滚。

## 全文搜索

标题与内容防抖搜索，覆盖整个笔记库，匹配结果在侧边栏显示内容匹配标记。

## 侧边栏

拖拽排序树形结构、嵌套文件夹、标签筛选、虚拟滚动，支持大型笔记库。

## 多格式导出

通过工具栏菜单一键导出为 \`.md\`、带样式 HTML 或 PDF。

## WebDAV

通过 \`/dav/\` 端点连接 Obsidian、iA Writer 等任意 WebDAV 客户端。

| 设置项 | 值 |
|---|---|
| 服务器地址 | \`http://<主机>:8080/dav/\` |
| 用户名 | 任意值 |
| 密码 | 解锁应用时使用的密码 |

## AI 访问（MCP）

在 **Settings → AI 访问** 中开启并生成令牌，将端点配置到 AI 客户端。访问规则支持按标签、笔记 ID、标题通配符或子树精细控制。

## 移动端

格式工具栏固定于键盘上方，响应式布局适配所有屏幕尺寸。

## 可选加密

在 **Settings** 中开启密码或指纹（WebAuthn）保护。开启后，所有笔记内容与元数据在客户端加密后再上传至服务器。

---

*YinMoNote 完全开源，采用 MIT 协议。所有代码由 [Claude Code](https://claude.ai/code) 与 [Gemini](https://gemini.google.com/) 生成。*
`

export const RELEASE_NOTE_EN = `# What's New in YinMoNote v${import.meta.env.VITE_APP_VERSION}

Welcome to **YinMoNote** — a self-hosted note-taking app that brings a commercial-grade Markdown writing experience, inspired by Feishu Docs and Notion, to your own server.

> Feel free to delete this note once you've read it.

## Editor

Every element renders live as you type — no mode-switching, no preview toggle.

**Supported syntax:**

| Category | Syntax |
|---|---|
| Headings | H1 – H6 |
| Inline | Bold, italic, strikethrough, underline, code, highlight, superscript, subscript |
| Blocks | Blockquote, ordered / unordered / task lists, horizontal rule |
| Code | Fenced code blocks with syntax highlighting (Python, Go, TypeScript, SQL, Rust …) |
| Math | KaTeX — inline \`$E=mc^2$\` and block math |
| Diagrams | Mermaid flowchart, sequence, class (auto dark-mode) |
| Rich blocks | Callout (info / warning / tip / danger), Toggle |
| Tables | Full row / column editing |
| Images | Upload with lazy loading |

**Keyboard shortcuts:**

- Type \`/\` to insert any block element without leaving the keyboard
- Select text to open the bubble formatting menu
- Paste raw Markdown — it is parsed and rendered automatically

## Version History

Every note is tracked by a built-in Git engine that auto-commits every 30 seconds. Click the clock icon in the toolbar to browse past versions, compare with **character-level inline diff highlights**, and roll back with one click.

## Full-Text Search

Debounced title and content search across the entire note library. Results appear in the sidebar with content-match badges.

## Sidebar

Drag-and-drop tree with nested folders, tag filter, and virtual scroll — stays fast with thousands of notes.

## Export

Export any note as \`.md\`, styled HTML, or PDF via the toolbar menu.

## WebDAV

Connect Obsidian, iA Writer, or any WebDAV client to \`/dav/\`.

| Setting | Value |
|---|---|
| Server URL | \`http://<host>:8080/dav/\` |
| Username | Any value |
| Password | Your unlock password |

## AI Access (MCP)

Enable in **Settings → AI Access**, generate a token, then add the endpoint to your AI client. Access rules restrict by tag, note ID, title glob, or subtree.

## Mobile

Format toolbar pinned above the keyboard, responsive layout for all screen sizes.

## Optional Encryption

Password or biometric (WebAuthn) protection — enable in **Settings**. When enabled, all note content and metadata are encrypted on the client before reaching the server.

---

*YinMoNote is open source under the MIT License. All code was generated with [Claude Code](https://claude.ai/code) and [Gemini](https://gemini.google.com/).*
`
