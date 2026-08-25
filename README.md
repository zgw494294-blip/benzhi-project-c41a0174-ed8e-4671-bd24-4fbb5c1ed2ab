# 城市雨洪调蓄设施汛前安全核验与启用许可

本项目为市政运维人员提供调蓄设施建档、汛前现场检查、风险识别、缺陷整改、复核放行和汛期启用许可签发的一体化 JSON HTTP API。服务默认监听 `127.0.0.1:19081`，可使用 `-addr=127.0.0.1:<port>` 或 `PORT` 环境变量配置。

## 构建、运行与测试

```text
go test ./...
go run ./cmd/server -addr=127.0.0.1:19081
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

主要接口包括设施 `/status` 停运与恢复、`POST /api/facilities/{facilityID}/inspection-batches` 开批（需提交带时区的 `windowStart`、`windowEnd`）、读数提交与 `/items/{itemID}/revisions` 修订、批量缺陷分派和期限调整、整改提交与撤回、逐项复核和许可签发。批次查询会返回设施状态、检查窗口、当前读数及历史、最近校验报告、风险评估快照、趋势、缺陷修订、班组和逾期/升级汇总；`GET /api/inspection-batches/{batchID}/assessments?trend=true`、`assessment-diff`、`GET /api/permits/{permitID}/verification` 与 `GET /api/audit-events` 用于解释和追溯。所有公开写请求支持 `Idempotency-Key`，并可在 JSON 中携带 `expectedVersion` 或使用 `If-Match` 进行并发控制；`GET /api/idempotency/{key}` 可查询写请求结果。
