# tests/

本目录包含 YinMoNote 的全部测试基础设施。

---

## 目录结构

```
tests/
├── test.sh                   # 统一测试入口（backend / frontend / e2e）
├── run-e2e.sh                # E2E 测试运行器（test.sh 也会调用它）
├── gen-stress-data.sh        # 压测数据生成器
├── Dockerfile.playwright     # Playwright 镜像（yinmonote-playwright:latest）
├── Dockerfile.playwright-patch  # 快速补丁镜像（仅刷新测试脚本，跳过完整重建）
├── docker-compose.e2e.yml    # E2E 双容器编排（app + playwright）
└── e2e/
    ├── package.json
    ├── playwright.config.ts
    ├── fixtures.ts
    ├── helpers/
    │   └── app.ts            # 公共封装：freshPage、unlock、createNote 等
    └── specs/
        ├── 01-unlock.spec.ts           # 初次解锁、密码 / WebAuthn 流程
        ├── 02-notes-crud.spec.ts       # 新建 / 编辑 / 删除笔记
        ├── 03-editor-features.spec.ts  # 格式化、代码块、导出
        ├── 04-sidebar-search.spec.ts   # 标题搜索、标签过滤
        ├── 05-settings.spec.ts         # 外观 / 编辑器 / 配额设置
        ├── 06-history.spec.ts          # 版本历史、diff、回滚
        ├── 07-lock-unlock.spec.ts      # 锁定 / 重新解锁
        ├── 08-server-encrypt.spec.ts   # serverEncrypt 模式
        ├── 09-encryption-modes.spec.ts # 加密模式切换
        ├── 10-coverage.spec.ts         # 空搜索、多笔记持久化、保存状态
        ├── 11-multi-device.spec.ts     # 新设备登录、并发会话、重锁重解
        └── 99-screenshots.spec.ts      # 截图捕获（仅 CI 环境）
```

各 spec 的用例数详见 [docs/testing-guide.md](../docs/testing-guide.md)。

单元测试与各自的源码放在一起：

```
frontend/tests/         # Vitest 单元测试
backend/*_test.go       # Go 单元测试（backend/ 包）
tests/unit/webdav/      # Go 单元测试（WebDAV 标题虚拟化——需单独运行）
```

---

## 运行测试

所有测试均在 Docker 容器内运行，确保环境一致性。运行前需安装并启动 Docker。

### 单元测试

```bash
# 后端 + 前端（默认）
./tests/test.sh

# 仅后端（Go test）
./tests/test.sh backend

# 仅前端（Vitest）
./tests/test.sh frontend
```

`test.sh` 采用两阶段流程：先 `docker build --target <stage> --quiet` 构建测试镜像
（静默模式，充分利用 BuildKit 缓存），再 `docker run --rm` 获得不含构建噪声的纯净输出。

单元测试也会在 `make docker` / `make build` 过程中自动运行——测试失败则镜像无法产出。

### E2E 测试

```bash
# 完整运行——构建镜像 + 执行所有 spec
./tests/test.sh e2e
./tests/run-e2e.sh          # 等价

# 跳过镜像重建（仅测试代码有变动时加速重跑）
./tests/run-e2e.sh --no-build

# 只运行指定 spec 文件
./tests/run-e2e.sh specs/06
./tests/run-e2e.sh --no-build specs/06 specs/07

# 查看 HTML 测试报告（含截图、时间线）
open tests/e2e/playwright-report/index.html
```

**使用的 Docker 镜像：**
- `yinmonote:e2e` — 被测应用（以 `VITE_PBKDF2_ITERATIONS=1000` 构建，加速加密操作）
- `yinmonote-playwright:latest` — Playwright 执行器

两个镜像在每次测试结束后均由 cleanup trap 自动删除。

---

## 压力测试数据

生成覆盖所有支持的 Markdown 语法元素（H1–H6、内联格式、代码块、KaTeX 数学公式、
Mermaid 图表、Callout 块、折叠块、表格等）的大批量笔记：

```bash
./tests/gen-stress-data.sh --help

# 示例
./tests/gen-stress-data.sh                                         # 默认：100 篇，20–200 KB，中英混合
./tests/gen-stress-data.sh --notes 1000                           # 1000 篇
./tests/gen-stress-data.sh --notes 500 --lang zh                  # 中文内容
./tests/gen-stress-data.sh --notes 200 --min-size 50 --max-size 300 --data-dir /tmp/stress
```

脚本直接将文件写入目标笔记目录，并自动调高 `config.json` 中的配额限制以接受批量导入。

---

架构设计、覆盖范围说明、设计原则和用例数量汇总，请参阅
[docs/testing-guide.md](../docs/testing-guide.md)。
