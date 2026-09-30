# CPR 额度失败的最小接入修复

状态：本地修复完成；单元回归、构建和静态检查通过。真实数据库容器补验受本机 Docker 启动故障限制。
基线：Sub2API d130858c7；CPR 3.18.2 e30aad4。

## 已确认的设计树

- 最小改动、复用现有重试次数、退避、冷却和调度缓存：已确认。
- CPR 退出直接 OAuth 的 120 秒重试、延迟冷却、额外换号次数限制：已确认。
- 明确耗尽停止当前同账号重试；跨账号次数继续遵守原配置：已确认。
- 只有结构化耗尽或管理接口确认的账号级限流才补额度冷却；队列、Key 预算、模型限额和容量错误不扩为全账号耗尽：已确认。
- 有效截止时间复用现有限流持久化；无有效时间沿用配置兜底；并发不能缩短已知较长冷却：已确认。
- HTTP、SSE、WS 共用额度事实，保持原始帧、计费、取消和不重放边界：已确认。
- 不增加退避参数、数据库迁移、CPR 内核改造或定时恢复流程：已确认。

本轮“按照上述措施建议完成问题修复和验证”视为对上一轮具体方案的实施确认，无待决定的产品分支。更新术语表；该修复是可逆的接入纠错，不新增 ADR。

## 验收

验证配置次数与直接 OAuth 隔离；明确耗尽与其他错误分类；真实重置时间和缺失/过期时间降级；管理接口超时和查询合并；并发不缩短冷却；HTTP 与 SSE/WS 原始错误保持；已取得 usage、取消与并发收尾保持。

## 实现结果

1. CPR 从直接 OAuth 的 120 秒重试窗口、429 延迟冷却和额外换号次数限制中排除。通用 failover loop、配置次数和退避算法未修改。明确额度耗尽即使开启 pool mode 也不再同账号重试。
2. `openai_cpr_rate_limit.go` 适配结构化额度事实。有效响应 reset 优先；不足时查询绑定管理账号，超时 2 秒，同绑定 singleflight 合并及成功/失败短缓存。只接受确认耗尽的主窗口和账号级 rate_limited 截止时间；缺失、过期、错误绑定、管理失败沿用现有可配置兜底。兜底被关闭时不新增冻结。
3. 复用现有 `SetRateLimitedIfLater`、runtime block、调度快照和 outbox。普通 CPR 429 也使用延长写入，避免并发旧请求把长冷却改回短冷却。没有新数据库字段、恢复状态机或定时补账。
4. HTTP/SSE、Chat、Messages 复用 received-usage capture；WS 在失败帧写出后处理，同帧 headers 的 reset 也保留。Images/Alpha Search 无 capture 的错误入口直接适配。每次/每回合去重，不改原始字节、已取得 usage、取消和输出后不重放约束。

具体模型/Key/队列/容量错误 code 优先于宽泛的 type，不因自由文本或展示 `limitReached` 扩大为全账号耗尽。CPR 3.18.2 的管理 detail 读取存储快照，查询时间不表示新的 OpenAI 额度观测；到达 reset 只是再次尝试时间。客户端断连仍立即关闭上游，状态查询可额外延长本地收尾至其 2 秒上限，写库沿用已有 5 秒状态更新预算。

## 本地验证记录

- 先用回归复现 CPR 带上 OAuth retry deadline，修复后通过。
- `go test -tags=unit -p 2 ./... -count=1`：全量通过。首次因 Windows PATH 缺 `sh` 导致原有备份测试失败；加入本机 Git 的 `usr/bin` 后重跑通过，没有修改备份实现或测试。
- 最后增补错误作用域约束后，定向 CPR / raw relay / OAuth 429 回归：110 个顶层测试、含子例共 178 个通过，0 跳过。覆盖同号重试、原样帧、真实用量、取消收尾、查询超时/合并、错误缓存、绑定变更、并发不缩短、关闭兜底和短兜底。
- `go build -p 2 ./...`：最终源码构建通过。
- 与 CI 相同的 `golangci-lint v2.13.0`：`./cmd/... ./ent/... ./internal/... ./migrations/... ./pkg/...` 共 0 问题。工作区已有且被 Git 忽略的 `backend/tmp/models-preview` 预览程序不属于提交范围，未改动。
- 根仓库 `tools/validate-repo.ps1`：通过；产品仓库 `git diff --check`：通过。
- 初次 `go test -tags=integration -p 2 ./... -count=1` 命令通过，但真实 PostgreSQL/Redis 容器用例因 Docker 未运行而跳过；不将这个结果记为真实数据库通过。单独执行 `TestAccountRepoSuite/TestSetRateLimitedIfLaterDoesNotShortenReset` 也明确报告 Docker unavailable。尝试启动 Docker Desktop 后，其自身报 Ingest socket 初始化错误，未能启动引擎；已停止本次启动的进程，没有重置 Docker 数据或改动生产。因此真实数据库 IfLater 持久化与调度缓存集成断言仍需在可用 Docker/CI 环境运行。新适配逻辑的并发与单调延长已通过本地单元回归。

## 工作区与发布边界

已 fetch 核对：产品仓库 `cce-deploy` 与 `origin/cce-deploy` 在 d130858c7，ahead/behind 为 0/0；本次修复留在本地工作区。根仓库 `main` 与 `origin/main` 为 0/0，原有无关变更保留。没有创建分支、提交、推送或部署本次补丁，也没有改动生产账号/CPR 内核。生产状态以发布记录为准。
