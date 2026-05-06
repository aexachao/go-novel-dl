# iOS App 接入 go-novel-dl 后端集成指南

> 本文档面向 iOS 开发者，详述如何将 go-novel-dl 后端服务接入 iOS 应用。

## 目录

1. [服务部署](#1-服务部署)
2. [两种运行模式](#2-两种运行模式)
3. [认证流程](#3-认证流程)
4. [API 端点详解](#4-api-端点详解)
5. [iOS 端架构设计](#5-ios-端架构设计)
6. [配额系统](#6-配额系统)
7. [错误处理](#7-错误处理)
8. [自建服务器支持](#8-自建服务器支持)
9. [API 响应示例](#9-api-响应示例)
10. [最佳实践](#10-最佳实践)

---

## 1. 服务部署

### Docker 部署（推荐）

```bash
# 免认证模式（自建服务）
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  aexachao/go-novel-dl:latest web --port 8080

# 启用认证模式（你的共享服务）
docker run -d -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  aexachao/go-novel-dl:latest web --port 8080 \
    --auth \
    --auth-db /app/data/auth.db \
    --jwt-secret "your-secret-change-me"
```

### 基础信息

- 服务地址格式：`https://your-server.com/novel`
- API 前缀：`/novel/api/`
- 认证前缀：`/novel/api/auth/`
- 健康检查：`/novel/healthz`

---

## 2. 两种运行模式

### 免认证模式（自建服务）

```bash
./novel-dl web --port 8080
```

- 所有 API 无需认证
- 无配额限制
- 用户完全自主
- 适合：用户自己部署的服务

### 认证模式（共享服务）

```bash
./novel-dl web --port 8080 --auth-db ./data/auth.db --jwt-secret "secret"
```

- 所有核心 API 需要 JWT 或 API Key
- 有配额限制（Free / Pro）
- 适合：你的共享服务

### iOS 端如何区分

```swift
// App 启动时调用 meta 接口，判断 auth_enabled
let authEnabled = response["auth_enabled"] as? Bool ?? false

if authEnabled {
    // 需要登录
    showLoginFlow()
} else {
    // 直接访问，无需认证
}
```

---

## 3. 认证流程

### 3.1 注册

```
POST /novel/api/auth/register
Content-Type: application/json

{
    "email": "user@example.com",
    "password": "min6chars"
}
```

**响应（201 Created）**
```json
{
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "token_type": "access",
    "expires_at": 1735689600,
    "user": {
        "id": "1734567890-abc123",
        "email": "user@example.com",
        "plan": "free"
    }
}
```

### 3.2 登录

```
POST /novel/api/auth/login
Content-Type: application/json

{
    "email": "user@example.com",
    "password": "min6chars"
}
```

**响应（200 OK）** 同注册响应格式

### 3.3 Token 类型

| 类型 | 用途 | 有效期 |
|---|---|---|
| `access` (JWT) | App 日常 API 调用 | 7 天 |
| `api_key` | 第三方应用、服务器间调用 | 永久（可撤销） |

### 3.4 iOS Token 管理

```swift
import Security

class TokenManager {
    static let shared = TokenManager()
    
    private let accessTokenKey = "com.noveldln.accessToken"
    private let userIDKey = "com.noveldln.userID"
    
    // 登录后保存
    func saveAuth(token: String, userID: String) {
        save(key: accessTokenKey, value: token)
        save(key: userIDKey, value: userID)
    }
    
    // 获取当前 Token
    func getAccessToken() -> String? {
        return load(key: accessTokenKey)
    }
    
    // 清除（登出）
    func clearAuth() {
        delete(key: accessTokenKey)
        delete(key: userIDKey)
    }
    
    // --- Keychain 封装 ---
    
    private func save(key: String, value: String) {
        let data = value.data(using: .utf8)!
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecValueData as String: data,
            kSecAttrAccessible as String: kSecAttrAccessibleWhenUnlockedThisDeviceOnly
        ]
        SecItemDelete(query as CFDictionary)
        SecItemAdd(query as CFDictionary, nil)
    }
    
    private func load(key: String) -> String? {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key,
            kSecReturnData as String: true
        ]
        var result: AnyObject?
        let status = SecItemCopyMatching(query as CFDictionary, &result)
        guard status == errSecSuccess,
              let data = result as? Data,
              let string = String(data: data, encoding: .utf8) else {
            return nil
        }
        return string
    }
    
    private func delete(key: String) {
        let query: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrAccount as String: key
        ]
        SecItemDelete(query as CFDictionary)
    }
}
```

### 3.5 API Key 管理

用户可以在**设置页**生成 API Key，用于非交互场景：

```swift
// 获取当前用户的 API Key 列表
// GET /novel/api/auth/api-keys
// 需要认证

// 创建新的 API Key
// POST /novel/api/auth/api-keys
// 响应：{ "key": "nldl_xxx_xxx", "key_id": "xxx", "note": "..." }
// ⚠️ key 只在创建时返回一次，之后无法找回

// 删除 API Key
// DELETE /novel/api/auth/api-keys/:key_id
```

---

## 4. API 端点详解

### 4.1 元信息

#### 获取服务元信息（无需认证）

```
GET /novel/api/meta
```

```json
{
    "default_sources": [...],
    "all_sources": [...],
    "site_warnings": [...],
    "site_stats": [...],
    "general_config": {...},
    "auth_enabled": true
}
```

**用途**：App 启动时判断是否需要登录

---

#### 获取当前用户信息 + 配额

```
GET /novel/api/auth/me
Authorization: Bearer <token>
```

```json
{
    "user": {
        "id": "1734567890-abc123",
        "email": "user@example.com",
        "plan": "free"
    },
    "quota": {
        "search": {
            "used": 12,
            "limit": 50,
            "reset_at": "2025-01-02T00:00:00Z"
        },
        "download": {
            "used": 2,
            "limit": 5,
            "reset_at": "2025-01-02T00:00:00Z"
        },
        "plan": "free",
        "limits": {
            "daily_search": 50,
            "daily_download": 5,
            "max_workers": 1,
            "all_sites": false
        }
    }
}
```

---

### 4.2 搜索

#### 聚合搜索

```
POST /novel/api/search
Authorization: Bearer <token>
Content-Type: application/json

{
    "keyword": "三体",
    "sites": ["alicesw", "sfacg"],
    "page": 1,
    "page_size": 20,
    "scope": "default"
}
```

| 参数 | 类型 | 说明 |
|---|---|---|
| `keyword` | string | 搜索关键词，必填 |
| `sites` | string[] | 指定站点，不填则用默认 |
| `scope` | string | `"default"` 或 `"all"` |
| `page` | int | 页码，从 1 开始 |
| `page_size` | int | 每页数量，默认 50 |
| `site_limit` | int | 单站点最多返回 |

**响应**
```json
{
    "keyword": "三体",
    "sites": ["alicesw", "sfacg"],
    "results": [
        {
            "key": "title|author",
            "title": "三体",
            "author": "刘慈欣",
            "description": "...",
            "cover_url": "https://...",
            "latest_chapter": "第 30 章",
            "preferred_site": "alicesw",
            "source_count": 3,
            "score": 0.95,
            "primary": {
                "site": "alicesw",
                "book_id": "12345",
                "title": "三体",
                "author": "刘慈欣",
                "url": "https://alicesw.com/novel/12345"
            },
            "variants": [...]
        }
    ],
    "warnings": [],
    "page": 1,
    "page_size": 20,
    "total": 15,
    "total_exact": false,
    "has_prev": false,
    "has_next": true
}
```

**配额消耗**：每次成功的搜索请求消耗 1 次每日搜索配额

---

#### 搜索结果选择逻辑

- 同一本书在不同站点有不同 ID，后端按书名+作者归并
- `primary` 是推荐下载的版本
- `variants` 是同一本书的其他来源，可切换

---

### 4.3 书籍详情

```
GET /novel/api/books/detail?site=alicesw&book_id=12345
Authorization: Bearer <token>
```

```json
{
    "book": {
        "id": "12345",
        "site": "alicesw",
        "title": "三体",
        "author": "刘慈欣",
        "description": "...",
        "cover_url": "https://...",
        "source_url": "https://alicesw.com/novel/12345",
        "chapters": [
            {
                "id": "chapter_1",
                "title": "第一章 纳米研究中心",
                "index": 1,
                "word_count": 2345
            }
        ],
        "total_chapters": 30,
        "tags": ["科幻", "硬科幻"]
    }
}
```

**配额消耗**：消耗 1 次每日搜索配额

---

### 4.4 下载任务

#### 创建下载任务

```
POST /novel/api/download-tasks
Authorization: Bearer <token>
Content-Type: application/json

{
    "site": "alicesw",
    "book_id": "12345",
    "formats": ["epub"]
}
```

| 参数 | 类型 | 说明 |
|---|---|---|
| `site` | string | 站点 key |
| `book_id` | string | 书籍 ID |
| `formats` | string[] | 导出格式，`["txt"]`, `["epub"]`, `["html"]`, `["txt","epub"]` |

**响应（202 Accepted）**
```json
{
    "task": {
        "id": "task_abc123",
        "site": "alicesw",
        "book_id": "12345",
        "status": "loading_chapters"
    }
}
```

**配额消耗**：消耗 1 次每日下载配额

---

#### 查询任务进度

```
GET /novel/api/download-tasks/:task_id
Authorization: Bearer <token>
```

```json
{
    "task": {
        "id": "task_abc123",
        "site": "alicesw",
        "book_id": "12345",
        "status": "completed",
        "title": "三体",
        "progress": {
            "done": 30,
            "total": 30
        },
        "exported_files": [
            "/path/to/data/downloads/alicesw/12345.epub"
        ]
    }
}
```

**任务状态流转**

```
pending → loading_chapters → downloading → exporting → completed
                                              ↓
                                           failed
```

| 状态 | 说明 |
|---|---|
| `pending` | 刚创建 |
| `loading_chapters` | 正在获取目录 |
| `downloading` | 正在下载章节 |
| `exporting` | 正在生成文件 |
| `completed` | 完成，可下载 |
| `failed` | 失败 |

**轮询建议**：前端每 1-2 秒轮询一次，超时时间建议 60 秒

---

#### 下载导出文件

```
GET /novel/api/download-file?path=/path/to/data/downloads/alicesw/12345.epub
Authorization: Bearer <token>
```

**响应**：文件二进制流（Content-Type: application/octet-stream）

**注意**：`path` 是服务端返回的绝对路径，iOS 端原样传递即可。

---

## 5. iOS 端架构设计

### 5.1 分层结构

```
┌─────────────────────────────────────────┐
│              Views / SwiftUI             │
├─────────────────────────────────────────┤
│            ViewModels                    │
├─────────────────────────────────────────┤
│            Services / API Client         │
│  (NovelAPIClient + TokenManager)        │
├─────────────────────────────────────────┤
│         Network / URLSession             │
├─────────────────────────────────────────┤
│         Keychain / UserDefaults          │
└─────────────────────────────────────────┘
```

### 5.2 API Client 实现

```swift
import Foundation

enum APIError: Error {
    case unauthorized           // 401
    case quotaExceeded(limit: Int, resetAt: Date)
    case serverError(String)
    case networkError(Error)
    case decodingError(Error)
}

class NovelAPIClient {
    static let shared = NovelAPIClient()
    
    private let session: URLSession
    private let decoder: JSONDecoder
    private var baseURL: URL
    private var serverConfig: ServerConfig?
    
    init() {
        self.session = URLSession.shared
        self.decoder = JSONDecoder()
        self.decoder.dateDecodingStrategy = .iso8601
        // 默认地址，实际从设置获取
        self.baseURL = URL(string: "https://api.noveldln.com/novel")!
    }
    
    // MARK: - 配置
    
    struct ServerConfig {
        let baseURL: URL          // e.g. https://api.example.com/novel
        let requiresAuth: Bool
    }
    
    func configure(with config: ServerConfig) {
        self.baseURL = config.baseURL
        self.serverConfig = config
    }
    
    // MARK: - 请求构造
    
    private func request(
        _ method: String,
        path: String,
        body: Data? = nil,
        requiresAuth: Bool = true
    ) -> URLRequest {
        let url = baseURL.appendingPathComponent(path)
        var req = URLRequest(url: url)
        req.httpMethod = method
        req.setValue("application/json", forHTTPHeaderField: "Content-Type")
        
        if requiresAuth, let token = TokenManager.shared.getAccessToken() {
            req.setValue("Bearer \(token)", forHTTPHeaderField: "Authorization")
        }
        
        if let body = body {
            req.httpBody = body
        }
        return req
    }
    
    private func encode<T: Encodable>(_ value: T) -> Data? {
        try? JSONEncoder().encode(value)
    }
    
    // MARK: - 通用请求处理
    
    private func execute<T: Decodable>(
        _ request: URLRequest,
        responseType: T.Type
    ) async throws -> T {
        let (data, response) = try await session.data(for: request)
        
        guard let http = response as? HTTPURLResponse else {
            throw APIError.networkError(NSError(domain: "", code: -1))
        }
        
        // 配额超限
        if http.statusCode == 429, let t = try? decoder.decode(QuotaErrorResponse.self, from: data) {
            throw APIError.quotaExceeded(limit: t.limit, resetAt: ISO8601DateFormatter().date(from: t.reset_at) ?? Date())
        }
        
        // 未授权
        if http.statusCode == 401 {
            NotificationCenter.default.post(name: .didReceiveUnauthorized, object: nil)
            throw APIError.unauthorized
        }
        
        guard (200...299).contains(http.statusCode) else {
            let msg = String(data: data, encoding: .utf8) ?? "Unknown error"
            throw APIError.serverError(msg)
        }
        
        do {
            return try decoder.decode(T.self, from: data)
        } catch {
            throw APIError.decodingError(error)
        }
    }
    
    // MARK: - API 方法
    
    func fetchMeta() async throws -> MetaResponse {
        let req = request("GET", path: "api/meta", requiresAuth: false)
        return try await execute(req, responseType: MetaResponse.self)
    }
    
    func register(email: String, password: String) async throws -> AuthResponse {
        let body = encode(["email": email, "password": password])
        let req = request("POST", path: "api/auth/register", body: body, requiresAuth: false)
        return try await execute(req, responseType: AuthResponse.self)
    }
    
    func login(email: String, password: String) async throws -> AuthResponse {
        let body = encode(["email": email, "password": password])
        let req = request("POST", path: "api/auth/login", body: body, requiresAuth: false)
        return try await execute(req, responseType: AuthResponse.self)
    }
    
    func getMe() async throws -> MeResponse {
        let req = request("GET", path: "api/auth/me")
        return try await execute(req, responseType: MeResponse.self)
    }
    
    func search(keyword: String, sites: [String]? = nil, page: Int = 1) async throws -> SearchResponse {
        var payload: [String: Any] = ["keyword": keyword, "page": page]
        if let sites = sites { payload["sites"] = sites }
        let body = try JSONSerialization.data(withJSONObject: payload)
        let req = request("POST", path: "api/search", body: body)
        return try await execute(req, responseType: SearchResponse.self)
    }
    
    func getBookDetail(site: String, bookID: String) async throws -> BookDetailResponse {
        let path = "api/books/detail?site=\(site)&book_id=\(bookID)"
        let req = request("GET", path: path)
        return try await execute(req, responseType: BookDetailResponse.self)
    }
    
    func createDownloadTask(site: String, bookID: String, formats: [String]) async throws -> TaskResponse {
        let body = encode(["site": site, "book_id": bookID, "formats": formats] as [String: Any])
        let req = request("POST", path: "api/download-tasks", body: body)
        return try await execute(req, responseType: TaskResponse.self)
    }
    
    func getTaskStatus(taskID: String) async throws -> TaskStatusResponse {
        let req = request("GET", path: "api/download-tasks/\(taskID)")
        return try await execute(req, responseType: TaskStatusResponse.self)
    }
}

// MARK: - Notification Names

extension Notification.Name {
    static let didReceiveUnauthorized = Notification.Name("didReceiveUnauthorized")
}
```

### 5.3 数据模型

```swift
struct AuthResponse: Codable {
    let token: String
    let token_type: String
    let expires_at: Int
    let user: UserInfo
}

struct UserInfo: Codable {
    let id: String
    let email: String
    let plan: String
}

struct MetaResponse: Codable {
    let auth_enabled: Bool
    let default_sources: [SiteDescriptor]
    let all_sources: [SiteDescriptor]
}

struct SiteDescriptor: Codable, Identifiable {
    let key: String
    let display_name: String
    let capabilities: SiteCapabilities
}

struct SiteCapabilities: Codable {
    let search: Bool
    let download: Bool
    let login: Bool
}

struct SearchResponse: Codable {
    let results: [SearchResult]
    let page: Int
    let page_size: Int
    let total: Int
    let has_prev: Bool
    let has_next: Bool
    let warnings: [SearchWarning]
}

struct SearchResult: Codable, Identifiable {
    var id: String { key }
    let key: String
    let title: String
    let author: String
    let description: String?
    let cover_url: String?
    let latest_chapter: String?
    let preferred_site: String
    let primary: BookRef
    let variants: [BookRef]
    let source_count: Int
    let score: Double
}

struct BookRef: Codable, Identifiable {
    var id: String { "\(site)-\(book_id)" }
    let site: String
    let book_id: String
    let title: String
    let author: String
    let url: String?
    let cover_url: String?
}

struct BookDetail: Codable {
    let id: String
    let site: String
    let title: String
    let author: String
    let description: String?
    let cover_url: String?
    let source_url: String?
    let chapters: [Chapter]
    let total_chapters: Int
    let tags: [String]?
}

struct Chapter: Codable, Identifiable {
    var id: String { chapter_id }
    let chapter_id: String
    let title: String
    let index: Int
    let word_count: Int?
}

struct TaskResponse: Codable {
    let task: DownloadTask
}

struct DownloadTask: Codable, Identifiable {
    let id: String
    let site: String
    let book_id: String
    let status: String
    let title: String?
    let progress: TaskProgress?
    let exported_files: [String]?
}

struct TaskProgress: Codable {
    let done: Int
    let total: Int
}

struct TaskStatusResponse: Codable {
    let task: DownloadTask
}

struct QuotaErrorResponse: Codable {
    let error: String
    let limit: Int
    let reset_at: String
}

struct MeResponse: Codable {
    let user: UserInfo
    let quota: QuotaInfo
}

struct QuotaInfo: Codable {
    let search: QuotaCounter
    let download: QuotaCounter
    let plan: String
    let limits: QuotaLimits
}

struct QuotaCounter: Codable {
    let used: Int
    let limit: Int
    let reset_at: String
}

struct QuotaLimits: Codable {
    let daily_search: Int
    let daily_download: Int
    let max_workers: Int
    let all_sites: Bool
}
```

### 5.4 视图模型示例

```swift
@MainActor
class SearchViewModel: ObservableObject {
    @Published var results: [SearchResult] = []
    @Published var isLoading = false
    @Published var errorMessage: String?
    @Published var hasNextPage = false
    
    private var currentPage = 1
    private var keyword = ""
    
    func search(keyword: String) async {
        self.keyword = keyword
        self.currentPage = 1
        self.isLoading = true
        self.errorMessage = nil
        
        do {
            let response = try await NovelAPIClient.shared.search(keyword: keyword, page: 1)
            self.results = response.results
            self.hasNextPage = response.has_next
        } catch let error as APIError {
            self.errorMessage = self.message(for: error)
        } catch {
            self.errorMessage = error.localizedDescription
        }
        
        self.isLoading = false
    }
    
    func loadNextPage() async {
        guard hasNextPage, !isLoading else { return }
        currentPage += 1
        isLoading = true
        
        do {
            let response = try await NovelAPIClient.shared.search(keyword: keyword, page: currentPage)
            results.append(contentsOf: response.results)
            hasNextPage = response.has_next
        } catch {
            currentPage -= 1
        }
        
        isLoading = false
    }
    
    private func message(for error: APIError) -> String {
        switch error {
        case .unauthorized:
            return "登录已过期，请重新登录"
        case .quotaExceeded(let limit, _):
            return "今日搜索配额已用完（\(limit)次）"
        case .serverError(let msg):
            return msg
        case .networkError:
            return "网络连接失败"
        case .decodingError:
            return "数据解析失败"
        }
    }
}
```

### 5.5 下载任务管理

```swift
@MainActor
class DownloadManager: ObservableObject {
    @Published var activeTasks: [DownloadTask] = []
    @Published var completedFiles: [String] = []
    
    func startDownload(site: String, bookID: String, formats: [String] = ["epub"]) async throws {
        let response = try await NovelAPIClient.shared.createDownloadTask(
            site: site, bookID: bookID, formats: formats
        )
        activeTasks.append(response.task)
        
        // 开始轮询
        Task {
            await pollTask(taskID: response.task.id)
        }
    }
    
    private func pollTask(taskID: String) async {
        while true {
            try? await Task.sleep(nanoseconds: 1_500_000_000) // 1.5s
            
            do {
                let response = try await NovelAPIClient.shared.getTaskStatus(taskID: taskID)
                
                if let index = activeTasks.firstIndex(where: { $0.id == taskID }) {
                    activeTasks[index] = response.task
                }
                
                switch response.task.status {
                case "completed":
                    if let files = response.task.exported_files {
                        completedFiles.append(contentsOf: files)
                    }
                    activeTasks.removeAll { $0.id == taskID }
                    NotificationCenter.default.post(name: .downloadCompleted, object: taskID)
                    return
                    
                case "failed":
                    activeTasks.removeAll { $0.id == taskID }
                    NotificationCenter.default.post(name: .downloadFailed, object: taskID)
                    return
                    
                default:
                    break
                }
            } catch {
                // 继续重试
            }
        }
    }
}

extension Notification.Name {
    static let downloadCompleted = Notification.Name("downloadCompleted")
    static let downloadFailed = Notification.Name("downloadFailed")
}
```

---

## 6. 配额系统

### 6.1 配额规则

| 计划 | 每日搜索 | 每日下载 | 并发任务 | 站点范围 |
|---|---|---|---|---|
| Free | 50 | 5 | 1 | 主流量站点 |
| Pro | 500 | 50 | 3 | 全部站点 |

### 6.2 配额重置

- 每日 UTC 0 点重置
- 重置时间可通过响应中的 `reset_at` 字段获取

### 6.3 配额耗尽处理

```swift
// 在 API Client 中捕获 429
if http.statusCode == 429 {
    let t = try decoder.decode(QuotaErrorResponse.self, from: data)
    throw APIError.quotaExceeded(limit: t.limit, resetAt: parseDate(t.reset_at))
}

// 在 ViewModel 中展示
if case .quotaExceeded(let limit, let resetAt) = error {
    let formatter = RelativeDateTimeFormatter()
    formatter.unitsStyle = .full
    let relative = formatter.localizedString(for: resetAt, relativeTo: Date())
    alertMessage = "今日搜索配额（\(limit)次）已用完，\(relative) 重置"
}
```

### 6.4 配额展示

```swift
struct QuotaView: View {
    let quota: QuotaInfo
    
    var body: some View {
        VStack(alignment: .leading, spacing: 12) {
            Text("今日配额")
                .font(.headline)
            
            QuotaBar(label: "搜索", used: quota.search.used, limit: quota.search.limit)
            QuotaBar(label: "下载", used: quota.download.used, limit: quota.download.limit)
            
            HStack {
                Text("当前计划：\(quota.plan.uppercased())")
                    .foregroundColor(.secondary)
                Spacer()
                if quota.plan == "free" {
                    Text("升级 Pro")
                        .foregroundColor(.blue)
                }
            }
            .font(.caption)
        }
    }
}

struct QuotaBar: View {
    let label: String
    let used: Int
    let limit: Int
    
    var progress: Double { Double(used) / Double(limit) }
    var remaining: Int { limit - used }
    
    var body: some View {
        VStack(alignment: .leading, spacing: 4) {
            HStack {
                Text(label)
                    .font(.subheadline)
                Spacer()
                Text("\(remaining) / \(limit)")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
            ProgressView(value: progress)
                .tint(progress > 0.8 ? .red : .blue)
        }
    }
}
```

---

## 7. 错误处理

### 7.1 HTTP 状态码

| 状态码 | 含义 | iOS 处理 |
|---|---|---|
| 200 | 成功 | 解析响应 |
| 201 | 创建成功（注册） | 解析 token |
| 202 | 异步任务已接受 | 记录 task_id |
| 400 | 请求参数错误 | 展示错误信息 |
| 401 | 未登录 / Token 无效 | 跳转登录页 |
| 403 | 权限不足 | 提示升级 |
| 404 | 资源不存在 | 提示不存在 |
| 429 | 配额耗尽 | 展示剩余时间 |
| 500 | 服务器内部错误 | 展示"服务器异常" |

### 7.2 错误信息展示

```swift
@MainActor
func handleError(_ error: Error) {
    if let apiError = error as? APIError {
        switch apiError {
        case .unauthorized:
            showLoginRequiredAlert()
        case .quotaExceeded(let limit, let resetAt):
            showQuotaAlert(limit: limit, resetAt: resetAt)
        case .serverError(let msg):
            showAlert(title: "服务器错误", message: msg)
        case .networkError:
            showAlert(title: "网络错误", message: "请检查网络连接后重试")
        case .decodingError:
            showAlert(title: "数据错误", message: "请更新 App 后重试")
        }
    }
}
```

---

## 8. 自建服务器支持

### 8.1 用户引导流程

```
App 设置页
├── 🏠 我的服务器
│   ├── 地址输入框（https://...）
│   ├── [测试连接] → GET /novel/api/meta
│   └── [保存]
│
├── 📍 局域网自动发现（可选）
│   └── 在同一局域网广播 discovery 请求
```

### 8.2 连接测试

```swift
func testConnection(baseURL: URL) async -> Bool {
    var components = URLComponents(url: baseURL.appendingPathComponent("novel/api/meta"), resolvingAgainstBaseURL: true)!
    // 尝试 https 和 http
    for scheme in ["https", "http"] {
        components.scheme = scheme
        guard let url = components.url else { continue }
        var req = URLRequest(url: url)
        req.timeoutInterval = 5
        do {
            let (_, resp) = try await URLSession.shared.data(for: req)
            if let http = resp as? HTTPURLResponse, http.statusCode == 200 {
                return true
            }
        } catch { continue }
    }
    return false
}
```

### 8.3 服务器地址存储

```swift
class ServerConfigManager {
    static let shared = ServerConfigManager()
    
    private let serverURLKey = "com.noveldln.serverURL"
    
    // 可选：多个自建服务器历史记录
    private let serverHistoryKey = "com.noveldln.serverHistory"
    
    func getCurrentServer() -> URL? {
        guard let urlString = UserDefaults.standard.string(forKey: serverURLKey) else {
            return nil
        }
        return URL(string: urlString)
    }
    
    func setCurrentServer(_ url: URL) {
        UserDefaults.standard.set(url.absoluteString, forKey: serverURLKey)
    }
    
    func getServerHistory() -> [URL] {
        guard let data = UserDefaults.standard.data(forKey: serverHistoryKey),
              let urls = try? JSONDecoder().decode([String].self, from: data) else {
            return []
        }
        return urls.compactMap { URL(string: $0) }
    }
    
    func addToHistory(_ url: URL) {
        var history = getServerHistory().filter { $0 != url }
        history.insert(url, at: 0)
        if history.count > 5 { history = Array(history.prefix(5)) }
        let strings = history.map { $0.absoluteString }
        UserDefaults.standard.set(try? JSONEncoder().encode(strings), forKey: serverHistoryKey)
    }
}
```

---

## 9. API 响应示例

### 完整搜索响应

```json
{
    "keyword": "斗罗大陆",
    "sites": ["sfacg", "linovelib"],
    "results": [
        {
            "key": "斗罗大陆|唐家三少",
            "title": "斗罗大陆",
            "author": "唐家三少",
            "description": "唐门外门弟子唐三，因偷学内门绝学而被逼迫跳崖...\n魂师世界，强者为尊！",
            "cover_url": "https://cdn.sfacg.com/cover/12345.jpg",
            "latest_chapter": "第 32 部 大结局",
            "preferred_site": "sfacg",
            "primary": {
                "site": "sfacg",
                "book_id": "456123",
                "title": "斗罗大陆",
                "author": "唐家三少",
                "url": "https://www.sfacg.com/Novel/456123/",
                "cover_url": "https://cdn.sfacg.com/cover/12345.jpg"
            },
            "variants": [
                {
                    "site": "linovelib",
                    "book_id": "789012",
                    "title": "斗罗大陆",
                    "author": "唐家三少",
                    "url": "https://www.linovelib.com/novel/789012.html",
                    "cover_url": null
                }
            ],
            "source_count": 2,
            "score": 0.98
        }
    ],
    "warnings": [],
    "page": 1,
    "page_size": 20,
    "total": 1,
    "total_exact": true,
    "has_prev": false,
    "has_next": false
}
```

### 书籍详情响应

```json
{
    "book": {
        "id": "456123",
        "site": "sfacg",
        "title": "斗罗大陆",
        "author": "唐家三少",
        "description": "唐门外门弟子唐三，因偷学内门绝学而被逼迫跳崖...",
        "cover_url": "https://cdn.sfacg.com/cover/12345.jpg",
        "source_url": "https://www.sfacg.com/Novel/456123/",
        "chapters": [
            {
                "chapter_id": "1000001",
                "title": "第一章 斗罗大陆",
                "index": 1,
                "word_count": 3500
            },
            {
                "chapter_id": "1000002",
                "title": "第二章 父亲",
                "index": 2,
                "word_count": 2800
            }
        ],
        "total_chapters": 280,
        "tags": ["玄幻", "异世大陆", "斗罗"]
    }
}
```

### 任务状态响应

```json
{
    "task": {
        "id": "task_1735689600_abc123",
        "site": "sfacg",
        "book_id": "456123",
        "status": "downloading",
        "title": "斗罗大陆",
        "progress": {
            "done": 156,
            "total": 280
        },
        "exported_files": null
    }
}
```

---

## 10. 最佳实践

### 10.1 网络安全

```swift
// Info.plist 中添加 App Transport Security 例外
// 如果用户使用自签证书的自建服务
<key>NSAppTransportSecurity</key>
<dict>
    <key>NSAllowsArbitraryLoads</key>
    <false/>
    <key>NSAllowsLocalNetworking</key>
    <true/>
    <!-- 如果需要 HTTP 自建服务 -->
    <key>exception_domains</key>
    <array>
        <dict>
            <key>includesubdomains</key>
            <true/>
            <key>reason</key>
            <string>自建小说下载服务</string>
        </dict>
    </array>
</dict>
```

### 10.2 Token 刷新策略

```swift
class AuthManager {
    // Token 过期前自动刷新
    func ensureValidToken() async throws {
        if let token = TokenManager.shared.getAccessToken(),
           let exp = TokenManager.shared.getTokenExpiry(),
           Date() > exp.addingTimeInterval(-3600) { // 提前1小时
            // Token 快过期，尝试刷新或重新登录
            NotificationCenter.default.post(name: .didReceiveUnauthorized, object: nil)
        }
    }
}
```

### 10.3 下载文件处理

```swift
func downloadFile(path: String) async throws -> URL {
    let encodedPath = path.addingPercentEncoding(withAllowedCharacters: .urlQueryAllowed)!
    let urlString = "\(baseURL)/api/download-file?path=\(encodedPath)"
    var req = URLRequest(url: URL(string: urlString)!)
    req.setValue("Bearer \(TokenManager.shared.getAccessToken() ?? "")", 
                 forHTTPHeaderField: "Authorization")
    
    let (data, _) = try await session.data(for: req)
    
    let tempDir = FileManager.default.temporaryDirectory
    let fileName = URL(fileURLWithPath: path).lastPathComponent
    let fileURL = tempDir.appendingPathComponent(fileName)
    
    try data.write(to: fileURL)
    return fileURL
}
```

### 10.4 防重复下载

```swift
class DownloadDeduplication {
    static func isDuplicate(bookID: String, site: String) -> Bool {
        let key = "\(site)_\(bookID)"
        let active = UserDefaults.standard.stringArray(forKey: "activeDownloads") ?? []
        return active.contains(key)
    }
    
    static func markStarted(bookID: String, site: String) {
        var active = UserDefaults.standard.stringArray(forKey: "activeDownloads") ?? []
        active.append("\(site)_\(bookID)")
        UserDefaults.standard.set(active, forKey: "activeDownloads")
    }
    
    static func markFinished(bookID: String, site: String) {
        var active = UserDefaults.standard.stringArray(forKey: "activeDownloads") ?? []
        active.removeAll { $0 == "\(site)_\(bookID)" }
        UserDefaults.standard.set(active, forKey: "activeDownloads")
    }
}
```

---

## 附录：快速对照表

| 功能 | API | 认证 | 配额 |
|---|---|---|---|
| 判断是否需登录 | `GET /api/meta` | ❌ | ❌ |
| 注册 | `POST /api/auth/register` | ❌ | ❌ |
| 登录 | `POST /api/auth/login` | ❌ | ❌ |
| 我的信息+配额 | `GET /api/auth/me` | ✅ | ❌ |
| 搜索 | `POST /api/search` | ✅ | ✅ 搜索 |
| 书籍详情 | `GET /api/books/detail` | ✅ | ✅ 搜索 |
| 创建下载 | `POST /api/download-tasks` | ✅ | ✅ 下载 |
| 任务状态 | `GET /api/download-tasks/:id` | ✅ | ❌ |
| 下载文件 | `GET /api/download-file` | ✅ | ❌ |
| 健康检查 | `GET /healthz` | ❌ | ❌ |
