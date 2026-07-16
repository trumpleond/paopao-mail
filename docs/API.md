# paopao-api HTTP API 文档

自用邮箱账号池 + 收信代理。默认地址：`http://127.0.0.1:8080`

| 项 | 说明 |
|----|------|
| 协议 | HTTP/JSON（导入接口也支持 `text/plain`） |
| 鉴权 | 可选。环境变量 `API_KEY` 非空时，除 `/health` 外需带 Key |
| Web UI | `GET /`（浏览器管理页，与 API 同域） |
| 时区 | 时间字段为 SQLite `datetime` 文本（UTC 写入） |

---

## 1. 统一响应格式

### 成功

```json
{
  "code": 0,
  "message": "ok",
  "data": { }
}
```

### 失败

```json
{
  "code": 400,
  "message": "错误说明",
  "data": { }
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `code` | int | `0` 成功；非 0 为业务错误码（常与 HTTP 状态相关） |
| `message` | string | 可读说明 |
| `data` | any | 成功时为载荷；失败时可能带附加信息（如查信 201） |

### 常见 `code` / HTTP

| code | HTTP | 含义 |
|------|------|------|
| 0 | 200 | 成功 |
| 400 | 400 | 参数错误 |
| 401 | 401 | 未授权（API Key 错误或缺失） |
| 404 | 404 | 资源不存在 / 无可用账号 |
| 201 | 422 | 上游「未找到授权」；若账号在池中会**自动禁用** |
| 500 | 500 | 内部错误 |
| 502 | 502 | 上游收信失败（非 201 的其它上游错误） |

> 健康检查 `/health` **不**走上述 envelope，见下文。

---

## 2. 鉴权

环境变量 `API_KEY`（**API 与 Web 管理页共用同一把 Key**）：

- **空（默认）**：不校验，本地自用即可；打开网页可直接使用  
- **非空**：`/api/*` 必须带 Key；网页会弹出 **API Key 登录** 窗口  

```http
X-API-Key: <your-key>
```

或

```http
Authorization: Bearer <your-key>
```

### Web 前端行为

| 场景 | 行为 |
|------|------|
| 服务端未设 `API_KEY` | 无登录窗，顶部显示「鉴权 · 关闭」 |
| 服务端设了 `API_KEY` | 首次打开弹出登录；Key 存 `localStorage`（`paopao_api_key`） |
| 登录后 | 所有 `fetch` 自动带 `X-API-Key` |
| 401 / Key 失效 | 再次弹出登录 |
| 退出 | 清除本地 Key，重新要求登录 |

页面 HTML（`GET /`）本身仍可匿名访问；保护的是 API 数据。

示例：

```bash
curl -s http://127.0.0.1:8080/api/stats -H "X-API-Key: your-secret"
```

---

## 3. 环境变量 / `.env`

进程启动时会加载 **`.env`**（当前工作目录或可执行文件同目录），格式：

```env
KEY=value
# 注释
export API_KEY=secret
```

已存在的系统环境变量**优先**，不会被 `.env` 覆盖。模板见仓库根目录 `.env.example`。

| 变量 | 默认 | 说明 |
|------|------|------|
| `ADDR` | `:8080` | 监听地址 |
| `API_KEY` | 空 | 非空则启用鉴权 |
| `DB_PATH` | `./data/paopao.db` | SQLite 路径 |
| `UPSTREAM_BASE` | `https://query.paopaodw.com` | 上游收信服务根地址 |
| `UPSTREAM_TIMEOUT_SEC` | `30` | 上游超时（秒） |

---

## 4. 接口一览

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/health` | 健康检查（无需 Key） |
| GET | `/` | Web 管理页 |
| GET | `/mail.txt` | 可选：返回工作目录下 `mail.txt` |
| POST | `/api/accounts/import` | 批量导入 `邮箱----密码` |
| POST | `/api/accounts/pick` | 按平台随机取号（不自动标记） |
| POST | `/api/accounts/mark` | 按 email / id 标记平台 |
| POST | `/api/accounts/:id/mark` | 按 id 标记平台 |
| POST | `/api/accounts/:id/unmark` | 取消平台标记 |
| GET | `/api/accounts` | 账号列表（分页 + 筛选） |
| GET | `/api/accounts/:id` | 账号详情（含已标记平台） |
| PATCH | `/api/accounts/:id` | 更新 status / note / password |
| DELETE | `/api/accounts/:id` | 删除账号及标记 |
| GET | `/api/stats` | 池统计 |
| GET | `/api/emails` | 代理上游查信（可自动禁用） |

---

## 5. 接口详情

### 5.1 健康检查

`GET /health`

**响应（非 envelope）：**

```json
{
  "status": "ok",
  "time": "2026-07-17T12:00:00.000Z",
  "version": "v1.0.0",
  "commit": "5c5c0e7"
}
```

`version` / `commit` 由构建时 `-ldflags` 注入；本地 `go run` 多为 `dev` / `none`。

```bash
curl -s http://127.0.0.1:8080/health
```

---

### 5.2 批量导入

`POST /api/accounts/import`

#### 方式 A：纯文本（推荐）

- `Content-Type: text/plain`
- Body：每行 `邮箱----密码`；空行 / `#` 开头跳过  
- Query：`overwrite=1` 或 `true` 时，已存在邮箱**更新密码**；默认跳过  

```bash
curl -s -X POST "http://127.0.0.1:8080/api/accounts/import" \
  -H "Content-Type: text/plain; charset=utf-8" \
  --data-binary @mail.txt

# 覆盖密码
curl -s -X POST "http://127.0.0.1:8080/api/accounts/import?overwrite=1" \
  -H "Content-Type: text/plain" \
  --data-binary @mail.txt
```

#### 方式 B：JSON

```json
{
  "text": "a@outlook.com----pass1\nb@outlook.com----pass2\n",
  "overwrite": false
}
```

也可用字段 `lines` 代替 `text`。

**成功 `data`：**

```json
{
  "total": 50,
  "inserted": 48,
  "skipped": 2,
  "invalid": 0,
  "updated": 0
}
```

| 字段 | 说明 |
|------|------|
| `total` | 有效解析尝试行数（非空、非注释） |
| `inserted` | 新插入 |
| `skipped` | 已存在且未覆盖 |
| `invalid` | 格式非法 |
| `updated` | 覆盖更新密码数 |

---

### 5.3 按平台随机取号

`POST /api/accounts/pick`

**不会**自动标记。仅返回 `status=active` 且**未**在该平台标记过的账号。

**Body（JSON）或 Query：**

```json
{ "platform": "xai" }
```

也支持：`POST /api/accounts/pick?platform=xai`（body 可空时用 query）。

**成功 `data`：**

```json
{
  "id": 42,
  "email": "user@outlook.com",
  "password": "secret",
  "credential": "user@outlook.com----secret"
}
```

**无可用号：**

```json
{ "code": 404, "message": "no available account for platform" }
```

```bash
curl -s -X POST http://127.0.0.1:8080/api/accounts/pick \
  -H "Content-Type: application/json" \
  -d "{\"platform\":\"xai\"}"
```

**典型脚本流程：**

```
pick(platform) → 去平台注册 → GET /api/emails → 成功后 mark
```

---

### 5.4 标记已用于平台

#### 按 ID

`POST /api/accounts/:id/mark`

```json
{ "platform": "xai" }
```

Query 也可：`?platform=xai`

**成功 `data`：**

```json
{
  "account_id": 42,
  "platform": "xai",
  "marked": true
}
```

幂等：已标记再调仍成功。

#### 按 Email 或 ID（body）

`POST /api/accounts/mark`

```json
{ "email": "user@outlook.com", "platform": "xai" }
```

或：

```json
{ "id": 42, "platform": "xai" }
```

```bash
curl -s -X POST http://127.0.0.1:8080/api/accounts/42/mark \
  -H "Content-Type: application/json" \
  -d "{\"platform\":\"xai\"}"

curl -s -X POST http://127.0.0.1:8080/api/accounts/mark \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"user@outlook.com\",\"platform\":\"xai\"}"
```

---

### 5.5 取消平台标记

`POST /api/accounts/:id/unmark`

```json
{ "platform": "xai" }
```

注册失败时可释放，使该号再次可被该平台 pick。

```bash
curl -s -X POST http://127.0.0.1:8080/api/accounts/42/unmark \
  -H "Content-Type: application/json" \
  -d "{\"platform\":\"xai\"}"
```

---

### 5.6 账号列表

`GET /api/accounts`

| Query | 默认 | 说明 |
|-------|------|------|
| `page` | 1 | 页码（从 1 起） |
| `page_size` | 50 | 每页条数，最大 500 |
| `status` | 空 | `active` / `disabled`；空=全部 |
| `platform` | 空 | 配合 `unused` 按平台标记筛选 |
| `unused` | 0 | `1`/`true`：仅**未**标记该 platform；否则仅**已**标记 |

**成功 `data`：**

```json
{
  "items": [
    {
      "id": 1,
      "email": "a@outlook.com",
      "password": "p1",
      "status": "active",
      "note": "",
      "created_at": "2026-07-17 00:00:00",
      "updated_at": "2026-07-17 00:00:00"
    }
  ],
  "total": 50,
  "page": 1,
  "page_size": 50
}
```

**示例：找还能用于 xai 的 active 号**

```bash
curl -s "http://127.0.0.1:8080/api/accounts?status=active&platform=xai&unused=1&page=1&page_size=20"
```

---

### 5.7 账号详情

`GET /api/accounts/:id`

**成功 `data`：**

```json
{
  "id": 42,
  "email": "user@outlook.com",
  "password": "secret",
  "status": "active",
  "note": "",
  "created_at": "...",
  "updated_at": "...",
  "platforms": ["openai", "xai"]
}
```

`platforms`：已标记的平台列表（无则 `[]`）。

---

### 5.8 更新账号

`PATCH /api/accounts/:id`

所有字段可选：

```json
{
  "status": "disabled",
  "note": "密码错误",
  "password": "newpass"
}
```

| 字段 | 说明 |
|------|------|
| `status` | 仅允许 `active` / `disabled` |
| `note` | 备注 |
| `password` | 非空则更新密码 |

**禁用 vs 标记：**

- `status=disabled`：任意平台都不再被 **pick**  
- `mark`：只排除该平台；其它平台仍可 pick  

```bash
curl -s -X PATCH http://127.0.0.1:8080/api/accounts/42 \
  -H "Content-Type: application/json" \
  -d "{\"status\":\"disabled\",\"note\":\"坏号\"}"
```

---

### 5.9 删除账号

`DELETE /api/accounts/:id`

同时删除关联 `platform_marks`。不可恢复。

```bash
curl -s -X DELETE http://127.0.0.1:8080/api/accounts/42
```

**成功 `data`：**

```json
{ "deleted": true, "id": 42 }
```

---

### 5.10 统计

`GET /api/stats`

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "total": 50,
    "active": 45,
    "disabled": 5,
    "platform_marks": {
      "xai": 12,
      "openai": 8
    }
  }
}
```

```bash
curl -s http://127.0.0.1:8080/api/stats
```

---

### 5.11 查信（代理上游）

`GET /api/emails`

内部请求：

```
GET {UPSTREAM_BASE}/api/GetLastEmails
  ?email={email}----{password}
  &clientId=
  &refreshToken=
  &num={num}
  &boxType={boxType}
```

#### Query 参数

| 参数 | 必填 | 默认 | 说明 |
|------|------|------|------|
| `account_id` | 三选一 | — | 池内 id，自动拼密码 |
| `email` | 三选一 | — | 纯邮箱（库中有则补密码）或完整 `邮箱----密码` |
| `password` | 可选 | — | 与纯邮箱联用 |
| `num` | 否 | 5 | 拉取最近条数 |
| `boxType` / `box_type` | 否 | 3 | 与上游一致 |

解析优先级：`account_id` > 完整 credential 字符串 > `email+password` > 库内按 email 查。

> 邮件本身**不分页**（上游只支持最近 N 封）。**账号列表**才分页，见 `GET /api/accounts`。

#### 成功 `data`

```json
{
  "inbox": [
    {
      "Date": "2026-07-17 00:01:37",
      "From": "OpenAI <noreply@tm.openai.com>",
      "To": "user@outlook.com <user@outlook.com>",
      "Subject": "你的 OpenAI 临时验证码",
      "Body": "<html>...</html>"
    }
  ],
  "junk": [],
  "codes": ["697871"],
  "account_id": 42,
  "email": "user@outlook.com"
}
```

| 字段 | 说明 |
|------|------|
| `inbox` / `junk` | 邮件列表；元素字段 `Date/From/To/Subject/Body` |
| `codes` | 从主题/正文尽量提取的验证码（可能不全） |
| `account_id` / `email` | 若能关联到池内账号则返回 |
| `raw` | 上游 data 结构异常时保留原始 JSON |

```bash
curl -s "http://127.0.0.1:8080/api/emails?account_id=42&num=5&boxType=3"
```

#### 上游 201：未找到授权 → 自动禁用

上游：

```json
{ "code": 201, "message": "未找到该邮箱的授权信息", "data": null }
```

本服务：

1. 若邮箱在账号池中 → `status=disabled`，`note` 记 `auto-disabled: upstream 201 ...`  
2. HTTP **422**，业务 `code=201`  

```json
{
  "code": 201,
  "message": "upstream code 201: 未找到该邮箱的授权信息（已自动禁用该账号）",
  "data": {
    "upstream_code": 201,
    "upstream_message": "未找到该邮箱的授权信息",
    "account_id": 42,
    "email": "user@outlook.com",
    "auto_disabled": true,
    "auto_disable_reason": "auto-disabled: upstream 201 未找到该邮箱的授权信息"
  }
}
```

若账号不在池中：仍返回 201 语义错误，但 `auto_disabled=false`。

#### 示例

```bash
# 按 id
curl -s "http://127.0.0.1:8080/api/emails?account_id=42&num=5&boxType=3"

# 按邮箱（池内自动补密码）
curl -s "http://127.0.0.1:8080/api/emails?email=user@outlook.com&num=5"

# 完整凭证
curl -s "http://127.0.0.1:8080/api/emails?email=user@outlook.com----secret&num=5"
```

---

## 6. 领域概念速查

| 概念 | 说明 |
|------|------|
| 凭证格式 | `邮箱----密码`（四个短横线） |
| **标记 mark** | 账号在某 **platform** 已用过；该平台 pick 不再返回 |
| **禁用 disabled** | 账号整体退出 pick 池（所有平台） |
| **启用 active** | 默认可被 pick（还需未 mark 该平台） |
| platform | 自由字符串，如 `xai` / `openai` / `glados` / `cursor` |
| 取号 | **不**自动 mark，需业务侧成功后自行 mark |

---

## 7. 推荐调用流程（脚本）

```text
1. POST /api/accounts/import          # 导入池
2. POST /api/accounts/pick            # { "platform": "xai" }
3. 使用返回的 email/password 去平台注册
4. GET  /api/emails?account_id=...    # 取验证码（看 data.codes）
5. 注册成功 → POST /api/accounts/:id/mark { "platform": "xai" }
6. 注册失败 → 可不 mark；或 unmark 释放
7. 上游 201 无授权 → 自动 disabled，换号从 2 继续
```

### PowerShell 示例

```powershell
$Base = "http://127.0.0.1:8080"
# $Headers = @{ "X-API-Key" = "your-secret" }

# 导入
Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/import" `
  -ContentType "text/plain; charset=utf-8" -InFile "mail.txt"

# 取号
$pick = Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/pick" `
  -ContentType "application/json" -Body '{"platform":"xai"}'
$acc = $pick.data
Write-Host "got $($acc.email)"

# 查信
$mail = Invoke-RestMethod -Uri "$Base/api/emails?account_id=$($acc.id)&num=5"
$mail.data.codes

# 标记
Invoke-RestMethod -Method Post -Uri "$Base/api/accounts/$($acc.id)/mark" `
  -ContentType "application/json" -Body '{"platform":"xai"}'
```

### Python 示例

```python
import requests

BASE = "http://127.0.0.1:8080"
# headers = {"X-API-Key": "your-secret"}
headers = {}

# 取号
r = requests.post(f"{BASE}/api/accounts/pick", json={"platform": "xai"}, headers=headers)
r.raise_for_status()
body = r.json()
if body["code"] != 0:
    raise SystemExit(body["message"])
acc = body["data"]

# 查信
r = requests.get(
    f"{BASE}/api/emails",
    params={"account_id": acc["id"], "num": 5, "boxType": 3},
    headers=headers,
)
mail = r.json()
if mail["code"] == 201:
    # 已自动禁用（若在池内）
    print("auth missing, auto_disabled=", mail.get("data", {}).get("auto_disabled"))
elif mail["code"] == 0:
    print("codes:", mail["data"].get("codes"))
    # 成功后标记
    requests.post(
        f"{BASE}/api/accounts/{acc['id']}/mark",
        json={"platform": "xai"},
        headers=headers,
    )
```

---

## 8. 错误处理建议

| 场景 | 建议 |
|------|------|
| pick `code=404` | 该平台无可用号：导入更多 / unmark / 检查 disabled |
| emails `code=201` + `auto_disabled=true` | 换号重新 pick，勿再使用该 id |
| emails `code=502` | 上游超时/网络；可重试，**不会**因此自动禁用 |
| `401` | 检查 `API_KEY` 与请求头 |
| 导入 `invalid` 偏高 | 检查是否为 `邮箱----密码`（四个 `-`） |

---

## 9. 非 API 资源

| 路径 | 说明 |
|------|------|
| `GET /` | 内嵌/磁盘 Web 管理页 |
| `GET /mail.txt` | 若工作目录存在 `mail.txt` 则返回纯文本，供 UI「加载 mail.txt」 |

---

## 10. 变更记录（文档视角）

| 能力 | 行为 |
|------|------|
| 取号 | 不自动 mark |
| 查信 201 | 自动 `disabled` + 返回 `auto_disabled` |
| 凭证 | 仅 `email----password`；上游 clientId/refreshToken 固定空 |
| 密码存储 | SQLite 明文（仅限本机自用，勿公网暴露） |

---

*与代码同步维护：路由见 `cmd/server/main.go`，模型见 `internal/model`，收信见 `internal/service/email.go`。*
