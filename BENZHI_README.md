# BENZHI_README

基于 Go 实现的城市雨洪调蓄设施汛前安全核验与启用许可 HTTP API 项目，一款后端服务，提供调蓄设施建档、汛前检查、风险识别、缺陷整改、复核放行和不可变汛期启用许可的中文 JSON HTTP 服务。

## 项目说明
- 项目：benzhi-project-c41a0174-ed8e-4671-bd24-4fbb5c1ed2ab
- 项目用途：提供调蓄设施建档、汛前检查、风险识别、缺陷整改、复核放行和不可变汛期启用许可的中文 JSON HTTP 服务。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-c41a0174-ed8e-4671-bd24-4fbb5c1ed2ab-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-c41a0174-ed8e-4671-bd24-4fbb5c1ed2ab-arm64 linux/arm64
docker run -it benzhi-project-c41a0174-ed8e-4671-bd24-4fbb5c1ed2ab-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
