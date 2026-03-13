# Ontime Detector / my version：Go 行情告警后端概要

本文件只描述 `Ontime Detector/my version` 目录下的 **Go 告警后端**，用于让 AI / 开发者在不翻源码的情况下快速理解架构与对接方式。

---

## 1. 定位与目标

- **作用**：从行情数据源（当前为 Yahoo Finance）周期性抓取价格，根据用户配置的告警规则进行判断，并通过 **企业微信 Webhook** 推送通知。
- **特点**：
  - 无 UI、纯后端服务；
  - 仅负责「行情 → 告警 → 推送」，不处理自然语言解析等逻辑；
  - 通过简单 HTTP API 供上层（如 Agent Team v3 的 back agent / OpenClaw）创建、删除、查询告警。
- **与 `ticker` 的关系**：
  - `Ontime Detector/ticker`：上游 TUI 工具，完整终端看盘应用，保持只读。
  - `Ontime Detector/my version`：独立 Go 模块，仅借鉴 ticker 的“行情 + symbol 思路”，不依赖其 UI 或内部结构。

---

## 2. 项目结构

根路径：`Ontime Detector/my version`

```text
my version/
├── go.mod                      # Go 模块：ontime-detector-alert
├── config.yaml                 # 默认配置（行情源 / 调度间隔 / DB 路径 / Webhook / 监听地址）
├── claude.md                   # 本文件
├── alerts/                     # 告警领域模型与 SQLite 持久化
│   ├── model.go                # Alert 结构 & Direction 枚举
│   ├── repository.go           # Repository 接口 + SQLite 实现
│   └── id.go                   # 随机十六进制 ID 生成
├── api/                        # HTTP API（REST）
│   └── server.go               # /health、/alerts、/alerts/{id}
├── cmd/
│   └── server/
│       ├── main.go             # 进程入口（加载配置、启动 scheduler + API）
│       └── config.go           # Config 结构 + LoadConfig
├── engine/
│   └── engine.go               # 告警判定核心（CheckAlert / EvaluateAlerts）
├── notifier/
│   └── wecom.go                # 企业微信 Webhook 通知实现
├── priceprovider/
│   ├── provider.go             # 行情提供接口
│   └── yahoo.go                # Yahoo Finance 实现
└── scheduler/
    └── scheduler.go            # 定时器：周期拉价格 + 触发告警
```

---

## 3. 配置与运行

### 3.1 `config.yaml`

```yaml
price_provider:
  base_url: "https://query1.finance.yahoo.com"

scheduler_interval_seconds: 30

wecom_webhook_url: ""          # 为空时仅在日志里提示，不实际发通知

database_path: "alerts.db"     # SQLite 文件

listen_address: ":8080"        # HTTP 监听地址
```

运行时加载顺序：

- `cmd/server/main.go` 中：
  - 调用 `LoadConfig("config.yaml")` 读取上述文件；
  - 若 `DatabasePath` 为空，退回 `"alerts.db"`；
  - `WeComWebhookURL` 会被环境变量 `WECOM_WEBHOOK_URL` 覆盖（如不想写入 config.yaml，可只配 env）。

### 3.2 启动服务

工作目录需为 `Ontime Detector/my version`，确保能读到 `config.yaml`：

```bash
cd "d:\Projects\agent-jk\Ontime Detector\my version"
go run ./cmd/server
```

健康检查：

```text
GET http://localhost:8080/health
→ {"status":"ok"}
```

### 3.3 数据库存储

- 驱动：`modernc.org/sqlite`（纯 Go）。
- 表结构（简化）：

```sql
CREATE TABLE IF NOT EXISTS alerts (
    id TEXT PRIMARY KEY,
    symbol TEXT NOT NULL,
    direction TEXT NOT NULL,        -- "above" / "below"
    threshold REAL NOT NULL,
    user_id TEXT NOT NULL,
    active INTEGER NOT NULL DEFAULT 1,
    triggered_at TIMESTAMP NULL,
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    last_notified_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

所有时间均使用 `time.Now().UTC()` 写入，读取时视为 UTC。

---

## 4. 核心模块说明

### 4.1 `alerts`：告警模型与仓储

- `Alert` 结构（`alerts/model.go`）：
  - `ID string`：十六进制随机 ID。
  - `Symbol string`：如 `AAPL`、`BTC-USD`。
  - `Direction Direction`：`"above"` / `"below"`。
  - `Threshold float64`：触发阈值。
  - `UserID string`：上层用户标识（可用 WeCom userid、OpenClaw user id 等）。
  - `Active bool`：是否启用。
  - `CooldownSeconds int`：冷却时间（秒），控制重复通知频率。
  - `TriggeredAt *time.Time`：最近一次触发时间（当前实现为“最近一次”，不是“首次”）。
  - `LastNotifiedAt *time.Time`：最近一次成功发送通知时间。

- `Repository` 接口：
  - `Create(alert *Alert) error`
  - `Delete(id string) error`（无记录时返回 `sql.ErrNoRows`）
  - `ListByUser(userID string) ([]Alert, error)`
  - `ListActive() ([]Alert, error)`
  - `UpdateNotificationState(id string, triggeredAt, lastNotifiedAt *time.Time) error`
  - `Close() error`

### 4.2 `engine`：告警判定

核心逻辑（`engine/engine.go`）：

- `CheckAlert(a Alert, price float64, now time.Time) bool`
  - `above`：`price >= Threshold` 时为真。
  - `below`：`price <= Threshold` 时为真。
  - 若设置了 `CooldownSeconds` 且 `LastNotifiedAt` 非空，`now - LastNotifiedAt < cooldown` 时不会再次触发。
- `EvaluateAlerts(alertsList []Alert, prices map[string]float64, now time.Time) []Alert`
  - 逐条匹配 symbol 对应价格，调用 `CheckAlert`，收集需要通知的告警列表。

### 4.3 `priceprovider`：行情获取

- 接口（`priceprovider/provider.go`）：
  - `GetPrices(symbols []string) (map[string]float64, error)`
- 当前实现：`YahooProvider`（`priceprovider/yahoo.go`）：
  - 调用 `GET {baseURL}/v7/finance/quote?symbols=SYM1,SYM2,...`；
  - 从响应中读取 `regularMarketPrice` 作为当前价格；
  - 忽略未返回价格的 symbol（不触发告警，也不 panic）。

### 4.4 `notifier`：企业微信 Webhook

- 接口：`Notifier.SendText(content string) error`
- `weComNotifier`：
  - 若 `webhookURL` 为空，仅打印日志并跳过；
  - 发送 `{"msgtype":"text","text":{"content": "..."} }` 到企业微信机器人 webhook URL。

### 4.5 `scheduler`：定时调度

运行流程（`scheduler/scheduler.go`）：

1. 每 `interval` 秒：
   - `ListActive()` 读取全部 active alerts。
   - 收集涉及到的所有 symbol，调用 `Provider.GetPrices(symbols)`。
   - 使用 `engine.EvaluateAlerts` 判定哪些告警应触发。
2. 对每个触发的告警：
   - 使用统一模板构造文本：
     - `Symbol: {symbol}`
     - `Condition: >/< {threshold}`
     - `Price: {current_price}`
     - `Time: {RFC3339 UTC}`
   - 调 `Notifier.SendText(content)`（企业微信 Webhook）；
   - 额外调用一个固定的 OpenClaw 回调（当前部署在同一 Render 服务域名下）：

     ```text
     POST https://ontime-detector-alert.onrender.com/agent/notify
     Content-Type: application/json

     {
       "user_id": "{alert.UserID}",
       "message": "⚠️ Alert Triggered\nSymbol: {symbol}\nPrice: {current_price}"
     }
     ```

     上层 OpenClaw backend 可在该接口中把 `message` 以「机器人说话」的形式转发到当前会话。
   - 再调用一次 Telegram Bot API，将同一段 `content` 发送到固定聊天：

     - 通过环境变量配置：
       - `TELEGRAM_BOT_TOKEN`：Bot 的 token（如 `8379...:xxxx`），**不要写入仓库**。
       - `TELEGRAM_CHAT_ID`：接收告警的 chat_id（如群聊 ID）。
     - 代码内部会构造：

       ```text
       POST https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage
       Content-Type: application/json

       {
         "chat_id": "${TELEGRAM_CHAT_ID}",
         "text": "Symbol: ...\nCondition: ...\nPrice: ...\nTime: ..."
       }
       ```

     若 env 未设置或请求失败，仅记录日志，不影响主流程。
   - 成功后调用 `UpdateNotificationState(id, &now, &now)`。

`Scheduler.Run()` 在 `main` 中以 goroutine 形式启动，与 HTTP API 并行运行，共享同一个 `Repository` 实例。

### 4.6 `api`：HTTP 接口（供 back agent / OpenClaw 调用）

路由定义（`api/server.go`）：

- `GET /health`
  - 返回：`{"status":"ok"}`。

- `POST /alerts`
  - 请求 JSON：
    ```json
    {
      "symbol": "BTC-USD",
      "direction": "above",          // "above" 或 "below"
      "threshold": 60000,
      "user_id": "wecom:U123",
      "cooldown_seconds": 300
    }
    ```
  - 校验：`symbol` / `direction` / `user_id` 非空，`direction` 必须为 `"above"` 或 `"below"`。
  - 创建成功：`201 Created` + 完整 `Alert` JSON。

- `GET /alerts?user_id=...`
  - 必需 query：`user_id`；
  - 返回该用户所有告警列表。

- `DELETE /alerts/{id}`
  - 删除指定告警；
  - 找不到时返回 404。

---

## 5. 与上层 Agent / back agent 的对接约定

典型调用链：

```text
User (WeCom)
 → OpenClaw / back agent 解析自然语言
   → 按约定拼装 HTTP 请求
     → POST /alerts / GET /alerts / DELETE /alerts/{id}
       → 本服务落库 / 查询 / 删除
       → scheduler 周期性触发通知 → WeCom Webhook
```

约定（建议）：

- `user_id`：
  - 建议格式：`wecom:<userid>`，便于在同一表中区分来源；
  - back agent 负责维护 WeCom 用户与内部 user id 的映射。
- `symbol`：
  - 使用 Yahoo Finance 支持的 symbol（例如 `AAPL`、`BTC-USD` 等）；
  - 未来若扩展多数据源，可以在 symbol 上约定后缀（类似 ticker 的 `.X`、`.CB` 思路）。

---

## 6. 常见 Symbol 参考（Yahoo）

当前实现仅使用 Yahoo Finance 的行情接口，不同标的通过 symbol 区分。部分常用示例如下（仅作约定，代码层不做限制）：

| 品种           | Yahoo Symbol | 说明             |
| -------------- | ------------ | ---------------- |
| 布伦特原油       | BZ=F         | Brent Crude Oil  |
| WTI 原油（可选） | CL=F         | West Texas Crude |
| 比特币（示例）    | BTC-USD      | Bitcoin vs USD   |

### 6.1 布伦特原油告警示例（最小方案）

使用现有 `YahooProvider` 即可监控布伦特原油，无需新增 provider。通过 `POST /alerts` 创建如下告警：

```http
POST /alerts
Content-Type: application/json

{
  "symbol": "BZ=F",
  "direction": "above",
  "threshold": 85,
  "user_id": "wecom:u123",
  "cooldown_seconds": 300
}
```

含义：当布伦特原油（`BZ=F`）价格高于 `85` 时触发告警，向 `wecom:u123` 发送企业微信通知；在冷却时间（此处 300 秒）内不会重复推送。

---

## 7. 开发与测试提示

- 编译 / 测试：

```bash
cd "d:\Projects\agent-jk\Ontime Detector\my version"
go test ./...
```

- 当前没有 `*_test.go`，`go test ./...` 主要用于验证 **编译是否通过、依赖是否完整**。后续可为：
  - `engine` 编写纯函数单测；
  - `priceprovider` 写 mock / integration 测试；
  - `api` 写 HTTP handler 级别的单测。

---

## 8. 后续可演进点（非必须）

- 将 `scheduler` 的 `Stop` 由 `close(chan)` 改为 context 取消，提升可控性。
- 调整 `triggered_at` 语义（区分“首次触发时间”和“最近一次触发时间”）。
- 为 `/alerts` 增加更细的参数校验（如 `threshold > 0`、`cooldown_seconds` 上限）。
- 增加认证（例如简单的 token）保护 API，仅内部 back agent 可访问。

