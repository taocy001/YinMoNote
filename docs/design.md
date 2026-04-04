# 隐墨笔记 (YinMoNote) — 技术设计文档

> 本文档是活文档，随功能演进持续更新。上次全量刷新：2026-04-04

---

## 1. 项目愿景

隐墨笔记是一款以隐私为核心的个人笔记应用，采用端到端加密（E2EE）。核心主张：

- **零知识服务器**：服务端是盲存储仓库，持有的全是密文，没有密钥，无法读取任何内容。
- **硬件锁定**：支持 WebAuthn（平台认证器/TouchID）作为主要解锁方式，PBKDF2 密码派生作为备选。
- **个人私密使用**：不追求协作能力，只追求一个人用得放心、用得顺手。
- **自托管优先**：一条 `docker compose` 命令即可部署，数据落在自己机器上。

---

## 2. 系统架构

### 2.1 总体分层

```
浏览器 (Vue 3 SPA)
  ├── crypto.ts              # Web Crypto API 封装：密钥派生、AES-GCM 加解密
  ├── indexCache.ts           # IndexedDB 笔记内容缓存（加速全文索引构建）
  ├── composables/
  │   ├── useLibrary.ts        # 笔记状态唯一中枢：结构、CRUD、搜索
  │   ├── useContentIndex.ts   # 全文搜索索引（IndexedDB 缓存 + 批量获取 + 降级策略）
  │   ├── useOrphanCleanup.ts  # 孤立资源垃圾回收
  │   ├── useEditorSave.ts     # 编辑器自动保存系统（去抖/间隔/串行队列）
  │   ├── useFindReplace.ts    # 编辑器内查找替换
  │   ├── useImageDecrypt.ts   # 图片懒解密（Intersection Observer）
  │   ├── useExport.ts         # HTML/PDF/Markdown 导出 + 安全净化
  │   ├── useWordStats.ts      # 字数/字符数/阅读时间统计
  │   ├── useBatchEncryption.ts # 全库加密/解密批量转换
  │   ├── useDragDrop.ts       # 侧边栏拖拽调整宽度
  │   ├── useBatchImport.ts    # 批量导入（文件/文件夹/ZIP）
  │   ├── useLibraryTrash.ts   # 回收站操作（删除/恢复/永久删除/清空）
  │   └── useLibraryStructure.ts # 笔记结构操作（移动/创建子笔记）
  ├── components/
  │   ├── CommandPalette.vue # Cmd+K 命令面板（笔记搜索 + > 命令模式）
  │   ├── TabBar.vue         # 多页签栏（桌面端，拖拽排序，localStorage 持久化）
  │   ├── Editor.vue         # Tiptap 富文本编辑器（含增量渲染、TOC、Slash 菜单、搜索高亮）
  │   ├── DiffView.vue       # 飞书风格字符级 diff 对比
  │   ├── SearchResults.vue  # 全文搜索结果面板（含关键词高亮）
  │   ├── HistoryPanel.vue   # 版本历史右侧抽屉
  │   ├── SettingsPanel.vue  # 设置面板（外观/编辑器/安全/AI）
  │   ├── UnlockModal.vue    # 解锁/初始化模态框
  │   ├── ResetModal.vue     # 笔记库重置确认模态框
  │   ├── ImageView.vue      # 图片懒加载 + 解密 + 拖拽缩放
  │   ├── CodeBlockView.vue  # 代码块（语法高亮 + KaTeX 数学 + Mermaid 图表）
  │   ├── CalloutView.vue    # 高亮提示块渲染器
  │   ├── ToggleBlockView.vue# 折叠块渲染器
  │   ├── InlineMathView.vue # 行内公式渲染器
  │   ├── BubbleToolbar.vue  # 选中文本浮动格式菜单
  │   ├── MobileToolbar.vue  # 移动端底部格式工具栏
  │   ├── ShortcutsModal.vue # 快捷键帮助弹窗
  │   ├── SlashMenu.vue      # 斜杠命令菜单 + hover 块操作
  │   ├── TableOverlay.vue   # 表格覆盖层：飞书风格行列选择器栏（悬停显示），手机端长按上下文菜单
  │   ├── Callout.ts         # Callout Tiptap 扩展定义
  │   ├── ToggleBlock.ts     # ToggleBlock Tiptap 扩展定义
  │   ├── InlineMath.ts      # InlineMath Tiptap 扩展定义
  │   └── ToggleSwitch.vue   # 通用开关组件（role="switch"，SettingsPanel 复用）
  └── App.vue                # 布局容器：侧边栏 + 编辑区状态协调
           │ HTTPS
Go 后端 (Gin)
  ├── library.go          # NoteLibrary 核心结构、CRUD、ListNotes
  ├── library_structure.go # reconcileStructure、配额校验
  ├── library_trash.go    # 回收站（软删除、恢复、永久删除、purge）
  ├── library_git.go      # Git 自动提交（StartAutoCommitter、GitGC）
  ├── library_util.go     # 工具函数（atomicWriteFile、extractNoteTitle）
  ├── server.go           # HTTP 路由、认证退避、安全头
  ├── auth_srp.go         # SRP-6a 握手（/api/auth/srp/init + verify）
  ├── mcp.go              # MCP (Model Context Protocol) JSON-RPC 2.0 服务端
  ├── mcp_policy.go       # MCP 访问控制策略求值
  ├── config.go           # 配置结构与 clamp 逻辑
  ├── types.go            # 共享数据类型
  ├── webdav.go           # WebDAV 支持（/dav/ 路径；标题虚拟化；vault 前缀忽略）
  ├── selfca.go           # 自签名 TLS 证书生成与加载
  └── static.go           # go:embed 前端静态资源
           │ volume mount
文件系统
  ├── ~/.yinmonote/config.json   # 服务配置（固定路径，不随笔记目录变化）
  └── ~/.yinmonote/notes/        # 默认笔记目录（可通过 DATA_DIR 独立指定）
      ├── _structure.json        # 加密后的层级结构元数据
      ├── *.md                   # 加密笔记正文（ENC1: 前缀）
      └── assets/                # 加密图片资产
```

### 2.2 后端架构（Go）

后端代码按职责分布于 `backend/` 目录下多个文件：NoteLibrary 层已拆分为 5 个文件（`library.go` 核心 CRUD、`library_structure.go` 结构对账与配额、`library_trash.go` 回收站、`library_git.go` Git 自动提交、`library_util.go` 工具函数）；`auth_srp.go` 实现 SRP-6a 握手；`server.go` 负责 HTTP 路由与中间件；`mcp.go` + `mcp_policy.go` 实现 MCP 协议与访问控制；`config.go` 管理配置结构与 clamp 逻辑；`webdav.go` 提供 WebDAV 支持（含标题虚拟化层和 vault 前缀忽略）；`selfca.go` 生成自签 TLS；`static.go` 嵌入前端资源。`main.go` 仅做入口初始化。

**NoteLibrary** 是纯文件操作层，不感知 HTTP：
- `AtomicWrite`：先写 `.tmp` 再 `os.Rename`，保证崩溃安全；无论进程何时被杀，磁盘上不会留下半写文件。
- `sync.Mutex`（非 RWMutex）：所有文件操作均为写密集型，用普通互斥锁避免 RWMutex 在写多读少场景下的优先级反转。
- 配额检查（`CheckNoteQuota` / `CheckAssetQuota` / `CheckStructureQuota`）在写入前完成，防止部分写入后才触发超限。
- Git 自动提交：`StartAutoCommitter` goroutine 采用空闲感知策略——用户停止编辑 5 分钟后提交，或持续编辑时每 10 分钟强制提交（以先到者为准），保留完整版本历史。
- `reconcileStructure`：在三种场景下触发：(1) 启动时；(2) `GetStructure()` 发现文件缺失；(3) `StartReconcileDebouncer` 每 2 秒轮询 `reconcilePending` 标志（WebDAV 写入/删除后设置）。自动修复 `_structure.json` 与磁盘文件的不一致，并清理 Parents map 中的孤立条目。

**Server** 是纯路由层，不直接操作文件：
- 认证优先级：Bearer Token（SRP-6a 握手后颁发，`SRPVerifier` 已设置）→ HTTP Basic Auth（`AUTH_USER` 环境变量）→ 开放访问（keyless 模式）。
- IP 级连续失败退避（共享于所有认证端点），每次失败增加 500ms 延迟（上限 5s），并发延迟上限 20 个。
- `handleSaveStructure` 对明文 JSON 做空结构完整性校验：若 order 为空但磁盘存在笔记文件，则拒绝写入，防止前端 bug 导致结构被清空。
- `IsValidName` 用严格正则（`^[0-9]{8}[a-z0-9]{16}\.(md|...)$`）校验文件名，配合物理路径 Join 双重拦截路径穿透。
- Token 比对使用 `subtle.ConstantTimeCompare()`，防止时序攻击。

**MCP 服务端**（Model Context Protocol）：
- 基于 JSON-RPC 2.0，通过 SSE（`GET /mcp/sse`）+ HTTP POST（`POST /mcp/messages`）双通道通信。
- 提供 7 个工具：`list_notes`、`read_note`、`get_structure`、`search_notes`、`get_note_history`、`read_note_version`、`update_note`。
- 访问控制策略（MCPPolicy）：按 tag / note_id / title_glob / subtree_of 规则匹配，first-match-wins 语义。
- MCP Token 独立于 Session Token，服务端生成，仅存储 SHA-256 哈希。
- 最大 50 个并发 MCP 会话。

**配置**（`~/.yinmonote/config.json`）：配置文件路径固定，不随笔记目录（`DATA_DIR`）变化。每次启动读取并写回（含范围修正）。镜像重建后不会丢失，因为 `~/.yinmonote/` 通过 volume 挂载。

### 2.3 前端架构（Vue 3）

**状态管理**：无 Pinia/Vuex，状态分布于 composables + provide/inject + localStorage/sessionStorage/IndexedDB 三层本地存储。

**`useLibrary` composable** 是笔记状态的唯一中枢：
- 所有组件通过此 composable 读写笔记状态，不在组件内直接操作 structure。
- `sanitizeStructure`：每次 `loadNotesList` 后执行，修复结构与磁盘文件列表不一致（文件存在但不在 structure、structure 引用已删除文件）。这是一种自愈机制而非容错兜底。
- `saveStructure`：同时持久化 structure + titles + tags 为一个原子对象，确保服务端数据与本地状态始终一致。
- `buildContentIndex`：解锁后在后台批量（每批 5 条，100ms 间隔）拉取并解密所有笔记正文，构建内存全文索引。优先从 IndexedDB 缓存加载（基于 modTime 判断命中），仅增量拉取有变更的笔记。
- `displayList`：带虚拟分页（每页 40 条）的扁平化层级列表，含折叠状态和内容匹配标记。

**IndexedDB 缓存**（`indexCache.ts`）：
- 存储笔记的原始服务端内容（加密模式存 ENC1 密文，keyless 模式存明文），配合 localStorage 中的 modTime 元数据实现命中判断。
- IndexedDB 不可用时静默降级为 no-op（如 happy-dom 测试环境）。
- 加密模式下锁定时清空缓存；keyless 模式下跨会话持久化。
- `GET /api/notes/bulk` 批量接口一次获取所有笔记内容，消除 N 次 HTTP 往返。

**乐观 UI 原则**：所有结构变更（新建、删除、移动、标签编辑）先更新本地 reactive 状态，再异步持久化，不等待服务端响应后 reload。

**增量内容渲染**：大文档按 100 行分块，首次仅加载第一块，滚动到底部（距底 300px）时懒加载后续块，避免大文档卡顿。

**CSS 设计系统**（见第 4 节）：所有颜色通过 `var(--*)` CSS 变量定义，组件不写死颜色值。

### 2.4 Docker 多阶段构建

```
Stage 1 (node:20-alpine):   npm run build → dist/（前端单元测试在此阶段执行）
Stage 2 (golang:1.21-alpine): go build → yinmonote binary（go:embed 将 dist/ 打包进二进制；后端单元测试在此阶段执行）
Stage 3 (debian:bookworm):  仅复制最终二进制 → 最终镜像（非 root 用户 yinmonote:1000 运行）
```

前端 `dist/` 通过 `go:embed` 在编译期内嵌进二进制，最终镜像仅含一个可执行文件，无需挂载静态目录。

**多平台打包**：除 Docker 外，还支持 Linux DEB（amd64/arm64）和 macOS DMG（amd64/arm64）原生安装包。

---

## 3. 安全模型

### 3.1 密钥派生

| 方式 | 实现 | 特点 |
|------|------|------|
| WebAuthn | `navigator.credentials.create/get` + platform authenticator | 密钥绑定硬件，需要生物识别/PIN |
| PBKDF2 | 100,000 次迭代，SHA-256，**每用户随机 salt**（首次初始化时生成；存量用户在首次解锁时自动迁移） | 密码越强越安全，适合无硬件场景 |

WebAuthn 实现细节（双层策略）：**Tier 1**（PRF 扩展，Chrome 120+/macOS Sonoma+）使用硬件绑定的 PRF 秘密值与 rawId 拼接后喂给 PBKDF2，即使 localStorage 泄露也无法离线派生密钥；**Tier 2**（无 PRF 支持时回退）仅使用 rawId，提供 UI 级生物识别门控。详见 `docs/security.md`。

### 3.2 内存中的密钥

- 运行时密钥以 `CryptoKey` 形式存在于内存变量 `_key`。首次派生时 `extractable: true`（需导出 JWK 进行 session wrapping），从 session 恢复时导入为 `extractable: false`（后续操作中密钥不可再导出）。
- **非导出密钥**：XSS 攻击者无法通过 `exportKey` 或控制台指令取出原始密钥字节。
- 唯一例外：`saveKeyToSession` 调用时需临时导出为 JWK，立即用 `sessionWrapKey` 再次加密后丢弃 JWK 明文。

### 3.3 会话保持（sessionStorage）

锁屏体验不应要求每次刷新都重新输入密码。实现方案：
1. 用 `window.name`（tab 级别，非持久化）派生 `sessionWrapKey`（PBKDF2，10,000 次迭代）。
2. 将主密钥 JWK 用 `sessionWrapKey` 加密后存入 `sessionStorage`。
3. 下次访问时，用 `sessionWrapKey` 解包，恢复 `_key` 到内存中。
4. 关闭标签页 → `window.name` 消失 → `sessionWrapKey` 无法重建 → sessionStorage 中的密文永久失效。
5. `unload` 事件主动清空 `window.name`，防止跨域导航后被读取（使用 `unload` 而非 `beforeunload`，确保在保存完成后执行且不可取消）。

这使得会话恢复不依赖密码，同时保证标签页关闭后密钥自动失效。

### 3.4 认证体系

服务端支持三种认证方式（按优先级，详见 `docs/security.md` 第 4 节）：

| 方式 | 条件 | 说明 |
|------|------|------|
| Bearer Token | `SRPVerifier` 已设置 | SRP-6a 握手验证密码后颁发 32 字节随机 token；`Authorization: Bearer <token>`；24 小时 TTL |
| HTTP Basic Auth | `AUTH_USER` 环境变量 | 环境变量指定用户名密码 |
| 开放访问 | 以上均未配置 | Keyless 模式，无认证 |

MCP 独立认证：MCP Token 由服务端生成（48 字符随机），仅存哈希，生成时返回一次明文。  
WebDAV 独立认证：静态 WebDAV token（与 SRP Session Token 无关），详见 `docs/security.md` 第 4.3 节。

**WebDAV 标题虚拟化**（`webdav.go`）：笔记在磁盘上以 `<YYYYMMDD><16位随机字符>.md` 的 canonical ID 存储，但 WebDAV 客户端（如 Obsidian）看到的是笔记标题（如 `数据中心网络.md`）。`davFileSystem` 实现了完整的 `webdav.FileSystem` 接口，在所有五个方法（`OpenFile`、`RemoveAll`、`Rename`、`Stat`、`Mkdir`）中透明地完成标题 ↔ canonical ID 的双向映射：
- `buildTitleMap()`：扫描 `ListNotes()` 结果，调用 `extractNoteTitle()` 读取首行，经 `davSanitizeTitle()` 净化（替换 `/\:*?"<>|` 为 `_`，截断至 200 字节），构建双向 map。同名笔记自动追加 `(2).md`、`(3).md` 去重后缀。
- `Rename` 作为标题修改：将 canonical ID 文件的 MOVE 操作实现为 H1 内容更新（`updateNoteH1`），保留 canonical ID 不变。
- `normalizePath()`：剥除路径的第一段（vault 名前缀），使 Obsidian "Remote Base Directory" 设置为任意名称均可透明工作，无需与服务端目录匹配。

### 3.5 E2EE 元数据

- `_structure.json` 在上传前被全量加密为 `ENC1:` Blob，服务端只看到密文。
- 攻击者控制服务器也无法推断笔记数量、层级关系、标题或标签。
- 客户端本地 `localStorage` 缓存（`yinmo_structure_backup_v2`）同样以加密形式存储。

### 3.6 传输安全

应用内置四种 TLS 模式（通过环境变量选择），同时也可配合外部 Nginx/Caddy 或 Tailscale WireGuard 使用：

| 模式 | 环境变量 | 适用场景 |
|------|----------|---------|
| 自动证书（Let's Encrypt） | `ACME_DOMAIN=notes.example.com` | 有公网域名 |
| 手动证书 | `TLS_CERT=<路径> TLS_KEY=<路径>` | 自有证书 |
| 自签名 TLS | `TLS_SELF=1`（可加 `TLS_EXTRA_IPS`） | IP 直连、无域名 |
| 纯 HTTP | 不设置上述变量 | 本地 / 反代后端 |

- Content-Security-Policy 和 `X-Frame-Options: DENY` 防止点击劫持和 iframe 嵌入。
- `X-Content-Type-Options: nosniff` 防止 MIME 类型嗅探。
- `Referrer-Policy: no-referrer` 防止 Referer 信息泄露。
- `Permissions-Policy: camera=(), microphone=(), geolocation=()` 限制浏览器 API。
- 请求体大小限制（20MB）防止大请求 DoS。
- Proxy 信任仅限 `127.0.0.1` 和 `::1`，防止 IP 伪造。

### 3.7 各模式加密状态总览

| | ① Keyless | ② Keyless<br>+ HTTPS | ③ 密码/指纹<br>无云端加密 | ④ 密码/指纹<br>无云端加密<br>+ HTTPS | ⑤ 密码/指纹<br>有云端加密 | ⑥ 密码/指纹<br>有云端加密<br>+ HTTPS |
|---|---|---|---|---|---|---|
| 笔记内容传输 | 明文 | TLS 保护 | **明文** | TLS 保护 | ENC1 密文 | ENC1 密文 + TLS |
| 笔记内容存储 | 明文 | 明文 | **明文** | **明文** | ENC1 密文 | ENC1 密文 |
| 标题/标签/结构传输 | 明文 | TLS 保护 | **明文** | TLS 保护 | ENC1 密文 | ENC1 密文 + TLS |
| 图片资产传输 | 明文 | TLS 保护 | **明文** | TLS 保护 | ENC1 密文 | ENC1 密文 + TLS |
| Bearer Token 传输 | 无 | 无 | **明文暴露** | TLS 保护 | **明文暴露** | TLS 保护 |
| 文件名/请求路径传输 | 明文 | TLS 保护 | 明文 | TLS 保护 | 明文 | TLS 保护 |
| 服务器磁盘泄露 | 全裸 | 全裸 | 全裸 | 全裸 | 安全 | 安全 |
| 网络窃听（笔记内容） | 全裸 | 安全 | 全裸 | 安全 | 安全 | 安全 |
| 网络窃听（Token/元数据） | 无 Token | 无 Token | 暴露 | 安全 | 暴露 | 安全 |
| 服务端鉴权 | 无 | 无 | Bearer Token | Bearer Token | Bearer Token | Bearer Token |

**说明**：
- 云端加密保护的是**内容**（笔记、结构、图片），即使 HTTP 被窃听，密文也不可读；但 Bearer Token 和请求元数据（路径、文件名）仍明文暴露。
- HTTPS 补全了云端加密的盲区：保护 Token、文件名、请求路径等元数据；但对服务器磁盘泄露无效。
- 两者叠加（云端加密 + HTTPS）才能同时应对网络窃听和服务器入侵两类威胁。
- **UI 呈现**：开启云端加密时顶部显示「密文传输&密文存储」蓝色徽标；关闭时显示「明文传输&明文存储」橙色警告徽标。Keyless 模式强制关闭云端加密，不显示开关。

---

## 4. 设计系统（Refined Ink）

### 4.1 设计语言

"Refined Ink"（精炼墨迹）：以墨水、纸张质感为灵感，暖白 + 暖灰色调，避免纯冷白的压迫感。字体选用 Inter（西文）+ 系统中文字体，行高宽松，让长文阅读舒适。

### 4.2 CSS 变量 Token 体系

所有颜色、阴影、字号均通过 CSS 变量定义，亮色/暗色模式切换只改变 `:root` 和 `.dark` 下的变量值，组件模板无需感知主题。

```css
/* 核心 Token（亮色模式） */
--bg-app          /* 应用背景 */
--bg-editor       /* 编辑区背景，略亮于 app */
--bg-hover        /* 悬停/选中背景 */
--text-primary    /* 主文字 */
--text-secondary  /* 次要文字 */
--text-muted      /* 静默文字（占位符、辅助说明） */
--accent          /* 强调色（靛蓝 #4F46E5 / 暗色 #818CF8） */
--accent-light    /* 强调色浅版（按钮激活背景） */
--border          /* 常规分割线 */
--border-strong   /* 较重分割线 */
--shadow-md       /* 中等阴影（弹出层） */
```

### 4.3 SVG 图标系统

所有图标内联为 SVG，不引入任何图标库。图标尺寸统一为 14×14px 视口，stroke-width 1.3，使用 `currentColor` 继承文字颜色，适配主题切换。

### 4.4 组件模式

| 区域 | 模式 |
|------|------|
| **侧边栏** | 桌面端可折叠为 44px 图标条；移动端以 overlay 展开；宽度可拖拽调整（最小 160px） |
| **笔记列表** | 虚拟分页（每页 40 条 + 滚动加载），拖拽排序（HTML5 DnD API），内容匹配徽标 |
| **编辑器头部** | 桌面专属；TOC 按钮、字数统计、保存状态、导出菜单、历史、专注模式、快捷键、设置 |
| **编辑器** | 增量渲染（100 行/块），Slash 命令菜单，浮动格式栏，拖拽手柄，搜索高亮 |
| **搜索结果** | 右侧面板，按笔记分组显示匹配片段（每条最多 5 段），点击跳转并高亮 |
| **版本历史** | 右侧抽屉，飞书风格字符级 diff（LCS + diff-match-patch），支持回滚 |
| **设置面板** | 全屏 overlay，四个标签页：外观 / 编辑器 / 安全 / AI |

---

## 5. 功能清单

### 5.1 编辑器

| 功能 | 状态 | 说明 |
|------|------|------|
| 富文本格式 | ✅ | 粗体、斜体、下划线、删除线、高亮、上标、下标、行内代码 |
| 标题层级 | ✅ | H1–H6，支持 Markdown 快捷输入 |
| 列表 | ✅ | 无序列表、有序列表、任务清单（勾选框） |
| 引用块 | ✅ | Markdown `>` 语法 |
| Callout 高亮块 | ✅ | 信息/警告/提示/危险四种样式，自定义 emoji |
| Toggle 折叠块 | ✅ | 任意内容可折叠/展开 |
| 代码块语法高亮 | ✅ | Lowlight 引擎，30+ 语言，一键复制 |
| 行内数学公式 | ✅ | `$formula$` 语法触发 KaTeX 渲染 |
| 数学公式块 | ✅ | 代码块语言设为 `math`，KaTeX 渲染 |
| Mermaid 图表 | ✅ | 代码块语言设为 `mermaid`，可切换源码/预览，暗色模式适配 |
| 表格 | ✅ | 完整表格编辑，表头行，可调列宽，行列操作；飞书风格行列选择器栏（悬停显示），手机端长按上下文菜单 |
| 链接 | ✅ | 自动识别 URL，气泡菜单编辑链接 |
| 图片 | ✅ | 拖入/粘贴上传，懒加载（IntersectionObserver），拖拽缩放，Retina 修正 |
| 水平分割线 | ✅ | `---` 语法或 Slash 命令 |
| Slash 命令菜单 | ✅ | `/` 触发，20 种块类型（h1-h6、列表、待办、引用、表格、代码、公式、图表、分割线、4 种 Callout、折叠块） |
| 浮动格式栏 | ✅ | 选中文字弹出，桌面端；移动端底部工具栏 |
| 块拖拽排序 | ✅ | 悬停显示拖拽手柄 |
| 增量内容渲染 | ✅ | 大文档按 100 行分块懒加载，避免首屏卡顿 |
| 目录面板（TOC） | ✅ | 自动从标题生成，点击跳转 |
| 智能全选 | ✅ | Ctrl+A 循环：当前块 → 正文（排除标题） |
| Markdown 智能粘贴 | ✅ | 粘贴 Markdown 文本自动解析为富文本 |
| 专注模式 | ✅ | 隐藏侧边栏/面板，淡化工具栏，悬停临时显示 |
| 打字机模式 | ✅ | 光标始终居中于视口 |
| 自动保存 | ✅ | 3 秒防抖 + 10 秒强制间隔；保存状态实时显示；失败可点击重试 |
| 字数统计 | ✅ | 字数、字符数、预估阅读时间，CJK 按单字计 |

### 5.2 组织与导航

| 功能 | 状态 | 说明 |
|------|------|------|
| 树形结构 | ✅ | 多层嵌套（服务端限制最深 3 层），父子关系存于 structure |
| 拖拽排序 | ✅ | HTML5 DnD，支持 before/after/inside 三种落点 |
| 虚拟分页 | ✅ | 每页 40 条 + 滚动加载，千条笔记首屏 <200ms |
| 标签系统 | ✅ | 侧边栏标签过滤，悬停编辑标签 |
| 侧边栏可调宽度 | ✅ | 拖拽调整，最小 160px |
| 折叠/展开 | ✅ | 桌面端可折叠为图标条；移动端以 overlay 展开 |
| 收藏/置顶 | ✅ | structure.pinned，侧边栏顶部分区，图钉按钮切换 |
| 多页签 | ✅ | 桌面端编辑器顶部页签栏，拖拽排序，中键关闭，localStorage 持久化 |
| 面包屑导航 | ❌ | 未实现 |

### 5.3 搜索

| 功能 | 状态 | 说明 |
|------|------|------|
| 标题搜索 | ✅ | 实时过滤 displayList，200ms 防抖 |
| 全文内容搜索 | ✅ | 解锁后后台建立内存索引，搜索同时匹配标题与正文 |
| 搜索结果面板 | ✅ | 按笔记分组显示上下文片段（每条最多 5 段），点击跳转，编辑器内关键词高亮 |
| 标签过滤 | ✅ | 侧边栏点击标签即过滤 |
| IndexedDB 缓存 | ✅ | 基于 modTime 增量更新，加密模式锁定时清空，加速索引构建 |
| 批量接口加速 | ✅ | `GET /api/notes/bulk` 一次获取所有笔记，消除 N 次 HTTP 往返 |
| 编辑器内查找替换 | ✅ | Ctrl+F 查找 / Ctrl+H 替换，匹配高亮+计数，逐个/全部替换 |
| 高级搜索语法 | ❌ | 未实现（无 `tag:`/`path:`/正则 等搜索运算符） |
| OCR 图片文字搜索 | ❌ | 未实现 |

### 5.4 版本历史

| 功能 | 状态 | 说明 |
|------|------|------|
| Git 自动提交 | ✅ | 空闲感知策略：5 分钟无写入提交 / 10 分钟强制提交 |
| 历史列表 | ✅ | 右侧抽屉，按时间倒序展示提交记录 |
| 字符级 Diff | ✅ | 飞书风格内联 diff（LCS + diff-match-patch），保留富文本格式 |
| 版本回滚 | ✅ | 一键还原到任意历史版本 |
| 版本命名/标记 | ✅ | structure.commitLabels，HistoryPanel 内联编辑标记 |

### 5.5 导入导出

| 功能 | 状态 | 说明 |
|------|------|------|
| 导出 HTML | ✅ | 带样式内联下载，安全净化（移除 script/svg/style/on* 事件） |
| 导出 PDF | ✅ | 浏览器打印对话框，隐藏页脚 URL |
| 导出 Markdown | ✅ | `.md` 文件，Callout/Toggle/表格完整序列化 |
| 系统"另存为"对话框 | ✅ | 使用 File System Access API，不支持时回退默认下载 |
| Markdown 粘贴导入 | ✅ | 粘贴或拖入 Markdown 文本/文件自动解析 |
| 导出 DOCX | ❌ | 未实现 |
| 批量导出 | ❌ | 未实现（仅支持单篇导出） |
| 批量导入 .md 文件 | ✅ | 支持文件/文件夹/ZIP 批量导入 |
| 从 Notion/Evernote 导入 | ❌ | 未实现 |

### 5.6 安全与加密

| 功能 | 状态 | 说明 |
|------|------|------|
| WebAuthn 解锁 | ✅ | 平台认证器（TouchID/FaceID/PIN） |
| PBKDF2 密码解锁 | ✅ | 100k 次迭代，每用户随机 salt |
| Keyless 模式 | ✅ | 无加密，适合完全私有部署 |
| 服务端加密（serverEncrypt）| ✅ | 笔记上传前 AES-GCM 加密，服务端存储 ENC1 密文 |
| 批量加密转换 | ✅ | 一键全库明文/密文转换，含图片资产，进度条 |
| 会话保持 | ✅ | sessionWrapKey 加密后存 sessionStorage |
| 闲置自动锁定 | ✅ | 可配置超时时间（分钟） |
| 密钥导出/导入 | ✅ | Base64 导出，支持多设备同步密钥 |
| 配额管理 | ✅ | 服务端限制笔记数、大小、嵌套深度等 |
| 内置 TLS | ✅ | ACME/手动证书/自签名三种模式 |

### 5.7 平台与接入

| 功能 | 状态 | 说明 |
|------|------|------|
| PWA | ✅ | manifest + Service Worker，可安装到桌面/主屏幕 |
| 移动端适配 | ✅ | 响应式布局，侧边栏折叠，底部格式工具栏 |
| WebDAV | ✅ | `/dav/` 路径；标题虚拟化（笔记标题而非随机 ID）；vault 前缀自动忽略 |
| MCP 协议 | ✅ | SSE + JSON-RPC 2.0，7 个工具，AI 助手读写笔记 |
| MCP 访问控制 | ✅ | 基于 tag/note_id/title_glob/subtree_of 的细粒度权限 |
| 暗色模式 | ✅ | CSS 变量切换，跟随系统或手动 |
| 多语言（中/英） | ✅ | 运行时切换 |
| 快捷键帮助 | ✅ | `?` 键弹出速查表 |
| 原生桌面应用 | ❌ | 仅 Web（PWA 可安装，但无原生菜单/系统集成） |
| 原生移动应用 | ❌ | 仅 PWA（无推送通知、离线同步等原生能力） |
| 浏览器剪藏扩展 | ❌ | 未实现 |
| 公开链接分享 | ❌ | 未实现（个人隐私定位，不需要分享） |

### 5.8 竞品常见功能对照

**已实现的竞品功能：**

| 功能 | 说明 |
|------|------|
| 回收站/软删除 | 删除移入回收站，30 天自动清理 |
| 编辑器内查找替换 | Ctrl+F/H，匹配高亮+计数+替换 |
| 收藏/置顶笔记 | 图钉按钮，置顶分区 |
| 上次打开记忆 | 刷新后恢复上次打开笔记 |
| 批量导入 .md 文件 | 文件/文件夹/ZIP 批量导入 |
| 命令面板 (Cmd+K) | 笔记搜索 + > 命令模式 |

**待实现 / 路线图：**

| 功能 | 竞品参考 | 优先级 |
|------|----------|--------|
| 双向链接 `[[]]` + 反向引用 | Obsidian、Notion、思源 | ✅ 适合，已列入路线图 |
| 模板系统 | Notion、Obsidian、思源、飞书 | ✅ 适合 |
| 导出 DOCX | Notion、思源、飞书 | 🔶 可选 |
| 知识图谱可视化 | Obsidian、思源 | 🔶 可选，依赖双向链接 |
| 自定义 CSS/主题市场 | Obsidian、思源 | 🔶 可选 |
| 嵌入式内容（YouTube 等） | Notion、飞书 | 🔶 可选 |
| Web Clipper 浏览器扩展 | Evernote、Obsidian、Notion | 🔶 可选 |
| OCR 图片文字搜索 | Evernote、思源 | 🔶 可选 |

**不适合本产品定位：**

| 功能 | 原因 |
|------|------|
| 数据库/关系表格 | 超出个人笔记定位 |
| 实时协同编辑 | 个人私密使用定位 |
| 插件/扩展生态 | 维护成本过高，保持轻量 |
| 间隔重复/闪卡 | 超出笔记核心功能 |

---

## 6. 类似产品功能对比

### 6.1 产品定位对比

| 维度 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 定位 | 团队协作工作台 | 个人知识管理 | 本地知识库 | 云端笔记 | 企业文档协作 | 在线文档协作 | **个人加密笔记** |
| 部署方式 | SaaS | 本地应用 | 本地/自托管 | SaaS | SaaS | SaaS | **自托管** |
| 开源 | ❌ | ❌（插件 API 开放） | ✅ AGPLv3 | ❌ | ❌ | ❌ | ✅ |
| 收费模式 | 免费+付费 | 免费+付费同步 | 免费+付费云 | 免费+付费 | 免费（企业付费） | 免费（企业付费） | **完全免费** |
| 目标用户 | 团队/个人 | 知识工作者 | 技术用户 | 大众 | 企业团队 | 大众/教育 | **隐私敏感个人** |

### 6.2 编辑器能力对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 编辑模式 | 块编辑器 | Markdown（实时预览） | 块编辑器（Markdown 混合） | 富文本 | 富文本 | 富文本 | **富文本（Tiptap）** |
| Markdown 原生 | 部分（输入快捷） | ✅（核心） | ✅（混合） | 部分 | 部分（输入快捷） | ❌ | **✅（智能粘贴+导出）** |
| 代码块语法高亮 | ✅（90+ 语言） | ✅（PrismJS） | ✅ | ✅（基础） | ✅ | 基础 | **✅（Lowlight 30+）** |
| 数学公式 | ✅ KaTeX | ✅ MathJax | ✅ KaTeX | ❌ | ✅ LaTeX | 基础 | **✅ KaTeX 行内+块级** |
| Mermaid 图表 | 部分（2024 新增） | ✅（内置） | ✅（+PlantUML/Graphviz） | ❌ | 部分 | ❌ | **✅（内置，源码/预览切换）** |
| 表格 | ✅ + 数据库表 | 基础 Markdown 表格 | ✅（增强） | 基础 | ✅ + Bitable | ✅（电子表格） | **✅ 基础（表头+列宽可调）** |
| Callout/高亮块 | ✅ | ✅（`>[!type]`） | ✅ | ❌ | ✅ | 基础 | **✅（4 种+自定义 emoji）** |
| Toggle 折叠块 | ✅ | ✅（标题折叠） | ✅ | ❌ | ✅ | ❌ | **✅** |
| Slash 命令 | ✅（首创） | ✅ | ✅ | ✅（v10+） | ✅ | ✅ | **✅（20 种块类型）** |
| 块拖拽重排 | ✅ | 有限 | ✅ | ❌ | ✅ | ❌ | **✅** |
| 嵌入内容（视频等） | ✅（50+ 类型） | 本地文件 | ✅（iframe） | 有限 | ✅ | 有限 | **❌** |
| 编辑器内查找替换 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | **✅（Ctrl+F/H，高亮+计数+替换）** |
| 模板系统 | ✅（最佳） | ✅（+Templater） | ✅（Go 模板语法） | ✅ | ✅ | ✅ | **❌** |

### 6.3 组织与知识管理对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 层级结构 | ✅ 无限嵌套页面 | ✅ 文件系统文件夹 | ✅ 笔记本>文档 | ⚠️ 仅 2 层（笔记组>笔记本） | ✅ 空间>文件夹 | ✅ 文件夹 | **✅ 多层嵌套树** |
| 标签 | ⚠️ 仅数据库属性 | ✅ 嵌套 `#tag/sub` | ✅ `#tag#` 语法 | ✅（核心功能） | ⚠️ 有限 | ❌ | **✅ 全局标签** |
| 双向链接 | ✅ | ✅（核心特性） | ✅（块级引用） | ⚠️ 基础 | ⚠️ @提及 | ❌ | **❌（待实现）** |
| 知识图谱 | ❌ | ✅（核心特性） | ✅ | ❌ | ❌ | ❌ | **❌** |
| 收藏/置顶 | ✅ Favorites | ✅ Bookmarks | ✅ | ✅ Shortcuts | ✅ | ✅ | **✅（图钉按钮，侧边栏置顶分区）** |
| 最近访问 / 多页签 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | **✅（多页签栏 + 拖拽排序 + 持久化）** |
| 回收站 | ✅ | ✅（Trash 文件夹） | ✅ | ✅ | ✅ | ✅ | **✅（软删除，30 天自动清理）** |

### 6.4 搜索能力对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 全文搜索 | ✅ | ✅ | ✅ | ✅（最快之一） | ✅ | ✅ | **✅** |
| 高级搜索语法 | ⚠️ 基础 | ✅ `path:/tag:/regex` | ✅ SQL 查询 | ✅ `tag:/intitle:/created:` | ⚠️ 过滤器 | ⚠️ 基础 | **❌** |
| OCR 图片搜索 | ❌ | ⚠️ 插件 | ✅（内置） | ✅（最佳） | ⚠️ 有限 | ⚠️ 有限 | **❌** |
| 搜索结果预览 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | **✅（上下文片段+高亮）** |

### 6.5 安全与隐私对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| E2E 加密 | ❌ | ⚠️ 仅 Sync | ✅ AES-256-GCM | ⚠️ 仅选中文字 | ❌ | ❌ | **✅ 全量 E2EE** |
| 元数据加密 | ❌ | ⚠️ Sync 加密 | ✅ | ❌ | ❌ | ❌ | **✅（结构/标题/标签全加密）** |
| 零知识架构 | ❌ | ⚠️ 仅 Sync | ✅ | ❌ | ❌ | ❌ | **✅** |
| 生物识别解锁 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅ WebAuthn** |
| 自托管 | ❌ | N/A（本地） | ✅ Docker | ❌ | ❌ | ❌ | **✅ Docker** |
| 数据所有权 | 厂商控制 | ✅ 用户所有 | ✅ 用户所有 | 厂商控制 | 厂商控制 | 厂商控制 | **✅ 用户所有** |

### 6.6 版本历史与导入导出对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 版本历史 | ✅（按套餐 7-∞ 天） | ⚠️ 需 Sync/插件 | ✅（快照） | ✅（付费） | ✅ | ✅ | **✅ Git（无限期）** |
| Diff 可视化 | ✅ 并排对比 | ⚠️ Sync | ✅ | ⚠️ 基础 | ✅ | ⚠️ 基础 | **✅ 字符级内联 diff** |
| 版本回滚 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | **✅** |
| 导出 PDF | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | **✅** |
| 导出 HTML | ✅ | ⚠️ 插件 | ✅ | ✅ | ❌ | ❌ | **✅** |
| 导出 Markdown | ✅ | ✅（原生） | ✅ | ⚠️ 有限 | ✅ | ⚠️ 有限 | **✅** |
| 导出 DOCX | ❌ | ⚠️ 插件 | ✅ | ❌ | ✅ | ✅ | **❌** |
| 导出格式总数 | 3-4 | 原生 MD | **10+（最丰富）** | 2-3 | 3-4 | 3-4 | **3（HTML/PDF/MD）** |
| 批量导入 | ✅（Notion/Evernote/...） | ✅（官方 Importer） | ✅（Markdown 文件夹） | ✅（ENEX） | ✅（Word/Confluence） | ✅（Word/Excel） | ✅（文件/文件夹/ZIP） |

### 6.7 AI 与平台能力对比

| 特性 | Notion | Obsidian | 思源笔记 | 印象笔记 | 飞书文档 | 腾讯文档 | **隐墨笔记** |
|------|--------|----------|----------|----------|----------|----------|-------------|
| 内置 AI | ✅ Notion AI | ❌（插件） | ⚠️ 需配端点 | ✅ 基础 | ✅（最深度集成） | ✅ 混元 | **✅ MCP 协议** |
| AI 访问控制 | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ | **✅（细粒度策略）** |
| API | ✅ REST | ⚠️ 插件 API | ✅ 本地 HTTP | ⚠️ 老旧 | ✅ 开放平台 | ⚠️ 企业版 | **✅ REST + MCP + WebDAV** |
| WebDAV | ❌ | ❌（本地文件） | ✅（同步后端） | ❌ | ❌ | ❌ | **✅** |
| 实时协作 | ✅（最佳之一） | ❌ | ❌ | ⚠️ 基础 | ✅（最佳之一） | ✅（200+ 并发） | **❌（个人定位）** |
| 桌面应用 | Win/Mac | **Win/Mac/Linux** | **Win/Mac/Linux** | Win/Mac | Win/Mac | Win/Mac | **PWA（跨平台）** |
| 移动应用 | iOS/Android | iOS/Android | iOS/Android | iOS/Android | iOS/Android | iOS/Android | **PWA（可安装）** |
| Web Clipper | ✅ | ✅ | ✅ | ✅（最佳） | ⚠️ 有限 | ❌ | **❌** |
| 插件生态 | ❌（封闭） | **✅ 1500+** | ✅ 社区市场 | ❌ | ⚠️ 应用平台 | ⚠️ 有限 | **❌** |

### 6.8 隐墨笔记竞争力总结

**核心优势（竞品难以匹敌）**：
1. **真正的 E2EE + 零知识架构**：全量加密（内容+结构+标题+标签+图片），竞品中仅思源笔记接近此水平，Obsidian Sync 加密但非自托管
2. **WebAuthn 生物识别解锁**：所有竞品均无此功能，密钥绑定硬件
3. **MCP 协议 + 细粒度 AI 访问控制**：支持第三方 AI 读写笔记且可精确控制权限，竞品中独一无二
4. **Git 驱动的无限版本历史**：无付费限制、无时间限制，字符级 diff 对比
5. **完全免费自托管**：数据 100% 用户所有，无订阅费用

**主要差距（按优先级排序）**：
1. **双向链接 + 反向引用**：知识管理的核心能力，Obsidian/Notion/思源 的标志性功能
2. **模板系统**：提升新建笔记效率，Notion/Obsidian 验证了此功能的价值
3. ~~**批量导入**~~：已实现（文件/文件夹/ZIP）

---

## 7. 路线图（待实现）

### 近期
- **双向链接 `[[]]`**：`[[文件名]]` 语法，Tiptap 扩展实现自动补全与跳转，反向引用面板展示链入笔记。
- **模板系统**：预定义模板（会议记录、日记、周报等），新建笔记时选择；支持用户自定义模板。
- ~~**批量导入 Markdown**~~：已实现（文件/文件夹/ZIP 批量导入）。
- **每日笔记入口**：一键创建以今日日期命名的笔记，作为快速记录入口。

### 中期
- **离线编辑**：Service Worker 拦截 API 请求，缓存加密内容，联网后自动同步（冲突策略 TBD）。
- **高级搜索语法**：支持 `tag:`、`title:`、日期范围等搜索运算符。
- **导出 DOCX**：通过 docx 库生成 Word 文档。

### 远期
- **知识图谱可视化**：依赖双向链接数据，力导向图展示笔记关联关系。
- **WebAuthn Challenge 规范化**：后端提供一次性 challenge endpoint，符合完整 WebAuthn 规范。

### 明确不做
- **实时协同编辑**：与个人隐私笔记定位冲突，需要密钥分发协议，复杂度极高。
- **数据库表/关系型视图**：Notion 式数据库超出笔记工具范畴。
- **插件/扩展生态**：维护成本不适合小团队，保持核心精简。
- **间隔重复/闪卡**：非笔记核心功能，思源的实现证明这适合作为独立产品。

---

## 8. 构建与部署

构建命令、环境变量、Docker 镜像命名规范详见 [build/README.zh.md](../build/README.zh.md)。

测试命令、目录结构、E2E 运行方式详见 [tests/README.zh.md](../tests/README.zh.md)。

测试架构设计、覆盖范围、设计原则详见 [docs/testing-guide.md](testing-guide.md)。
