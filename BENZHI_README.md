# BENZHI_README

基于 Go 实现的stage-rigging-clearance HTTP API 项目，一款后端服务，已完整实现面向剧院技术团队的舞台吊挂设备安全检验 HTTP 服务，覆盖建档、基线锁定、负载测试、缺陷整改、独立复核、报告冻结、复役凭据、SQLite 恢复校验和连续哈希审计轨迹。

## 项目说明
- 项目：benzhi-project-aa687564-665f-480d-bf5a-00f0548d4af4
- 项目用途：已完整实现面向剧院技术团队的舞台吊挂设备安全检验 HTTP 服务，覆盖建档、基线锁定、负载测试、缺陷整改、独立复核、报告冻结、复役凭据、SQLite 恢复校验和连续哈希审计轨迹。
- Go 工具链：`golang:1.22`
- 前端工具链：无

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-aa687564-665f-480d-bf5a-00f0548d4af4-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-aa687564-665f-480d-bf5a-00f0548d4af4-arm64 linux/arm64
docker run -it benzhi-project-aa687564-665f-480d-bf5a-00f0548d4af4-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -selfcheck -addr=127.0.0.1:19081`
