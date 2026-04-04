# YinMoNote 安全现状

> 上次全量更新：2026-04-04

本文档描述 YinMoNote 当前的安全防护体系、已知限制和可接受的风险。

---

## 1. 威胁模型

**假设攻击者场景**：攻击者可能拥有服务器控制权（磁盘访问、流量监听），甚至客户端物理控制权，但**没有用户的解密密钥/密码**。

**不在防护范围内**：
- 用户设备被完全入侵（操作系统级 keylogger、内存 dump）
- 用户被社会工程攻击骗取密码

---

## 2. 加密体系

### 2.1 密钥派生

| 方式 | 实现 | 特点 |
|------|------|------|
| WebAuthn | 平台认证器（TouchID/FaceID/PIN），双层策略：**Tier 1** 使用 WebAuthn PRF 扩展获取硬件绑定秘密值，与 rawId 拼接后喂给 PBKDF2（Chrome 120+/macOS Sonoma+）；**Tier 2** 不支持 PRF 时回退到仅 rawId 派生（UI 级拦截） | Tier 1 密钥真正绑定硬件，即使 localStorage 泄露也无法离线派生；Tier 2 仅提供生物识别门控 |
| PBKDF2 密码 | 100,000 次迭代，SHA-256，**每用户随机 salt**（首次初始化时生成，存于 localStorage） | 跨设备可恢复，密码丢失则不可找回 |
| Keyless 模式 | 无密钥，跳过所有加密 | 适合完全私有的单人本地部署 |

**存量用户盐值迁移**：使用旧版固定盐值（`yinmo-stable-salt-v1`）初始化的用户，在首次解锁时自动迁移至随机盐值（`migrateLegacySaltIfNeeded`）。

### 2.2 运行时密钥保护

- 主密钥以 `CryptoKey` 形式存于模块私有变量 `_key`。首次派生时 `extractable: true`（需导出 JWK 进行 session wrapping），JWK 导出后立即丢弃，后续从 session 恢复时导入为 `extractable: false`，XSS 攻击者无法通过 `crypto.subtle.exportKey` 取出原始字节。
- 会话保持：主密钥 JWK 用 `sessionWrapKey` 加密后存入 sessionStorage；`sessionWrapKey` 从 `window.name` 派生，仅存在于当前标签页内存，关闭标签页后自动失效。
- 锁定时立即清空 `_key` 引用并清理 sessionStorage 缓存。
- `window.name` 在页面卸载（`unload`）时主动清空，防止跨域导航后被读取。使用 `unload` 而非 `beforeunload` 确保清理不可被取消，且在保存操作完成之后执行。

### 2.3 静态数据加密（serverEncrypt 模式）

开启云端加密后：
- 笔记正文在上传前经过 AES-256-GCM 加密，格式：`ENC1:<iv_base64>:<ciphertext_base64>`
- `_structure.json`（包含层级关系、标题、标签）全量加密，服务端无法推断任何元数据
- 图片资产同样加密存储

### 2.4 IndexedDB 缓存安全

- `indexCache.ts` 在加密模式下存储笔记的 **ENC1 密文**（非明文），lockLibrary 时调用 `clearCache()` 清空。
- Keyless 模式下存储明文并跨会话持久化（无安全风险，因本身无加密需求）。
- IndexedDB 不可用时（隐私模式、配额耗尽）静默降级为 no-op，不抛异常。
- modTime 元数据存于 localStorage（~40 字节/笔记），命中判断无需反序列化 IndexedDB 内容。

---

## 3. 网络与传输安全

应用内置四种 TLS 模式：

| 模式 | 环境变量 | 适用场景 |
|------|----------|---------|
| ACME（Let's Encrypt） | `ACME_DOMAIN=notes.example.com` | 有公网域名，自动续期 |
| 手动证书 | `TLS_CERT=<路径> TLS_KEY=<路径>` | 自有证书 |
| 自签名 TLS | `TLS_SELF=1` | IP 直连、无域名；CA 证书可通过 `/ca.crt` 下载 |
| 纯 HTTP | 不设置上述变量 | 本地 / 反代后端 |

也可配合外部 Nginx/Caddy TLS 终止或 Tailscale WireGuard 隧道使用。

安全响应头：
- `Content-Security-Policy`：`'self'` 限制脚本源，`unsafe-inline` 用于 Tiptap 编辑器样式
- `X-Frame-Options: DENY` 防止点击劫持
- `X-Content-Type-Options: nosniff` 防止 MIME 类型嗅探
- `Referrer-Policy: no-referrer` 防止 Referer 信息泄露
- `Permissions-Policy: camera=(), microphone=(), geolocation=()` 限制浏览器 API
- 请求体大小限制（20MB）防止大请求 DoS

---

## 4. 认证与访问控制

### 4.1 主认证

认证基于 SRP-6a（RFC 5054 2048-bit 群组）。三种方式按优先级评估：

| 方式 | 条件 | 说明 |
|------|------|------|
| Bearer Token | `SRPVerifier` 已设置 | SRP-6a 握手（`/api/auth/srp/init` + `/api/auth/srp/verify`）验证密码后颁发 32 字节随机 Bearer token；通过 `Authorization: Bearer <token>` 传递；24 小时 TTL |
| HTTP Basic Auth | `AUTH_USER` 环境变量 | 环境变量指定用户名密码 |
| 开放访问 | 以上均未配置 | Keyless 模式，无认证 |

Bearer token 存储于内存两个同步 map（`activeTokens` + `activeTokenExpiry`，各限 1000 条），不持久化到磁盘。

**容量限制行为**：
- `srpSessions` 握手会话最多 200 条（5 分钟 TTL，每 2 分钟清理）。超过上限时 `/api/auth/srp/init` 返回 503，不计入 IP 失败计数，响应体为通用 `{"error":"service_unavailable"}`。
- `activeTokens` Bearer token 最多 1000 条（24 小时 TTL，每 10 分钟清理）。超过上限时 `/api/auth/srp/verify` 返回 503，响应体同为 `{"error":"service_unavailable"}`。额外有**每 IP 最多 `maxTokensPerIP`（10）条**的限制，防止单一 IP 耗尽全局 cap 导致所有用户无法登录的 DoS。
- `GET /api/auth/status` 端点**不**施加 `applyAuthDelay`。该端点返回 `pbkdf2Salt`（固定值，不可暴力破解）和 `initialized`（布尔值，登录 UI 已公开），延迟对此无安全收益。更重要的是，延迟会引起竞态：若 IP 已累积失败次数，延迟可能超过前端 handleUnlock 的执行时间，导致 `serverInitialized` 仍为 false，SRP 握手被跳过。
- 两种 cap 均使用不透明错误消息，无法从外部区分"容量耗尽"与其他服务端错误，避免攻击者探测内部状态。

### 4.2 MCP 认证

- MCP Token 独立于 Session Bearer Token，由服务端生成（48 字符随机），仅存储 SHA-256 哈希。
- 生成时返回一次明文，之后不可再次获取。
- 未配置 MCP Token 时，MCP 端点返回 401。

### 4.3 WebDAV 认证

WebDAV 使用独立的静态 token（与 SRP Session Token 无关）：

- 通过 Settings → Security 生成，原文仅展示一次（关闭面板后清除）。
- 服务端存储 SHA-256 哈希（`webdavTokenHash` 字段，跨重启持久化）。
- 认证方式：HTTP Basic Auth，用户名任意，密码为 raw token 原文。
- 服务端对提交密码计算 SHA-256 后与 `webdavTokenHash` 进行常量时间比对。
- 访问规则：已设置 `webdavTokenHash` → 必须提供 token；已设置 `SRPVerifier` 但未设置 `webdavTokenHash` → 拒绝所有请求；均未设置 → 开放访问（keyless 模式）。
- 共享主认证的 IP 级退避机制。

### 4.4 暴力破解防护

所有认证端点（API / Basic Auth / MCP / WebDAV）共享同一套 IP 级防护：
- 3 次以上连续失败 → 每次增加 500ms 延迟（上限 5s）
- 并发认证延迟限流：同时进行的延迟最多 20 个（信号量），超过上限时**跳过本次 sleep**（延迟绕过），但 `recordAuthFailure` 仍然执行，累计失败计数不受影响
- `authFailures` 计数 30 分钟无活动后清除该 IP
- `authFailures` 表最多记录 **1000** 个 IP（`maxAuthFailureEntries`）；超过上限时新 IP 的失败计数被静默忽略，已记录 IP 不受影响

---

## 5. 输入校验与路径安全

- 文件名严格正则校验：`^[0-9]{8}[a-z0-9]{16}\.(md|png|jpg|jpeg|gif|webp)$`
- 路径拼接后物理字符检查，双重防止路径穿透
- `handleGetNote`/`handleDeleteNote` 使用 `isExposableNote`（黑名单，接受非 canonical 文件名）；`handleGetHistory`/`handleGetVersion`/`handleRollback` 仍使用 `IsValidName`（白名单正则）；`handleSaveNote` 对 canonical 名使用白名单、非 canonical 名使用黑名单（只允许更新，不允许创建）
- 文件写入前配额检查（`CheckNoteQuota` / `CheckAssetQuota` / `CheckStructureQuota`），防止部分写入
- Git 哈希校验：`[0-9a-f]{40}`（40 字符小写十六进制）
- Proxy 信任仅限 `127.0.0.1` 和 `::1`，防止 X-Forwarded-For 伪造

### Diamond DAG DoS 防护

`CheckStructureQuota` 在遍历树形结构时使用全局 `processed` map，防止 diamond 型 DAG 的指数级重复遍历（已认证用户可构造此攻击，修复后时间复杂度降至 O(n)）。

---

## 6. 富文本安全

- 粘贴内容经过 DOM walker 过滤：移除 `<script>`、危险属性（on*）、`javascript:` 协议链接
- 导出 HTML/PDF 经过 `sanitizeExportHtml`：额外阻止 `<svg>`、`<style>`、`<iframe>` 标签、`data:` URI 及 CSS `url()` 外链
- Mermaid SVG 渲染后二次净化：移除 `<script>`、`<foreignObject>`、`<use>` 及 on* 属性
- 外部 HTTP 图片默认阻断（SEC-016），需用户在设置中显式允许
- 表格复制（TableOverlay）仅使用现代 `ClipboardItem` API（HTTPS 环境），已移除 `execCommand` 回退——`execCommand` 会将 HTML 写入系统剪贴板，在某些场景下存在 XSS 风险

---

## 7. MCP 访问控制

- **策略模型**：MCPPolicy 定义 enabled / defaultAccess / rules 三级结构
- **规则匹配**：支持 tag / note_id / title_glob / subtree_of 四种条件，first-match-wins 语义
- **权限级别**：deny（不可见）/ read（只读）/ write（可修改）
- **加密处理**：ENC1 前缀的笔记对 MCP 不透明，无法检查标题/标签内容
- **结构遍历**：subtree_of 匹配使用 BFS + 环检测，防止循环结构导致无限遍历
- **策略清洗**：`sanitizeMCPPolicy()` 在保存前移除无效规则和非法 access 值

---

## 8. 依赖安全

- `go-git v5.13.2`（CVE-2024-21484 已修复）
- 随机字节生成全部使用 `crypto/rand`（后端）和 `window.crypto.getRandomValues`（前端）
- generateId 使用 rejection sampling 消除模偏差（36 字符集，拒绝字节值 ≥ 252）

---

## 9. 已知限制与接受的风险

| 项目 | 状态 | 说明 |
|------|------|------|
| ~~WebAuthn challenge 全零~~ | ✅ 已修复 | 注册和断言均使用 `crypto.getRandomValues(new Uint8Array(32))` 生成随机 challenge |
| git 历史保留历史明文 | 📝 已记录 | 开启 E2EE 之前保存的笔记版本永久保留明文历史；UI 已显示警告；彻底修复需清除 git 历史 |
| batchUpdateEncryption 无原子性 | 📝 已记录 | 批量加密部分失败时服务端处于混合状态；已有失败计数提示和重试入口，无法彻底原子化 |
| sessionWrapKey PBKDF2 10,000 次迭代（固定盐） | 📝 已记录 | 已从 1,000 提升至 10,000 次；盐固定为 `'yinmo-session-wrap-v1'`（非随机）；XSS 场景下 window.name 和 sessionStorage 均可被读取，迭代数和固定盐的实际防护意义有限（代码注释已说明此权衡）；主密钥的 non-extractable 是最终防线 |
| Bearer Token 在无 HTTPS 时明文传输 | 📝 已记录 | 强烈建议搭配内置 TLS 或 Nginx TLS 或 Tailscale 使用，不要将裸 HTTP 暴露在公网 |
| authFailures 按 IP 空闲超时 | 📝 已记录 | 30 分钟无活动后清除该 IP 计数；清除后该 IP 可再次触发延迟计数，单用户私有部署场景可接受；`authFailures` 表上限 1000 条（`maxAuthFailureEntries`），超出后新 IP 的失败计数被静默忽略 |
| IndexedDB 缓存在非正常退出时可能残留 | 📝 已记录 | 加密模式下 lockLibrary 清空缓存，但直接关闭标签页时 `beforeunload` 不保证执行；缓存存储的是 ENC1 密文而非明文，风险可控 |
| `E2E_RESET_AUTH=1` 开放无认证重置端点 | 📝 已记录 | `E2E_RESET_AUTH=1` 时注册 `POST /api/test/reset-auth`，无需 Bearer token 即可清除 SRP 凭据；该变量仅供 E2E 测试环境使用，**严禁在生产部署中设置**；端点在运行时额外检查请求 IP 必须为 loopback（127.0.0.1/::1），详见 TD-M3-034 |
| WebDAV `PROPFIND Depth: infinity` 被拒绝 | 📝 已记录 | 所有 `Depth: infinity` 的 PROPFIND 请求返回 `403 Forbidden`（RFC 4918 §9.1 允许此行为）；部分 WebDAV 客户端（如某些 Obsidian 插件版本）依赖 infinity 深度进行初始化同步，使用前请确认客户端兼容性 |
