# YinMoNote 测试体系

> 如需运行命令和目录结构，参见 [tests/README.zh.md](../tests/README.zh.md)。

---

## 1. 分层设计

共三层测试，各有明确的职责边界，全部在 Docker 容器内运行以保证环境一致性。

```
┌─────────────────────────────────────────────────────────────────┐
│  E2E 测试（Playwright + Chromium）                               │
│  真实浏览器 → 真实后端 → 真实 Git，验证完整用户场景               │
├─────────────────────────────────────────────────────────────────┤
│  前端单元测试（Vitest + happy-dom）                              │
│  加密逻辑、状态管理、i18n，隔离于 DOM 环境中验证                  │
├─────────────────────────────────────────────────────────────────┤
│  后端单元测试（Go test + httptest）                              │
│  HTTP handler、配额校验、安全边界，进程内验证                     │
└─────────────────────────────────────────────────────────────────┘
```

**单元测试内嵌于构建阶段**：`build/Dockerfile` 在构建生产镜像的过程中自动运行前端 Vitest 和后端 Go test——测试失败则镜像无法产出，这是最轻量的持续集成形式。

**E2E 测试**使用 Playwright 驱动无头 Chromium，对运行中的生产镜像发起真实 HTTP 请求。两个容器通过独立 Docker 网络通信：
- `yinmonote:e2e`：E2E 专用应用镜像（`VITE_PBKDF2_ITERATIONS=1000`，加速加密操作），健康检查通过后才允许测试开始
- `yinmonote-playwright:latest`：Playwright runner，访问 `http://app:8080`

---

## 2. 后端单元测试覆盖范围

**框架**：Go 标准 `testing` + `testify/assert` + `net/http/httptest`

**文件**：`backend/main_test.go`（85 个函数）、`backend/mcp_test.go`（18 个函数 / 73 个子测试）、`backend/perf_test.go`（1 个 Benchmark）

**WebDAV 专项测试**（`tests/unit/webdav/`，26 个函数）：独立 Go 包，需单独运行（`cd tests/unit/webdav && go test ./...`），**不在** `./tests/test.sh backend` 的扫描范围内。

| 文件 | 函数数 | 覆盖内容 |
|---|---|---|
| `dav_title_test.go` | 16 | 标题虚拟化：PROPFIND listing、GET/PUT/DELETE by title、MOVE→H1 更新、重名去重、非 canonical 直通、配额拦截、标题截断、空标题降级 |
| `m3_gaps_test.go` | 7 | 路径深度拒绝（depth 6/7）、vault 前缀剥离（normalizePath）、Remotely Save probe 序列 |
| `round3_gaps_test.go` | 3 | 内部文件屏蔽（_structure.json）、隐藏文件拒绝、深路径拒绝 |

> **注意**：以下测试计数以代码为准，可通过 `grep -c '^func Test' backend/*_test.go` 和 `npx vitest --reporter=verbose` 验证。

覆盖内容：

- HTTP handler 的输入校验与安全边界（路径穿透、文件名格式、内容类型）
- NoteLibrary 的文件原子写、配额逻辑、结构完整性保护
- 配置 clamp（防止手动改配置文件绕过配额）
- 安全响应头（CSP、X-Frame-Options、X-Content-Type-Options、Referrer-Policy、Permissions-Policy）
- 加密随机字符串的均匀分布验证
- MCP 策略求值（tag / note_id / title_glob / subtree_of 规则、first-match-wins、循环结构防护）
- MCP 令牌生成与撤销（hash 存储、`/api/config` 仅暴露布尔值）
- MCP 认证中间件（未配置 → 503、错误令牌 → 401）
- MCP 策略清洗（sanitizeMCPPolicy：无效 access、空条件规则）
- MCP 工具参数校验（畸形 JSON、无效 search_in、未知工具）
- MCP 结构加载（缺失文件、有效 JSON、ENC1 前缀、畸形 JSON）

---

## 3. 前端单元测试覆盖范围

**框架**：Vitest + happy-dom（模拟 DOM 和 Web Crypto API）

| 文件 | 用例数 | 覆盖内容 |
|------|--------|----------|
| `crypto.test.ts` | 45 | 密钥派生（PBKDF2）、AES-GCM 加解密全流程、keyless 模式、JWK 导入导出、legacy salt 迁移、大负载加密 |
| `useLibrary.test.ts` | 97 | 结构加载与 sanitize、displayList 过滤与虚拟分页、ENC1 解包、createNewNote / deleteNote / setNoteTags / moveNote 边界、toggleCollapse、hasServerNotes、togglePin、restoreNote、permanentDeleteNote、emptyTrash、setCommitLabel |
| `i18n.test.ts` | 30 | 中英文切换、localStorage 持久化、侧边栏/编辑器/Slash 菜单/密钥管理/移动端工具栏翻译、完整性检查 |
| `useExport.test.ts` | 28 | sanitizeExportHtml（script/svg/style/iframe/on\*/javascript:/data: URI/CSS url() 拦截）、escapeHtml 实体编码、exportMarkdown/exportHTML 触发下载 |
| `blockExtensions.test.ts` | 26 | Callout/ToggleBlock Markdown 序列化、parseHTML 属性提取、emoji/title HTML 转义、CALLOUT_DEFAULTS |
| `useWordStats.test.ts` | 16 | CJK/Latin/混合词数统计、韩文/扩展A/兼容汉字、readMin 估算、reactive 更新 |
| `findReplace.test.ts` | 11 | collectMatches 空搜索、大小写不敏感、多节点、CJK、非文本节点跳过、非贪婪重叠 |
| `useDragDrop.test.ts` | 9 | 侧边栏宽度拖拽、isDragging 状态、minWidth 最小值夹紧、多次拖拽事件 |
| `useImageDecrypt.test.ts` | 6 | data URL 直通、外部 http 图片拦截（SEC-016）、ENC1 检测、锁定库行为、axios 错误降级 |
| `useBatchEncryption.test.ts` | 5 | 锁定时中止、明文笔记加密、不重复加密已加密笔记、部分失败报告、onComplete 回调 |
| `batchImport.test.ts` | 32 | 批量导入文件/文件夹/ZIP、文件名冲突、进度回调、错误处理 |
| `designSystem.test.ts` | 62 | CSS 变量完整性（type scale/动效/语义色/surface）、工具类、字号标准化 |
| `commandPalette.test.ts` | 27 | 模式切换、命令过滤、笔记搜索、最近访问、键盘导航、事件触发、ARIA 无障碍 |

---

## 4. E2E 测试覆盖范围

**框架**：Playwright + Chromium

| 文件 | 用例数 | 覆盖内容 |
|------|--------|----------|
| `01-unlock.spec.ts` | 9 | 解锁模态框、模式标签页、keyless 流程、密码输入 |
| `02-notes-crud.spec.ts` | 12 | 笔记创建/读取/更新/删除、保存状态、侧边栏交互 |
| `03-editor-features.spec.ts` | 13 | TOC、历史、专注模式、导出、快捷键、格式化、字数统计 |
| `04-sidebar-search.spec.ts` | 9 | 搜索过滤、内容匹配徽标、标签编辑、子笔记 |
| `05-settings.spec.ts` | 16 | 主题切换、语言切换、字号、打字机模式、安全标签页 |
| `06-history.spec.ts` | 9 | 版本历史列表、diff 视图、回滚操作 |
| `07-lock-unlock.spec.ts` | 8 | 密码模式锁定/解锁、错误处理 |
| `08-server-encrypt.spec.ts` | 9 | serverEncrypt 开关、内容加密验证、数据完整性 |
| `09-encryption-modes.spec.ts` | 5 | 加密模式切换往返、round-trip 验证 |
| `10-coverage.spec.ts` | 7 | 测试覆盖验证 |
| `11-multi-device.spec.ts` | 13 | 多设备认证、跨设备同步 |
| `99-screenshots.spec.ts` | 2 | 截图捕获 |

### E2E 测试设计原则

- **keyless 模式**：所有 spec 使用无加密模式，消除 PBKDF2 派生的性能开销，专注于功能路径
- **隔离**：每个测试用 `freshPage()` 清空 localStorage / sessionStorage，保证用例间互不干扰
- **顺序模拟真实路径**：spec 文件按数字前缀排序，模拟"解锁 → 编辑 → 历史 → 锁定"的完整使用路径
- **即时 Git 提交**：E2E 环境启用 `SYNC_COMMIT=1`，每次保存立即提交，使历史类测试可即时读到版本记录
- **cleanup trap**：`trap cleanup EXIT` 保证在任何退出路径（成功、失败、Ctrl+C）后都执行 `docker compose down -v`，同时删除 E2E 镜像

**调试技巧**：
- 失败用例的截图保存在 `tests/e2e/test-results/`（已 gitignore，仅本地）
- HTML 报告的时间线可定位具体失败步骤：`open tests/e2e/playwright-report/index.html`
- 如需增加超时，修改 `tests/e2e/playwright.config.ts` 中的 `timeout`

---

## 5. 压力测试基准

`tests/gen-stress-data.sh` 生成覆盖所有支持 Markdown 语法的大批量笔记，用于验证侧边栏虚拟滚动、搜索性能和解密延迟。

| 指标 | 目标 |
|------|------|
| 1000 篇笔记侧边栏首屏渲染 | < 200 ms（虚拟滚动分页） |
| 单篇 512 KB 笔记解密 | < 50 ms |
| 标题搜索防抖响应 | ~200 ms |

---

## 6. 用例数量汇总

| 层级 | 框架 | 用例数（约）| 验证命令 |
|------|------|------------|----------|
| 后端单元（backend/） | Go test | ~240 | `./tests/test.sh backend` |
| 后端单元（WebDAV） | Go test | 26 | `cd tests/unit/webdav && go test ./...` |
| 前端单元 | Vitest | ~400 | `./tests/test.sh frontend` |
| E2E | Playwright | ~110 | `./tests/test.sh e2e` |
| **合计** | | **~750** | `./tests/test.sh` |
