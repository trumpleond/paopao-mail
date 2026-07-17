# 使用 GHCR 发布 / 拉取 Docker 镜像

镜像地址（仓库所有者小写）：

```text
ghcr.io/trumpleond/paopao-api
```

CI 工作流：`.github/workflows/docker.yml`  
触发：`master`/`main` 推送、tag `v*`、手动 Run workflow。

---

## 一、一次性设置（GitHub 网页）

### 1. 确保 Actions 有写 Packages 权限

仓库 → **Settings** → **Actions** → **General** → **Workflow permissions**

- 勾选 **Read and write permissions**
- 勾选 **Allow GitHub Actions to create and approve pull requests**（可选）
- Save

> 工作流里已声明 `permissions.packages: write`，一般用默认 `GITHUB_TOKEN` 即可推包。

### 2. 推送代码后让 CI 构建

```bash
git add .
git commit -m "ci: publish docker image to GHCR"
git push origin master
```

或打版本 tag：

```bash
git tag v1.0.2
git push origin v1.0.2
```

到 **Actions** 页看 **Docker** 工作流是否成功。

### 3. 把包设为 Public（推荐自用拉取省事）

首次推送成功后：

1. 打开 GitHub 头像 → **Your packages**  
   或：`https://github.com/users/trumpleond/packages`
2. 点开 **paopao-api**
3. **Package settings** → **Change visibility** → **Public**  
   （Private 也可以，拉取时要登录，见下文）

可选：在 Package 页 **Connect repository** 关联 `trumpleond/paopao-mail`，方便在仓库侧边看到包。

### 4. 常见标签

| 场景 | 标签示例 |
|------|----------|
| 默认分支最新 | `ghcr.io/trumpleond/paopao-api:latest` |
| 某次 commit | `ghcr.io/trumpleond/paopao-api:sha-64e2ec3` |
| 版本 tag | `ghcr.io/trumpleond/paopao-api:1.0.2`（semver，不带 v 前缀时以 metadata 为准） |
| 分支 | `ghcr.io/trumpleond/paopao-api:master` |

以 Actions 日志 / Package 页上的 Tags 为准。

---

## 二、本机拉取运行

### 公开包（已设 Public）

```bash
docker pull ghcr.io/trumpleond/paopao-api:latest

docker run --rm -p 8080:8080 \
  -e DB_PATH=/data/paopao.db \
  -e API_KEY= \
  -v paopao-data:/data \
  ghcr.io/trumpleond/paopao-api:latest
```

或用 compose：

```bash
cp .env.example .env
docker compose -f docker-compose.ghcr.yml pull
docker compose -f docker-compose.ghcr.yml up -d
```

### 私有包（需登录）

1. GitHub → Settings → **Developer settings** → **Personal access tokens**  
   - Fine-grained：对账号 Packages 勾选 **Read**（推送另需 write）  
   - 或 classic：勾选 `read:packages`（推送要 `write:packages`）
2. 登录：

```bash
# Windows PowerShell
echo YOUR_TOKEN | docker login ghcr.io -u trumpleond --password-stdin

docker pull ghcr.io/trumpleond/paopao-api:latest
```

---

## 三、和本地 build 的区别

| 方式 | 命令 | 镜像从哪来 |
|------|------|------------|
| 本地构建 | `docker compose up -d --build` | 本机编译 Dockerfile |
| GHCR 拉取 | `docker compose -f docker-compose.ghcr.yml up -d` | `ghcr.io/trumpleond/paopao-api` |

数据卷名都是 `paopao-data`，切换部署方式时注意是否共用同一卷。

---

## 四、故障排查

| 现象 | 处理 |
|------|------|
| Actions 推送 403 | 检查 Workflow permissions 是否 Read and write；确认 `packages: write` |
| `denied` / 拉取 401 | 包是 Private → 先 `docker login ghcr.io` |
| 找不到包 | 等第一次 Docker workflow 成功；名称必须全小写 |
| 架构不对 | 镜像打了 `linux/amd64,linux/arm64`，一般 x86 服务器与 Apple Silicon 都可 |

---

## 五、删除 / 清理

Package 页 → **Manage versions** 可删旧版本。  
不需要镜像时：Package settings → Delete this package。
