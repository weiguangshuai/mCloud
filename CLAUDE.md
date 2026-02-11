# CLAUDE.md

本文件为 Claude Code (claude.ai/code) 在此仓库中工作时提供指引。

## 项目概述

mCloud 是一个自托管的私人网盘系统，由 Go 后端 API 服务和 Vue 3 前端 SPA 组成。

## 开发命令

### 后端（在 `backend/` 目录下执行）

```bash
# 启动服务（需要先启动 MySQL 和 Redis）
go run .

# 编译
go build -o mcloud .

# 运行全部测试
go test ./...

# 运行单个包的测试
go test ./handlers/...
go test ./utils/...
```

### 前端（在 `frontend/` 目录下执行）

```bash
# 安装依赖
npm install

# 开发服务器（端口 5173，/api 代理到 localhost:8080）
npm run dev

# 生产构建
npm run build
```

## 架构

### 后端 (`backend/`)

Go + Gin 框架。入口为 `main.go`，负责加载配置、初始化 MySQL/Redis、执行 GORM 自动迁移并注册路由。

- `config/` — YAML 配置加载。所有配置项在 `config.yaml` 中（服务器、数据库、Redis、JWT、存储、缩略图、回收站、分页）。
- `database/` — 全局单例 `DB` (*gorm.DB) 和 `RedisClient`，启动时初始化。
- `models/` — GORM 模型：User、Folder、File、UploadTask、RecycleBinItem、ThumbnailTask。表结构自动迁移。
- `handlers/` — Gin 处理函数，按领域分文件（auth、file、folder、recycle_bin、user、health）。
- `middleware/` — `AuthMiddleware`（JWT Bearer token → 在 gin.Context 中设置 `user_id`）、`CORSMiddleware`。
- `services/` — 业务逻辑（目前为基于 `imaging` 库的缩略图生成）。
- `utils/` — JWT 工具、bcrypt 密码哈希、统一 JSON 响应工具（`Success`、`Error` 等）。

所有 API 路由在 `/api` 下。公开路由：`/api/auth/register`、`/api/auth/login`、`/api/health`，其余均需 JWT 认证。

关键模式：
- 中间件从 JWT 中提取用户 ID，handler 中通过 `c.GetUint("user_id")` 获取。
- 响应统一使用 `utils.Success(c, data)` / `utils.Error(c, httpCode, message)`，JSON 格式为 `{code, message, data}`。
- 文件上传支持简单上传和分片上传（init → 上传分片 → complete），分片状态同时记录在 MySQL（UploadTask）和 Redis 中。
- 文件存储在磁��� `config.storage.base_path/files/`，缩略图在 `.../thumbnails/`，临时分片在 `.../temp/`。

### 前端 (`frontend/`)

Vue 3 + Vite + Pinia + Element Plus。

- `src/api/` — 按领域封装的 Axios 请求（auth、file、folder、recycleBin）。
- `src/utils/request.js` — Axios 实例，自动注入 JWT，401 时自动跳转登录页。
- `src/store/index.js` — Pinia 状态管理：用户信息、当前文件夹 ID、面包屑导航。
- `src/router/index.js` — 三个路由：Login、Register、Home。通过 `meta.requiresAuth` 实现路由守卫。
- `src/views/` — 页面组件（Login、Register、Home）。
- `src/components/` — UI 组件：FolderTree、FileList、FileUpload、Breadcrumb、ImagePreview、RecycleBin。

开发环境下前端通过 Vite 代理将 `/api` 请求转发到后端 `localhost:8080`（配置在 `vite.config.js`）。

## 配置

后端配置文件为 `backend/config.yaml`。本地开发需调整的关键配置：
- `database.password` — MySQL 密码
- `storage.base_path` — 文件存储磁盘路径
- `jwt.secret` — 生产环境务必修改默认值
