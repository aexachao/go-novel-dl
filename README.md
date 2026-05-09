# go-novel-dl (aexachao fork)

<p align="center">
  <img src="./internal/web/templates/icon-256.png" alt="Novel Downloader Icon" width="220" />
</p>

本项目是 [guohuiyuan/go-novel-dl](https://github.com/guohuiyuan/go-novel-dl) 的 fork，在上游基础上新增了**多用户认证系统**、**配额限制**与 **iOS App 对接支持**。

> ⚠️ **已迁移至 GitHub**: https://github.com/aexachao/go-novel-dl
> Docker Hub: `aexachao/go-novel-dl:latest`

---

## 核心能力（上游 + 本 fork 定制）

### 基础功能（来自上游 v1.0.7）

- **聚合搜索**：并发搜索多个站点，按书名/作者归并同作品变体
- **混合结果排序**：结合关键词匹配、站点优先级、简介完整度、封面可用性选出主结果
- **URL 直达**：CLI 下载和 Web 搜索都支持直接输入站点链接进行解析
- **详情页预取**：Web 详情通过 `DownloadPlan` 拉取目录与书籍元数据
- **Web 阅读器**：支持按需加载章节正文、上下文预加载、滚动续读、主题/背景/字号和章节排版设置
- **Web 内容缓存**：详情页和章节正文带 TTL 缓存与并发请求合并，减少重复抓取
- **异步下载**：Web 下载任务异步执行，通过轮询查询进度与导出文件
- **分阶段存储**：原始数据、处理后数据、导出文件分层保存
- **多格式导出**：`txt`、`html`、`epub`
- **站点兼容**：Alice Book House 加密章节接口、Linovelib 多页目录、N23QB 站点地图搜索等

### 本 fork 定制功能

- **🔐 多用户认证系统**：JWT 登录 + API Key，支持注册/登录/Token 管理
- **📊 配额系统**：Free 用户每日 50 次搜索 / 5 次下载，Pro 用户 500/50，配额每日 UTC 0 点重置
- **📱 iOS App 对接**：完整 iOS 集成指南（Token 管理、API Client、SwiftUI ViewModel、Keychain 存储），详见 [docs/ios-integration.md](docs/ios-integration.md)
- **🐳 aarch64 原生支持**：Docker 镜像兼容 ARM64 服务器（如 Oracle Cloud A1）

---

## 快速部署

### Docker 部署（推荐）

```bash
# 免认证模式（自建服务）
docker run -d -p 4397:8080 \
  -v $(pwd)/data:/app/data \
  aexachao/go-novel-dl:latest web --port 8080

# 启用认证模式（共享服务）
docker run -d -p 4397:8080 \
  -v $(pwd)/data:/app/data \
  aexachao/go-novel-dl:latest web --port 8080 \
    --auth \
    --auth-db /app/data/auth.db \
    --jwt-secret "your-secret-change-me"
```

### Docker Compose 部署（带认证）

```yaml
services:
  go-novel-dl:
    image: aexachao/go-novel-dl:latest
    container_name: go-novel-dl
    restart: unless-stopped
    ports:
      - "4397:8080"
    volumes:
      - ./data:/app/data
    command: web --port 8080 --auth --auth-db /app/data/auth.db --jwt-secret "change-me-in-production"
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/novel/healthz"]
      interval: 30s
      timeout: 10s
      retries: 3
```

```bash
docker compose up -d
# 访问 http://localhost:4397/novel
```

### Docker 镜像说明

- **镜像名**：`aexachao/go-novel-dl:latest`
- **架构**：同时支持 `linux/amd64` 和 `linux/arm64`（aarch64）
- **默认端口**：容器内 `8080`，映射到主机 `4397`
- **数据卷**：`/app/data` — 包含 `site_catalog.db`、`auth.db`、下载文件等

---

## 认证与配额

### 认证模式

启动时添加 `--auth` 参数即可开启多用户认证：

```bash
./novel-dl web --port 8080 --auth --auth-db ./data/auth.db --jwt-secret "your-secret"
```

| 参数 | 说明 |
|---|---|
| `--auth` | 启用认证模式 |
| `--auth-db` | 用户数据库路径（SQLite） |
| `--jwt-secret` | JWT 签名密钥（生产环境请使用随机字符串） |

### 用户配额

| 计划 | 每日搜索 | 每日下载 | 并发任务 | 站点范围 |
|---|---|---|---|---|
| **Free** | 50 | 5 | 1 | 主流量站点 |
| **Pro** | 500 | 50 | 3 | 全部站点 |

配额在每日 UTC 0 点重置。

### API Key

用户可以在设置页生成 API Key，用于第三方应用或服务器间调用（永久有效，可撤销）。

---

## 内置站点

### 已注册站点（20+）

```
alicesw   esjzone    yibige     linovelib  n23qb
biquge345 fsshu      n69shuba   ixdzs8     novalpie
ruochu    n17k       hongxiuzhao fanqienovel faloo
wenku8    sfacg      ciyuanji   ciweimao   n8novel
shuhaige
```

### 支持搜索 + 下载

```
alicesw   esjzone    linovelib  n23qb      fsshu
ixdzs8    n17k       n8novel    sfacg      shuhaige
ciyuanji  ciweimao   ruochu     faloo      n69shuba
```

### 仅支持下载

```
fanqienovel  hongxiuzhao  novalpie  wenku8  yibige
```

---

## API 端点概览

所有 API 前缀：`/novel/api/`

| 功能 | 端点 | 认证 | 说明 |
|---|---|---|---|
| 服务元信息 | `GET /api/meta` | ❌ | 返回 `auth_enabled` 等配置 |
| 注册 | `POST /api/auth/register` | ❌ | 返回 JWT |
| 登录 | `POST /api/auth/login` | ❌ | 返回 JWT |
| 当前用户+配额 | `GET /api/auth/me` | ✅ | |
| 聚合搜索 | `POST /api/search` | ✅ | 消耗搜索配额 |
| 书籍详情 | `GET /api/books/detail` | ✅ | 消耗搜索配额 |
| 创建下载任务 | `POST /api/download-tasks` | ✅ | 消耗下载配额 |
| 查询任务进度 | `GET /api/download-tasks/:id` | ✅ | |
| 下载导出文件 | `GET /api/download-file` | ✅ | |
| 健康检查 | `GET /healthz` | ❌ | |

详细 API 文档和 iOS 集成示例见 [docs/ios-integration.md](docs/ios-integration.md)。

---

## 构建

### 本地构建

```bash
go build ./...
go build -o novel-dl ./cmd/novel-dl
```

### Docker 构建

```bash
docker build -t go-novel-dl:latest .
```

### 运行测试

```bash
go test ./...
```

---

## 数据目录

```
data/
├─ site_catalog.db          SQLite 配置中心
├─ auth.db                  用户认证数据库（启用 --auth 时）
├─ raw_data/<site>/<book_id>/
│  ├─ book_info.raw.json
│  ├─ chapters.raw.sqlite
│  └─ pipeline.json
├─ downloads/<site>/         导出文件
├─ novel_cache/              运行缓存
├─ logs/                     调试日志
└─ state.json                轻量状态文件
```

---

## 免责声明

- 本项目仅供学习、研究与个人技术验证使用
- 请遵守目标站点服务条款、版权要求与当地法律法规
- 部分站点可能受限流、Cloudflare、登录态、反爬或网络连通性影响

---

## 相关文档

- [iOS App 集成指南](docs/ios-integration.md) — 完整的 iOS Swift/SwiftUI 对接文档
- [上游架构文档](docs/architecture.md) — 来自上游的架构说明

---

## 版本历史

### 本 fork 定制

- **合并上游 v1.0.7**：Web 阅读器、ESJ Zone 免登录优化、Alice Book House 加密章节、内容缓存
- **新增多用户认证**：JWT + API Key，配额系统（Free/Pro）
- **新增 iOS 集成支持**：完整 Swift/SwiftUI 客户端示例
- **aarch64 Docker 支持**：移除 `GOARCH=amd64`，支持 ARM64 部署

### 上游 v1.0.7（guohuiyuan/go-novel-dl）

- Web 新增章节正文接口 `/novel/api/chapter-content`，内置阅读器按需加载
- 阅读器支持上下文预加载、滚动触发加载、主题/背景/字号调整
- Alice Book House 支持加密章节 RSA/AES 解密
- 多个站点兼容性与稳定性优化
