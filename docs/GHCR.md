# Docker / GHCR 部署说明

## 镜像地址（已构建、Public）

```text
ghcr.io/trumpleond/paopao-api
```

| 常用标签 | 用途 |
|----------|------|
| `latest` | 默认分支最新 |
| `1.0.2` | 版本发布（git `v1.0.2`） |
| `1.0` | 主版本线 |
| `master` | master 分支构建 |
| `sha-3ccab46` | 某次 commit |

包页面：https://github.com/users/trumpleond/packages/container/package/paopao-api  

架构：`linux/amd64`、`linux/arm64`（多架构 manifest）。

CI：`.github/workflows/docker.yml`（push `master` / tag `v*` / 手动触发）。

---

## 推荐：Docker Compose + 预构建镜像

仓库文件：`docker-compose.ghcr.yml`（**不本地 build**，只 pull）。

### 1. 准备文件

```bash
git clone https://github.com/trumpleond/paopao-mail.git
cd paopao-mail
```

或只下载 compose：

```bash
curl -fsSL -o docker-compose.ghcr.yml \
  https://raw.githubusercontent.com/trumpleond/paopao-mail/master/docker-compose.ghcr.yml
curl -fsSL -o .env.example \
  https://raw.githubusercontent.com/trumpleond/paopao-mail/master/.env.example
```

### 2. 配置环境变量（可选）

```bash
cp .env.example .env
```

可改项示例：

```env
API_KEY=your-secret
HOST_PORT=8080
UPSTREAM_BASE=https://query.paopaodw.com
UPSTREAM_TIMEOUT_SEC=30
# 指定镜像标签（不设则 latest）
# IMAGE_TAG=1.0.2
```

> Compose 会把容器内 `DB_PATH` 固定为 `/data/paopao.db`，数据在 Docker 卷里，不必用宿主机路径。

### 3. 启动

```bash
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d
```

指定版本：

```bash
# Linux / macOS
IMAGE_TAG=1.0.2 docker compose -f docker-compose.ghcr.yml up -d

# Windows PowerShell
$env:IMAGE_TAG="1.0.2"
docker compose -f docker-compose.ghcr.yml up -d
```

### 4. 验证

```bash
curl -s http://127.0.0.1:8080/health
docker compose -f docker-compose.ghcr.yml logs -f
```

浏览器：http://127.0.0.1:8080/

### 5. 更新 / 停止

```bash
# 拉新 latest 并滚动重启
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d

# 停止（保留 paopao-data 卷）
docker compose -f docker-compose.ghcr.yml down

# 停止并删除数据库卷
docker compose -f docker-compose.ghcr.yml down -v
```

### compose 映射关系

| 项 | 值 |
|----|-----|
| 镜像 | `ghcr.io/trumpleond/paopao-api:${IMAGE_TAG:-latest}` |
| 端口 | `${HOST_PORT:-8080}:8080` |
| 数据卷 | `paopao-data` → `/data` |
| 健康检查 | `GET /health` |

---

## 仅 docker run（不用 compose）

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

---

## 本地构建（开发）

需要改代码并在本机编译镜像时：

```bash
cp .env.example .env
docker compose up -d --build
```

使用 `docker-compose.yml`（含 `build:`），镜像名 `paopao-api:local`。

---

## 私有包时的登录

当前包已是 **Public**，一般无需登录。若改回 Private：

1. GitHub → Settings → Developer settings → PAT，勾选 `read:packages`
2. 登录：

```bash
echo YOUR_TOKEN | docker login ghcr.io -u trumpleond --password-stdin
docker pull ghcr.io/trumpleond/paopao-api:latest
```

---

## 发布新版本镜像（维护者）

```bash
# 确保仓库 Settings → Actions → Workflow permissions = Read and write
git tag v1.0.3
git push origin v1.0.3
```

Actions **Docker** 成功后，包上会出现 `1.0.3` / `1.0` 等标签；**Build** 工作流会挂二进制到 Release。

---

## 故障排查

| 现象 | 处理 |
|------|------|
| `pull access denied` / 401 | 包若为 Private → `docker login ghcr.io`；或确认已 Public |
| 端口占用 | `.env` 设 `HOST_PORT=18080` |
| 数据丢失 | 不要用 `down -v` 除非确认要清空；备份卷 `paopao-data` |
| 架构不匹配 | 镜像支持 amd64/arm64，老 32 位机请用 Release 二进制 |
