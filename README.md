# stage-rigging-clearance

`stage-rigging-clearance` 是面向剧院技术团队的舞台吊挂设备安全检验服务。它通过单一 JSON HTTP API 固化从检验建档、设备安全基线、负载测试、缺陷整改、独立复核，到冻结报告和签发复役凭据的完整闭环。

服务将档案聚合、设备、原始测试读数与复测关系、缺陷证据版本与逐项决定、冻结报告、凭据、幂等响应和审计事件保存到本地 SQLite。每个变更命令都要求 `expectedVersion`、`idempotencyKey`、`actor` 和 `role`。同一档案的命令在进程内串行执行，SQLite 写事务以版本条件提交聚合、关系明细、幂等响应和连续哈希审计事件。

## 构建、运行和测试

要求 Go 1.22 或更高版本。

```text
go build ./cmd/server
go test ./...
```

正常启动：

```text
go run ./cmd/server -addr=127.0.0.1:19081 -db=stage-rigging-clearance.db
```

默认监听 `127.0.0.1:19081`，不会绑定 `0.0.0.0`。可以显式传入 `-addr=127.0.0.1:<port>`；也可以设置 `PORT` 为端口号，服务会监听 `127.0.0.1:<PORT>`。`-addr` 显式参数优先于 `PORT` 产生的默认地址。

运行有界自检：

```text
go run ./cmd/server -selfcheck -addr=127.0.0.1:19081
```

自检使用真实回环监听和内存 SQLite，经 HTTP 依次完成建档、基线、四类测试、提交复核、复核通过、冻结、签发、审计查询和凭据校验，结束后关闭服务并输出 `selfcheck: ok`。

## API 约定

所有写接口使用 `Content-Type: application/json`。命令公共字段如下：

```json
{
  "expectedVersion": 1,
  "idempotencyKey": "client-unique-key-0001",
  "actor": "操作人员姓名或编号",
  "role": "inspector"
}
```

角色值为：

- `inspector`：建档、基线、测试、人工缺陷和提交复核。
- `maintenance_manager`：提交整改证据，也可提交复核。
- `independent_reviewer`：退回、确认通过、冻结报告和签发凭据。

独立复核员不能与档案中的测试采集责任人相同。除批量设备登记返回逐项结果外，写请求成功后会返回完整档案及递增后的 `version`。相同 `idempotencyKey` 和相同请求会返回首次确定性响应；复用该键提交不同请求会返回 `409`。版本冲突同样返回 `409`。错误使用稳定的 `application/problem+json` 结构。

主要路由：

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| `POST` | `/api/v1/inspection-cases` | 创建检验档案 |
| `GET` | `/api/v1/inspection-cases` | 筛选、分页查询工作队列及风险统计 |
| `GET` | `/api/v1/inspection-cases/{caseNumber}` | 查询完整档案状态 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/prepare` | 进入基线准备态 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/assets` | 登记受检设备 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/assets/batch` | 原子登记 1 至 100 台受检设备 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/baseline/lock` | 校验并锁定安全基线 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/tests` | 保存测试原始读数并自动判定 |
| `GET` | `/api/v1/inspection-cases/{caseNumber}/tests/coverage` | 查询按设备和四类测试生成的覆盖矩阵 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/defects` | 登记人工观察缺陷 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/defects/{defectId}/remediation` | 提交整改证据 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/defects/{defectId}/review` | 接受或附理由驳回缺陷最新证据 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/review/submit` | 提交独立复核 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/review/return` | 附理由退回 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/review/approve` | 确认复核通过 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/report/freeze` | 冻结规范化报告及摘要 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/certificate/issue` | 签发不可变复役凭据 |
| `GET` | `/api/v1/inspection-cases/{caseNumber}/audit` | 查询并校验审计轨迹 |
| `GET` | `/api/v1/inspection-cases/{caseNumber}/certificate` | 查询并校验凭据摘要 |
| `POST` | `/api/v1/inspection-cases/{caseNumber}/certificate/verify` | 核验客户端携带的完整凭据内容 |
| `GET` | `/healthz` | 存活检查 |

设备 `assetType` 支持 `batten`、`winch`、`wire_rope`、`brake` 和 `limit_device`。批量登记请求使用 `assets` 数组，成功响应按请求顺序给出 `id`、`normalizedAssetCode`、`result`，以及 `addedCount` 和 `latestVersion`；批内或档案内编号冲突会使整批回滚。

测试 `testKind` 支持 `static_load`、`dynamic_load`、`brake_distance` 和 `limit_trigger`。静载要求至少达到额定载荷的 125%，动载要求至少达到 110%，制动距离和限位触发按设备锁定基线判定。失败读数会保留并自动生成分级缺陷。复测可在 `test` 中使用 `originalRecordId`（兼容 `retestOfRecordId`）引用同设备、同类型且尚未被后续复测取代的失败记录；合格复测不会自动关闭原缺陷。

整改证据每次提交都会追加带提交人和时间的版本。逐项复核请求使用 `accepted` 和 `comment`；驳回会将档案退回，且必须追加新证据才能再次提交。整体通过要求每个缺陷的最新证据版本已被逐项接受。

工作队列支持 `status`（可重复或逗号分隔）、`venueName`、`updatedFrom`、`updatedTo`、`highestSeverity`、`limit` 和 `cursor`。时间使用 RFC3339，`limit` 范围为 1 至 100。结果按 `updatedAt` 降序、`caseNumber` 升序排列，响应同时返回各状态与未解决缺陷级别的零值完备统计以及快照游标。

凭据携带核验请求包含 `certificateNumber`、`certificateId`、`caseNumber`、`reportDigest`、`issuedAt` 和 `verificationDigest`。核验只返回匹配结论和稳定原因，不返回未提供档案的业务详情，也不会推进档案版本或写入审计链。

服务启动时校验 `schemaVersion`、SQLite 外键、聚合快照与关系明细、冻结报告和凭据摘要，以及审计事件的序号和前序摘要；发现损坏会拒绝正常打开存储。
