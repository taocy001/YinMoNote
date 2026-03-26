# build/

YinMoNote 的构建脚本、Dockerfile 和服务配置文件。

---

## 文件说明

| 文件 | 作用 |
|---|---|
| `Dockerfile` | 多阶段镜像：前端构建 → 后端构建 → 生产运行时 |
| `docker-compose.yml` | 生产容器定义（供 `install-docker.sh` 使用） |
| `build.sh` | 编译原生二进制（Linux / macOS，无 Go/Node 时自动回退到 Docker） |
| `package-docker.sh` | 构建 Docker 镜像并保存为可分发的 `.tar` |
| `package-deb.sh` | 将 Linux 二进制打包为 `.deb` 安装包 |
| `package-dmg.sh` | 将 macOS 二进制打包为 `.dmg` 安装包 |
| `install.sh` | 原生交互式安装向导（数据目录 + 端口 + systemd / LaunchAgent） |
| `install-docker.sh` | Docker 交互式安装向导（加载镜像、启动容器） |
| `yinmonote.service` | systemd 用户服务模板（由 Linux 上的 `install.sh` 使用） |

---

## 构建命令

所有 `make` 目标均委托给本目录中的脚本执行。

```bash
make build            # 当前平台的原生二进制 → dist/yinmonote-<os>-<arch>
make docker           # Docker 镜像 → dist/yinmonote-<version>-docker-<arch>.tar
make release          # 所有平台：docker×2 + .deb×2 + .dmg → dist/
make install          # 构建 + 交互式原生安装（systemd / LaunchAgent）
make install-docker   # make docker + 交互式 Docker 安装
make clean            # 清理 dist/ 和 backend/dist/
```

强制使用 Docker 构建（适用于本地未安装 Go / Node 的情况）：

```bash
DOCKER=1 make build
```

直接调用 `build.sh` 进行交叉编译：

```bash
./build/build.sh linux  amd64   # Linux x86-64
./build/build.sh linux  arm64   # Linux ARM64（树莓派、AWS Graviton）
./build/build.sh darwin arm64   # macOS Apple Silicon
```

---

## Dockerfile — 多阶段构建架构

```
┌─────────────────────────────────────────────────────────────────────┐
│ 阶段：frontend-builder                                              │
│   基础镜像：node:20-alpine（阿里云镜像）                             │
│   npm install（npmmirror）→ npm run test → npm run build            │
│   产出：/app/dist（打包后的 Vue SPA）                                │
├─────────────────────────────────────────────────────────────────────┤
│ 阶段：backend-builder                                               │
│   基础镜像：golang:1.21-alpine（阿里云镜像）                         │
│   GOPROXY=goproxy.cn                                                │
│   go mod download → go test ./... → go build                        │
│   通过 go:embed 嵌入 /app/dist → 单文件自包含二进制                 │
│   产出：/app/YinMoNote                                              │
├─────────────────────────────────────────────────────────────────────┤
│ 阶段：frontend-test（继承 frontend-builder，CMD: npm run test）      │
│ 阶段：backend-test （继承 backend-builder， CMD: go test）           │
│   供 tests/test.sh 使用，获得无构建噪声的纯净测试输出               │
├─────────────────────────────────────────────────────────────────────┤
│ 阶段：生产运行时                                                     │
│   基础镜像：debian:bookworm（阿里云 apt 镜像）                       │
│   以非 root 用户 yinmonote（UID/GID 1000）运行                      │
│   复制 /app/YinMoNote → /home/yinmonote/YinMoNote                   │
│   EXPOSE 8080，CMD ./YinMoNote                                       │
└─────────────────────────────────────────────────────────────────────┘
```

单元测试嵌入在构建流程中：任意阶段测试失败都会中止镜像构建，`make docker` 本身即充当 CI 门禁。

---

## Docker 镜像命名规范

| 镜像 | 由谁构建 | 生命周期 |
|---|---|---|
| `yinmonote:<version>` / `yinmonote:latest` | `package-docker.sh` | 持久，每次新构建覆盖旧版本 |
| `yinmonote:e2e` | `tests/run-e2e.sh` | 临时，每次 E2E 测试结束后自动删除 |
| `yinmonote-playwright:latest` | `tests/run-e2e.sh` | 临时，同上 |
| `yinmonote-test-backend:latest` | `tests/test.sh` | 覆盖式保留（加速重跑） |
| `yinmonote-test-frontend:latest` | `tests/test.sh` | 覆盖式保留（加速重跑） |
| `yinmonote-build:tmp` | `build.sh`（DOCKER=1） | 提取二进制后立即删除 |

`package-docker.sh` 在重新构建前先删除同名旧镜像，保存 tar 后删除新镜像——`.tar` 才是分发产物，本地镜像只是中间产物。

---

## 应用环境变量

运行时传给容器或原生二进制（非构建时参数）。

| 变量 | 默认值 | 说明 |
|---|---|---|
| `DATA_DIR` | `~/.yinmonote/notes` | 笔记、资产和 Git 历史的存储目录 |
| `CONFIG_FILE` | `~/.yinmonote/config.json` | 应用配置（配额、外观等） |
| `PORT` | `:8080` | 监听地址，格式 `:端口号` |
| `TZ` | — | 时区（如 `Asia/Shanghai`） |
| `SYNC_COMMIT` | — | 设为 `1` 时每次保存立即提交 Git（E2E 测试用） |
| `ACME_DOMAIN` | — | 通过 Let's Encrypt 自动申请 TLS 证书（需 443 端口） |
| `TLS_CERT` / `TLS_KEY` | — | 自带 TLS：PEM 证书和私钥文件路径 |
| `TLS_SELF` | — | 设为 `1` 时自动生成自签名证书（适合局域网 / 内网）。**注意：**指纹/面容解锁（WebAuthn）需要域名，通过 IP 地址访问时该选项自动隐藏。如需指纹解锁，请使用 `ACME_DOMAIN` 或反向代理绑定域名。 |
| `TLS_EXTRA_IPS` | — | 自签名证书额外 SAN IP，逗号分隔 |
| `WEBDAV_DISABLED` | — | 设为 `1` 时禁用 WebDAV 端点（`/dav/`） |
| `ALLOWED_ORIGIN` | — | CORS 来源（仅前后端跨域时需要） |

`VITE_PBKDF2_ITERATIONS` 是**构建时**参数（默认 100 000；E2E 构建通过 `docker-compose.e2e.yml` 设为 1 000 以防止测试超时）。

---

## 构建产物

所有产物输出到 `dist/`：

| 文件 | 平台 |
|---|---|
| `yinmonote-linux-amd64` | Linux x86-64 原生二进制 |
| `yinmonote-linux-arm64` | Linux ARM64 原生二进制 |
| `yinmonote-darwin-arm64` | macOS Apple Silicon 原生二进制 |
| `yinmonote-<ver>-docker-amd64.tar` | Docker 镜像（linux/amd64） |
| `yinmonote-<ver>-docker-arm64.tar` | Docker 镜像（linux/arm64） |
| `yinmonote_<ver>_amd64.deb` | Debian/Ubuntu 安装包（x86-64） |
| `yinmonote_<ver>_arm64.deb` | Debian/Ubuntu 安装包（ARM64） |
| `yinmonote-<ver>-macos-arm64.dmg` | macOS 安装包 |

---

快速开始指南参见项目 [README.zh.md](../README.zh.md)。
