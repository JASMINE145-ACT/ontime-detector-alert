# Render 部署说明

本服务为 Go 告警后端，无前端；通过 HTTP API 提供告警的创建、查询、删除，并由内置 scheduler 定时拉取行情并触发企业微信通知。

## 部署步骤

1. 在 [Render](https://render.com) 连接你的 Git 仓库。
2. 新建 **Web Service**，选择本仓库。
3. **Root Directory**：设为 `Ontime Detector/my version`（或你仓库中 `go.mod`、`config.yaml` 所在目录），否则构建会失败。
4. **Build Command**：`go build -o server ./cmd/server`
5. **Start Command**：`./server`
6. **Environment**：添加变量（见下）。

若仓库根目录即本目录，可省略 Root Directory；也可使用同目录下的 `render.yaml` 让 Render 自动识别上述配置。

## 环境变量

| 变量 | 说明 |
|------|------|
| `WECOM_WEBHOOK_URL` | 企业微信机器人 webhook 地址；不设则仅打日志不推送。 |
| `PORT` | 由 Render 自动注入，无需手动设置。 |

敏感信息不要写入 `config.yaml`，仅在 Render Dashboard → Environment 中配置。

## 健康检查

- 服务提供 `GET /health`，返回 `{"status":"ok"}`。
- 在 Render 中可配置 Health Check Path 为 `/health`，由 Render 定期请求以判断服务是否存活。

## 保活（Free Tier 防休眠）

Render Free Tier 在约 **15 分钟**无 HTTP 请求后会休眠，scheduler 会停止，告警不再执行。

建议使用外部定时请求保持服务在线：

- [UptimeRobot](https://uptimerobot.com) 或 [cron-job.org](https://cron-job.org) 等
- 每 **5–15 分钟** 请求：`GET https://<your-service>.onrender.com/health`
- 这样容器不会因无流量而休眠，scheduler 可持续运行。

## SQLite 与数据持久化

当前使用 **SQLite**（`alerts.db`）。Render 的实例文件系统为 **临时**：

- 重新部署、重启或实例回收后，`alerts.db` 会丢失，已创建的告警需重新添加。
- MVP 阶段可接受；若需持久化，后续可改为 Render Postgres 或外部数据库。

## 部署前自检

- [ ] 代码已支持通过 `PORT` 环境变量监听（main 中已实现）。
- [ ] Render 服务 Root Directory 指向含 `go.mod` 和 `config.yaml` 的目录。
- [ ] Build Command：`go build -o server ./cmd/server`；Start Command：`./server`。
- [ ] 在 Render 环境变量中设置 `WECOM_WEBHOOK_URL`（若需企业微信推送）。
- [ ] 配置 Health Check Path 为 `/health`。
- [ ] （推荐）配置外部定时请求 `/health` 防止 Free Tier 休眠。
- [ ] 已知 SQLite 为临时存储，重启/重新部署会清空告警数据。
