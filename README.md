# paopao-api

自用邮箱账号池 + 收信代理（Go / Gin / SQLite）。

- **Web 管理页**：导入 / 按平台取号 / 标记 / 查信 / 账号列表  
- **REST API**：便于脚本调用（见 [docs/API.md](docs/API.md)）  
- **Docker 镜像（已构建、Public）**：`ghcr.io/trumpleond/paopao-api`

---

## 功能

1. **批量导入**账号：`邮箱----密码`，一行一个  
2. **按平台随机取号**：排除已在该平台标记过的账号（**不**自动标记）  
3. **手动标记 / 取消标记**：用完后 mark；失败可 unmark  
4. **代理收信**：上游 `GetLastEmails`，并尽量提取 `codes` 验证码  
5. **查信 201 自动禁用**：上游「未找到授权」时，池内账号自动 `disabled`  
6. **可选 API Key**：服务端 `API_KEY` 与 Web 登录窗共用  

---

## Docker Compose 部署（推荐 · 使用已构建镜像）

无需安装 Go、无需本机 `docker build`。CI 已将镜像推送到 GHCR：

```text
ghcr.io/trumpleond/paopao-api
```

| 标签 | 说明 |
|------|------|
| `latest` | 默认分支最新构建 |
| `1.0.2` | 对应发布 `v1.0.2` |
| `1.0` | 主版本线 |
| `master` | master 分支构建 |
| `sha-…` | 指定 commit |

- 包页面：https://github.com/users/trumpleond/packages/container/package/paopao-api  
- 仓库：https://github.com/trumpleond/paopao-mail  
- 架构：`linux/amd64`、`linux/arm64`  

### 一键启动

```bash
# 1. 获取项目（或只下载 docker-compose.ghcr.yml + .env.example）
git clone https://github.com/trumpleond/paopao-mail.git
cd paopao-mail

# 2. 配置（可选）
cp .env.example .env
# 编辑 .env，例如：
#   API_KEY=your-secret
#   HOST_PORT=8080
#   IMAGE_TAG=1.0.2

# 3. 拉取预构建镜像并后台启动
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d

# 4. 打开管理页
# http://127.0.0.1:8080/
```

只下载 compose、不 clone 整仓也可以：

```bash
curl -fsSL -o docker-compose.ghcr.yml \
  https://raw.githubusercontent.com/trumpleond/paopao-mail/master/docker-compose.ghcr.yml
curl -fsSL -o .env.example \
  https://raw.githubusercontent.com/trumpleond/paopao-mail/master/.env.example
cp .env.example .env
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d
```

### 指定镜像版本

```bash
# Linux / macOS
IMAGE_TAG=1.0.2 docker compose -f docker-compose.ghcr.yml up -d

# Windows PowerShell
$env:IMAGE_TAG = "1.0.2"
docker compose -f docker-compose.ghcr.yml up -d
```

或在 `.env` 中写：

```env
IMAGE_TAG=1.0.2
```

### 日常运维

```bash
docker compose -f docker-compose.ghcr.yml ps
docker compose -f docker-compose.ghcr.yml logs -f
curl -s http://127.0.0.1:8080/health

# 更新到 latest（或 .env 里的 IMAGE_TAG）
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d

# 停止（保留数据库卷 paopao-data）
docker compose -f docker-compose.ghcr.yml down

# 停止并删除数据
docker compose -f docker-compose.ghcr.yml down -v
```

### 映射说明

| 项 | 值 |
|----|-----|
| 镜像 | `ghcr.io/trumpleond/paopao-api:${IMAGE_TAG:-latest}` |
| 端口 | `${HOST_PORT:-8080}:8080` |
| 数据 | 命名卷 `paopao-data` → 容器 `/data/paopao.db` |
| 配置 | `.env` → `API_KEY` / `UPSTREAM_BASE` / `UPSTREAM_TIMEOUT_SEC` 等 |
| Compose 文件 | [`docker-compose.ghcr.yml`](docker-compose.ghcr.yml) |

### 等价 docker run

```bash
docker pull ghcr.io/trumpleond/paopao-api:latest

docker run -d --name paopao-api --restart unless-stopped \
  -p 8080:8080 \
  -e ADDR=:8080 \
  -e DB_PATH=/data/paopao.db \
  -e API_KEY= \
  -e UPSTREAM_BASE=https://query.paopaodw.com \
  -e UPSTREAM_TIMEOUT_SEC=30 \
  -v paopao-data:/data \
  ghcr.io/trumpleond/paopao-api:latest
```

更细的 GHCR 说明（标签、私有包登录、发布流程）：[docs/GHCR.md](docs/GHCR.md)。

---

## 其它运行方式

### Docker Compose 本地构建

改代码后在本机编译镜像：

```bash
cp .env.example .env
docker compose up -d --build
# 使用 docker-compose.yml，镜像名 paopao-api:local
```

### 本地 Go

依赖 **Go 1.22+**：

```bash
cp .env.example .env
go mod tidy
go run ./cmd/server
# 或
go build -o bin/paopao-api ./cmd/server && ./bin/paopao-api
```

默认：`http://127.0.0.1:8080/`，库文件 `./data/paopao.db`。

### 多平台二进制（无 Docker）

| 平台 | 架构 |
|------|------|
| Windows | amd64、arm64、386 |
| Linux | amd64、arm64、386 |

- 打 tag `v*` → [Releases](https://github.com/trumpleond/paopao-mail/releases) 附件 + GHCR 镜像  
- Actions Artifacts；本地：`make dist`  

---

## 环境变量 / `.env`

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADDR` | `:8080` | 监听地址（容器内由 compose 固定为 `:8080`） |
| `API_KEY` | 空 | 非空则 API + Web 共用；网页弹出登录 |
| `DB_PATH` | `./data/paopao.db` | SQLite 路径；**Compose/GHCR 固定为** `/data/paopao.db` |
| `UPSTREAM_BASE` | `https://query.paopaodw.com` | 上游收信 |
| `UPSTREAM_TIMEOUT_SEC` | `30` | 上游超时（秒） |
| `HOST_PORT` | `8080` | **仅 Compose**：主机映射端口 |
| `IMAGE_TAG` | `latest` | **仅 docker-compose.ghcr.yml**：镜像标签 |

- 模板：`.env.example`（可提交）  
- 本地：`.env`（已 gitignore）  
- `go run` / 二进制：启动时自动加载 `.env`（不覆盖已有系统环境变量）  
- Compose：通过 `env_file` + `environment` 注入  

启用鉴权示例：

```env
API_KEY=your-secret
```

请求头：

```http
X-API-Key: your-secret
```

或 `Authorization: Bearer your-secret`。Web 登录后会把 Key 存浏览器 `localStorage` 并自动附带。

---

## API 速查

统一响应：

```json
{ "code": 0, "message": "ok", "data": { } }
```

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查（无需 Key） |
| POST | `/api/accounts/import` | 批量导入 `邮箱----密码` |
| POST | `/api/accounts/pick` | 按平台随机取号 |
| POST | `/api/accounts/:id/mark` | 标记平台 |
| POST | `/api/accounts/:id/unmark` | 取消标记 |
| GET | `/api/emails` | 查信（代理上游） |
| GET | `/api/accounts` | 账号列表（支持 `page` / `page_size`） |
| GET | `/api/stats` | 统计 |

完整参数与示例：[docs/API.md](docs/API.md)。

### 常用 curl

```bash
# 健康检查
curl -s http://127.0.0.1:8080/health

# 导入
curl -s -X POST http://127.0.0.1:8080/api/accounts/import \
  -H "Content-Type: text/plain" --data-binary @mail.txt

# 取号
curl -s -X POST http://127.0.0.1:8080/api/accounts/pick \
  -H "Content-Type: application/json" \
  -d '{"platform":"xai"}'

# 查信
curl -s "http://127.0.0.1:8080/api/emails?account_id=1&num=5"

# 标记
curl -s -X POST http://127.0.0.1:8080/api/accounts/1/mark \
  -H "Content-Type: application/json" \
  -d '{"platform":"xai"}'

# 带鉴权
curl -s http://127.0.0.1:8080/api/stats -H "X-API-Key: your-secret"
```

---

## 典型流程

```text
导入账号池
  → pick(platform=xai)
  → 用该邮箱去平台注册
  → GET /api/emails 取验证码
  → 成功 → mark platform=xai
  → 失败 → 不 mark 或 unmark
  → 上游 201 无授权 → 自动 disabled，换号
```

---

## 目录结构

```text
cmd/server/                 入口
internal/                   配置 / DB / store / service / handler
web/                        Web 管理页
docs/API.md                 HTTP API 文档
docs/GHCR.md                GHCR 与 Compose 部署细节
Dockerfile                  镜像多阶段构建
docker-compose.yml          本地 build 部署
docker-compose.ghcr.yml     拉取预构建镜像部署（推荐）
.env.example                环境变量模板
.github/workflows/          二进制 + Docker 发布 CI
```

---

## 说明

- 取号**不会**自动 mark，需业务侧成功后自行调用 mark。  
- 凭证格式：`邮箱----密码`（四个短横线）；上游 `clientId` / `refreshToken` 固定空。  
- 查信上游 `code=201`：池内账号自动 `disabled`，响应含 `auto_disabled`。  
- 密码明文存 SQLite，仅限本机/内网；公网务必设 `API_KEY` 并限制端口暴露。  
- 数据卷 `paopao-data` 与部署方式绑定，切换 compose 文件时注意是否共用同一卷。  
