# go-novel-dl API Reference

> Base URL: `http://<your-server>:4397/novel`
>
> 所有需要认证的接口（除 `/api/auth/*` 公开接口外），请求 Header 必须携带：
> ```
> Authorization: Bearer <token>
> ```

---

## 认证接口（公开）

### 获取访客 Token
> 启动时调用，所有操作走这个 Token（Free 配额）

```
GET /novel/api/auth/guest-token
```

**响应 200:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "token_type": "access"
}
```

---

### 注册账户
```
POST /novel/api/auth/register
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "yourpassword"
}
```

**响应 200:**
```json
{ "token": "eyJ...", "token_type": "access" }
```

---

### 登录
```
POST /novel/api/auth/login
Content-Type: application/json

{
  "email": "user@example.com",
  "password": "yourpassword"
}
```

**响应 200:**
```json
{ "token": "eyJ...", "token_type": "access" }
```

---

### 认证健康检查
```
GET /novel/api/auth/health
```

**响应 200:**
```json
{ "status": "ok" }
```

---

## 用户接口（需认证）

### 当前用户信息 + 配额
```
GET /novel/api/auth/me
Authorization: Bearer <token>
```

**响应 200:**
```json
{
  "user": {
    "id": "1778311986916-E9E9E9E9",
    "email": "guest@web.noveldln.local",
    "plan": "free"
  },
  "quota": {
    "plan": "free",
    "search": {
      "used": 22,
      "limit": 50,
      "reset_at": "2026-05-10T08:00:00+08:00"
    },
    "download": {
      "used": 1,
      "limit": 5,
      "reset_at": "2026-05-10T08:00:00+08:00"
    },
    "limits": {
      "DailySearch": 50,
      "DailyDownload": 5,
      "MaxWorkers": 1,
      "AllSites": false
    }
  }
}
```

### 配额套餐

| Plan | 每日搜索 | 每日下载 | 并发上限 | 全部站点 |
|------|---------|---------|---------|---------|
| `free` | 50 | 5 | 1 | 仅默认站点 |
| `pro` | 500 | 50 | 3 | ✅ 全部站点 |
| `unlimited` | 999999 | 999999 | 10 | ✅ 全部站点 |

---

## 书籍搜索

### 搜索
> 需要认证。消耗 1 次搜索配额（成功后）。

```
POST /novel/api/search
Authorization: Bearer <token>
Content-Type: application/json

{
  "keyword": "斗破苍穹",
  "scope": "all",
  "sites": ["esjzone", "novalpie"],
  "page": 1,
  "page_size": 20
}
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `keyword` | string | ✅ | 搜索关键词 |
| `scope` | string | | `"default"`（默认来源）或 `"all"`（全部站点） |
| `sites` | string[] | | 指定站点 key 数组，为空则按 scope 决定 |
| `page` | int | | 页码，默认 1 |
| `page_size` | int | | 每页数量，默认 50 |

**响应 200:**
```json
{
  "keyword": "斗破苍穹",
  "sites": ["esjzone", "novalpie"],
  "results": [
    {
      "key": "斗破苍穹之足诀|苍蓝",
      "title": "斗破苍穹之足诀",
      "author": "苍蓝",
      "description": "足控向的文，作者只想被各种美少女踩来踩去...",
      "cover_url": "https://i.imgs.ovh/2026/02/23/ywqfLU.png",
      "latest_chapter": "第十八章 厄难毒体",
      "preferred_site": "esjzone",
      "primary": {
        "site": "esjzone",
        "book_id": "1771755194",
        "title": "斗破苍穹之足诀",
        "author": "苍蓝",
        "url": "https://www.esjzone.cc/detail/1771755194.html",
        "cover_url": "https://i.imgs.ovh/2026/02/23/ywqfLU.png"
      },
      "variants": [...],
      "source_count": 1,
      "score": 0.928
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 1,
  "total_exact": true,
  "has_prev": false,
  "has_next": false
}
```

**iOS 关键字段：**
- `results[].title` / `author` / `cover_url` → 书架卡片
- `results[].primary.book_id` + `results[].primary.site` → 后续请求的标识

---

### 通过链接直接获取书籍信息
> 如果你已经有目标书籍的链接，可以直接传 URL，接口会自动识别站点并返回书籍详情。消耗 1 次搜索配额。

```
POST /novel/api/search
Authorization: Bearer <token>
Content-Type: application/json

{
  "keyword": "https://www.esjzone.cc/detail/1771755194.html"
}
```

**响应 200:** 同上搜索接口格式（`page: 1, page_size: 1`）

---

## 书籍详情

### 获取书籍详情 + 目录
> 需要认证。消耗 1 次搜索配额。

```
GET /novel/api/books/detail?site=esjzone&book_id=1771755194
Authorization: Bearer <token>
```

| 参数 | 类型 | 必填 |
|------|------|------|
| `site` | string | ✅ 站点 key |
| `book_id` | string | ✅ 书籍 ID |

**响应 200:**
```json
{
  "book": {
    "title": "斗破苍穹之足诀",
    "author": "苍蓝",
    "description": "足控向的文...",
    "cover_url": "https://i.imgs.ovh/2026/02/23/ywqfLU.png",
    "source_url": "https://www.esjzone.cc/detail/1771755194.html",
    "chapters": [
      {
        "id": "486584",
        "title": "第一章 陨落的天才与旁观者",
        "url": "https://www.esjzone.one/forum/1771755194/486584.html",
        "order": 1
      },
      {
        "id": "486585",
        "title": "第二章 夜访与隐秘",
        "url": "https://www.esjzone.one/forum/1771755194/486585.html",
        "order": 2
      }
    ]
  }
}
```

**iOS 关键字段：**
- `book.title` / `author` / `description` / `cover_url` → 详情页
- `book.chapters[].title` → 目录列表
- `book.chapters[].id` + `url` → 获取正文用

---

## 章节内容

### 获取章节正文
> **不需要认证**，但有频率限制（服务端内存缓存）。

```
GET /novel/api/chapter-content
?site=esjzone
&book_id=1771755194
&chapter_id=486584
&title=第一章%20陨落的天才与旁观者
&url=https://www.esjzone.one/forum/1771755194/486584.html
```

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `site` | string | ✅ | 站点 key |
| `book_id` | string | ✅ | 书籍 ID |
| `chapter_id` | string | ✅ | 章节 ID（`id`/`title`/`url` 至少一个） |
| `title` | string | ✅ | 章节标题 |
| `url` | string | ✅ | 章节页面 URL |

> 注：三个章节标识参数 `chapter_id` / `title` / `url` 至少要提供一个，推荐全传以提高识别准确率。

**响应 200:**
```json
{
  "chapter": {
    "title": "第一章 陨落的天才与旁观者",
    "content": "火族的领地内，一场激烈的天才之间的战斗终于落下帷幕...",
    "book_title": "斗破苍穹之足诀"
  }
}
```

**iOS 关键字段：**
- `chapter.content` → 直接显示在阅读器中

---

## 站点信息

### 获取站点元信息
> **不需要认证**。启动时调用一次，缓存到本地。

```
GET /novel/api/meta
```

**响应 200:**
```json
{
  "default_sources": [
    { "key": "esjzone", "display_name": "ESJ Zone", "capabilities": { "search": true, "download": true } },
    { "key": "novalpie", "display_name": "노벨피아", "capabilities": { "search": true, "download": true } }
    // ... 共 15 个默认站点
  ],
  "all_sources": [
    // 全部可用站点，包括非默认的，共 17 个
    { "key": "fsshu", "display_name": "笔趣阁", "default_available": false, ... },
    { "key": "novalpie", "display_name": "노벨피아", "default_available": false, ... }
  ],
  "site_warnings": [],
  "site_stats": [],
  "auth_enabled": true
}
```

**iOS 用途：**
- `default_sources` → 显示默认选中的搜索站点列表
- `all_sources` → 显示完整站点列表供用户选择
- `auth_enabled` → 判断认证功能是否启用

### 当前可用全部站点 Key

| Key | 显示名 | 默认选中 | 类型 |
|-----|-------|---------|------|
| `esjzone` | ESJ Zone | ✅ | 轻小说转载 |
| `linovelib` | 哔哩轻小说 | ✅ | 轻小说 |
| `n23qb` | 铅笔小说 | ✅ | 网文转载 |
| `biquge345` | Biquge345 | ✅ | 网文转载 |
| `ixdzs8` | 爱下电子书 | ✅ | 网文转载 |
| `ruochu` | 若初文学网 | ✅ | 网文 |
| `n17k` | 17K小说网 | ✅ | 网文 |
| `faloo` | 飞卢小说网 | ✅ | 网文 |
| `sfacg` | SF轻小说 | ✅ | 轻小说 |
| `ciyuanji` | 次元姬 | ✅ | 轻小说 |
| `ciweimao` | 刺猬猫 | ✅ | 轻小说 |
| `n8novel` | 无限轻小说 | ✅ | 轻小说 |
| `shuhaige` | 书海阁小说网 | ✅ | 网文转载 |
| `tianyabooks` | 天涯书库 | ✅ | 网文 |
| `alicesw` | 爱丽丝书屋 | ✅ | 成人向 |
| `fsshu` | 笔趣阁 | ❌ | 网文转载 |
| `novalpie` | 노벨피아 | ❌ | 韩文轻小说 |

---

## 下载任务

### 创建下载任务
> 需要认证。消耗 1 次下载配额。服务器异步处理，客户端需要轮询状态。

```
POST /novel/api/download-tasks
Authorization: Bearer <token>
Content-Type: application/json

{
  "site": "esjzone",
  "book_id": "1771755194"
}
```

| 参数 | 类型 | 必填 |
|------|------|------|
| `site` | string | ✅ |
| `book_id` | string | ✅ |

**响应 202:**
```json
{
  "task": {
    "id": "task_abc123",
    "site": "esjzone",
    "book_id": "1771755194",
    "status": "loading_chapters",
    "created_at": "2026-05-09T16:00:00Z"
  }
}
```

### 查询任务状态
> 需要认证。

```
GET /novel/api/download-tasks/:id
Authorization: Bearer <token>
```

**响应 200:**
```json
{
  "task": {
    "id": "task_abc123",
    "site": "esjzone",
    "book_id": "1771755194",
    "status": "completed",
    "title": "斗破苍穹之足诀",
    "exported_files": ["/path/to/book.txt"],
    "created_at": "2026-05-09T16:00:00Z"
  }
}
```

**任务状态：**
| Status | 说明 |
|--------|------|
| `loading_chapters` | 正在获取章节列表 |
| `downloading` | 正在下载 |
| `completed` | 完成，`exported_files` 中有文件路径 |
| `failed` | 失败，`error` 字段有错误信息 |

### 下载文件
> 需要认证。`path` 为 `exported_files` 中的完整路径。

```
GET /novel/api/download-file?path=/path/to/book.txt
Authorization: Bearer <token>
```

**响应 200:** 文件下载（Content-Disposition 头包含文件名）

---

## iOS 接入建议

### 基础接入流程

```swift
// 1. 启动时获取 guest token
let tokenResponse = GET /api/auth/guest-token
let token = tokenResponse.token
// 保存到本地，每次请求带上

// 2. 获取站点列表（缓存）
let meta = GET /api/meta
// 展示站点选择器

// 3. 搜索
POST /api/search, headers: { Authorization: "Bearer \(token)" }
→ 展示搜索结果列表

// 4. 点击书籍 → 获取详情
GET /api/books/detail?site=\(site)&book_id=\(bookId)
→ 展示书籍详情 + 目录

// 5. 点击章节 → 获取正文
GET /api/chapter-content?site=\(site)&book_id=\(bookId)&chapter_id=xxx&title=xxx&url=xxx
→ 展示阅读器
```

### 用户配额状态
> 每次调用 `/api/auth/me` 时返回当前用户配额信息。建议在 iOS 设置页展示：
> - 当前套餐（free/pro/unlimited）
> - 今日已用搜索/下载次数
> - 配额重置时间

### 配额不足处理
> 响应 HTTP 403，body 包含 `"error": "daily search quota exceeded"`。
>
> iOS 应提示用户："今日搜索配额已用完，请在 24:00 后重试，或升级到 Pro 套餐"

### 登录用户
> 如果用户想注册账号（保留配额记录）而非每次用 guest token：
> 1. 调用 `/api/auth/register` 注册
> 2. 保存返回的 token
> 3. 后续请求用这个 token

### 升级 Pro（管理员操作）
> 用户订阅由 iOS 自行处理，订阅成功后 iOS 调用：
> ```
> PUT /api/admin/users/:user_id/plan
> Header: X-Admin-Key: <admin-key>
> Body: { "plan": "pro" }
> ```
> `user_id` 从 `/api/auth/me` 的 `user.id` 获取。

---

## 错误码

| HTTP Status | 说明 |
|-------------|------|
| 200 | 成功 |
| 202 | 异步任务接受（下载任务） |
| 400 | 请求参数错误，body 有 `error` 说明原因 |
| 401 | 未认证或 token 无效 |
| 403 | 配额超限（当日搜索/下载次数用完） |
| 404 | 资源不存在（书籍/任务/文件） |
| 500 | 服务器内部错误 |

---

## 健康检查

```
GET /novel/healthz
```

用于 Docker 健康检查，始终返回 `{"status": "ok"}`
