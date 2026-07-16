# paopao-api

自用邮箱账号池 + 收信代理（Go / Gin / SQLite）。

## 功能

1. **批量导入**账号：`邮箱----密码`，一行一个  
2. **按平台随机取号**：排除已在该平台标记过的账号  
3. **手动标记 / 取消标记**：用完后标记，注册失败可释放  
4. **代理收信**：调用上游 `GetLastEmails`，并尝试从邮件中提取验证码  

## 快速开始

### 依赖

- Go 1.22+

### 安装与启动

```bash
go mod tidy
go run ./cmd/server
# 或
go build -o bin/paopao-api ./cmd/server
./bin/paopao-api
```

### 多平台构建（CI / 本地）

GitHub Actions 在 push / tag `v*` 时自动交叉编译：

| 平台 | 架构 |
|------|------|
| Windows | amd64 (x86_64)、arm64、386 (x86) |
| Linux | amd64、arm64、386 |

- Actions 产物：仓库 **Actions** 页下载 artifact  
- 打 tag 发布：`git tag v1.0.0 && git push origin v1.0.0` → 自动创建 **Release** 并附带全部二进制 + sha256  
- 本地交叉编译：`make dist`（输出到 `dist/`）

默认监听 `:8080`，数据库 `./data/paopao.db`（自动创建）。

浏览器打开 **http://127.0.0.1:8080/** 即可使用本地 Web 管理页（导入 / 取号 / 标记 / 查信 / 账号列表）。

**完整 HTTP API 文档（脚本/二次调用）：** 见 [docs/API.md](docs/API.md)。

### 环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADDR` | `:8080` | 监听地址 |
| `API_KEY` | 空 | 非空则要求 `X-API-Key` 或 `Authorization: Bearer` |
| `DB_PATH` | `./data/paopao.db` | SQLite 路径 |
| `UPSTREAM_BASE` | `https://query.paopaodw.com` | 上游收信服务 |
| `UPSTREAM_TIMEOUT_SEC` | `30` | 上游超时（秒） |

示例见 `configs/config.example.env`。

---

## API

统一响应：

```json
{ "code": 0, "message": "ok", "data": { } }
```

`code != 0` 表示失败；鉴权开启时未带 Key 返回 `401`。

### 健康检查

```bash
curl http://127.0.0.1:8080/health
```

### 1. 批量导入

```bash
# 纯文本，一行一个
curl -s -X POST http://127.0.0.1:8080/api/accounts/import \
  -H "Content-Type: text/plain" \
  --data-binary @accounts.txt

# 或管道
cat <<EOF | curl -s -X POST http://127.0.0.1:8080/api/accounts/import \
  -H "Content-Type: text/plain" --data-binary @-
user1@outlook.com----pass1
user2@outlook.com----pass2
EOF

# 已存在邮箱时覆盖密码：?overwrite=1
curl -s -X POST "http://127.0.0.1:8080/api/accounts/import?overwrite=1" \
  -H "Content-Type: text/plain" --data-binary @accounts.txt
```

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": { "total": 2, "inserted": 2, "skipped": 0, "invalid": 0, "updated": 0 }
}
```

### 2. 按平台取号（不自动标记）

```bash
curl -s -X POST http://127.0.0.1:8080/api/accounts/pick \
  -H "Content-Type: application/json" \
  -d '{"platform":"xai"}'
```

```json
{
  "code": 0,
  "data": {
    "id": 1,
    "email": "user1@outlook.com",
    "password": "pass1",
    "credential": "user1@outlook.com----pass1"
  }
}
```

无可用号时 `code=404`。

### 3. 标记已用于某平台

```bash
# 按 id
curl -s -X POST http://127.0.0.1:8080/api/accounts/1/mark \
  -H "Content-Type: application/json" \
  -d '{"platform":"xai"}'

# 按 email
curl -s -X POST http://127.0.0.1:8080/api/accounts/mark \
  -H "Content-Type: application/json" \
  -d '{"email":"user1@outlook.com","platform":"xai"}'
```

### 4. 取消标记

```bash
curl -s -X POST http://127.0.0.1:8080/api/accounts/1/unmark \
  -H "Content-Type: application/json" \
  -d '{"platform":"xai"}'
```

### 5. 查信（代理上游）

上游格式：`GetLastEmails?email=邮箱----密码&clientId=&refreshToken=&num=2&boxType=3`

```bash
# 用库内账号 id（自动拼密码）
curl -s "http://127.0.0.1:8080/api/emails?account_id=1&num=2&boxType=3"

# 用邮箱（库中有则自动补密码）
curl -s "http://127.0.0.1:8080/api/emails?email=user1@outlook.com&num=2"

# 直接传完整凭证
curl -s "http://127.0.0.1:8080/api/emails?email=user1@outlook.com----pass1&num=2"
```

`data` 含 `inbox`、`junk`，以及尽量提取的 `codes`（验证码列表）。

### 6. 列表 / 详情 / 统计

```bash
# 列表；unused=1 且 platform=xai → 尚未用于 xai 的号
curl -s "http://127.0.0.1:8080/api/accounts?page=1&page_size=20"
curl -s "http://127.0.0.1:8080/api/accounts?platform=xai&unused=1&status=active"

curl -s http://127.0.0.1:8080/api/accounts/1
curl -s http://127.0.0.1:8080/api/stats
```

### 7. 禁用 / 删除

```bash
curl -s -X PATCH http://127.0.0.1:8080/api/accounts/1 \
  -H "Content-Type: application/json" \
  -d '{"status":"disabled","note":"密码错误"}'

curl -s -X DELETE http://127.0.0.1:8080/api/accounts/1
```

### 鉴权示例

```bash
export API_KEY=your-secret
# 启动时带上 API_KEY=your-secret

curl -s http://127.0.0.1:8080/api/stats -H "X-API-Key: your-secret"
```

---

## 典型流程

```text
导入账号池
   → pick(platform=xai) 得到账号
   → 用该邮箱注册 xAI
   → GET /api/emails 取验证码
   → 成功后 mark platform=xai（下次不再抽到）
   → 失败则可不 mark，或 unmark 释放
```

## 目录结构

```text
cmd/server/          入口
internal/config/     配置
internal/db/         SQLite 初始化
internal/model/      模型
internal/store/      账号与标记
internal/service/    上游收信
internal/handler/    HTTP
internal/middleware/ API Key
web/                 本地管理页
docs/API.md          HTTP API 文档
configs/             示例配置
```

## 说明

- 取号**不会**自动标记，需手动调用 mark，避免并发重复时可在业务侧取号后立刻 mark。  
- 凭证仅支持 `email----password`；上游 `clientId` / `refreshToken` 固定传空。  
- **查信自动禁用：** 上游返回 `code=201`（未找到该邮箱的授权信息）时，若该邮箱在账号池中，会自动 `status=disabled`，不再参与随机取号。响应中带 `auto_disabled` / `account_id` / `email`。  
- 纯自用服务，请勿将数据库或端口暴露到公网；需要时设置 `API_KEY`。  
