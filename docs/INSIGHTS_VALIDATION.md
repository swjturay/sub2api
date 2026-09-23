# Insights v1 验证记录

验证环境为隔离的本地 PostgreSQL 16.15、Redis 和真实 Sub2API 后端，前端通过同源 `/insights/` 访问。数据全部为测试夹具；没有读取生产业务数据、调用收费模型或部署生产环境。

## 自动检查

- 后端全仓库 `go test -tags=unit ./...` 已通过；最终变更后的 Insights、handler、routes 与 migrations 整包回归也通过，生产 embed 构建成功。

- 原 Vue 前端：lint、类型检查、最终 26 个关键测试文件 / 348 项测试通过，包含全部语言消息编译和独立应用登录返回。
- React 前端：最终 13 个文件 / 43 项测试通过；lint、类型检查和生产构建由实现代理完成，独立验收再次运行 13 个文件 / 43 项测试通过。
- 新空数据库：原完整迁移链及 `239_insights_v1.sql`、`240_insights_observed_activity.sql` 执行成功。
- PostgreSQL 实算覆盖用户隔离、部门完整值、日期边界、Token/缓存率公式、模型平台归属、零费用记录、逐请求流式 TPOT、部分桶分钟分母、个人/部门及网关 daily/raw 合并、频次边界、可信留存、共享热力色阶。
- 独立来源日汇总、乱序写入、重复清理后保留、生命周期在明细清除后保留均有真实 PostgreSQL 回归。
- UTC 数据库会话与 Asia/Shanghai 分析时区交叉验证，未覆盖的上午不会被认证为完整自然日。
- 调用与用量覆盖相互隔离；丢失通知通道满时，内存完整性也立即降级；用量排队、写入失败及任务异常分别跟踪。
- 带 `unit` 标签的 Insights、RecordUsage、UsageCleanup、DashboardAggregation 相关 service/repository/handler/migration 检查通过。带标签检查不可用无标签结果代替。
- Docker Compose 叠加配置 `config --quiet` 通过。本机 Docker daemon 不可用，因此不宣称容器镜像已实际运行。

## 真实接口与浏览器

已验证：

- 未登录深链跳原登录，密码登录后回到目标 Insights 页面，URL 不携带 token。
- 普通用户只显示个人/模型导航；管理员 dimensions 和模型资料写接口对普通用户返回 403；个人接口按服务端认证账号查询。
- 四模型目录、双模型比较、TPM/RPM/TTFT/TPOT 趋势、资料完整字段及 409 乐观锁冲突。
- 恶意模型标签作为文本显示，没有执行 HTML/JavaScript。
- 当天两份订阅、非零 Token、历史模型分布和稳定的 20 条游标分页。
- 人为离线刷新后保留旧值和筛选，显示刷新失败，成功更新时间不前移。
- 深浅色切换及 390×844 基本布局。

固定测试区间基准（2026-09-16 至 2026-09-22，Asia/Shanghai）：

| 项目 | 结果 |
| --- | --- |
| 部门当前总成员 / 活跃成员 | 6 / 5 |
| 部门总 Token / usage 记录数 | 58,174 / 273 |
| 网关最终调用总数 / 失败数 | 298 / 26 |
| 请求成功率 | 272 / 298 = 91.2751678% |
| 成功模型平均耗时 / 网关平均耗时 | 1250 ms / 25 ms |
| 期末用户 / 新增用户 | 6 / 1 |
| 高频 / 中频 / 低频 | 1 / 2 / 3 |

用量记录数和最终客户端调用数是不同总体；夹具中另有旧 usage 无新调用事实，以验证不会混为一个指标。

## 数据与协议边界

- 历史新指标、可信首次请求未知时保留 unknown/partial；不会由最早留存记录补造账号终身首次请求。
- WebRTC 直连媒体绕过 Sub2API，服务器无法可靠观察每个模型 turn。OpenAI Live 创建与 sideband 控制不冒充模型调用，相关请求使覆盖状态明确为不完整。
- Grok/xAI server-VAD 自动触发 turn 可见结果但不可见实际发送边界，保留结果与覆盖提示；没有实际计时依据时不提供伪造的阶段耗时。
- 批量任务中的 provider 状态时间不能代替真实网络 send 测量。无可靠样本的耗时保持空值。
- 原生实时音频按连接时长计费；它与逐 turn 调用事实是不同统计单位。既有计费规则不因分析而改变。
- OAuth 外部供应商与真实 2FA 授权跳转、现网部门属性、历史保留设置及生产负载尚未现场验证。已保留原登录流程并测试本地返回适配；这些结果不等于外部生产登录已实测。

## 最终收尾状态

2026-09-23 最终独立复验结果：

- 异步图片、批量图片和 Cyber nil-request 修复已通过独立定向检查：`go test -tags=unit ./internal/handler -run 'TestCyber|TestAsyncImage' -count=1`、`go test -tags=unit ./internal/service -run 'TestBatchImage' -count=1`、`go test ./internal/service -run 'TestBatchImageItemFact' -count=1`。
- 批量图片事实优先使用持久 `indexed_at`，未知网络发送边界不再生成 attempt/gateway/model duration；索引解析失败为每个已提交 item 生成稳定失败事实；异步图片 2xx JSON 未观察到协议终态时记录 `terminal_unobserved`，不再冒充成功。
- 多副本覆盖写入使用事务和 `SELECT ... FOR UPDATE` 合并。`mergeCoverage` 保留较新的 trusted boundary、gap 和 observed-through；陈旧 complete heartbeat 不能覆盖 fleet gap。实现与 `TestHealthyReplicaCannotEraseFleetGap` 的保守合并语义一致。
- 专用 Playwright 会话 `insights-final-validation` 通过真实同源 `4187` 验证管理员部门筛选、部门/成员双帕累托控件、独立性能模型筛选、网关质量/用户曲线、漏斗、部门偏好、普通用户管理路由回退及管理 API 403。
- 近 7 日网关真实 API 与固定夹具一致：298 次、26 失败、91.2751678%，模型/网关时长 1250/25 ms，6 位期末用户、1 位新增，高/中/低频 1/2/3。最后一个日桶明确 `complete=false`；漏斗保持 6→5→5→5→5；部门偏好每条比例合计 100%。
- 普通用户 2025/2026 热力图均返回相同共享色阶；年度图正常渲染。最终真实 API 返回 `2026-01-28 = missing`、`2026-09-22 = 533/value`。移动视口实际点击 9 月 22 日后，下方开始/结束日期均同步为 `2026-09-22`，已选择模型 `openai:gpt-4o` 保留；点击 1 月 28 日缺失格后日期和模型不变。

此前部门性能卡片把非零平均 RPM 显示为 `0` 的缺陷已关闭。最终真实页面显示全部门平均 RPM `0.0256`，小额金额保留两位小数；null 继续显示 `—`。tooltip 使用完整计数，紧凑格式只用于轴标签。

未现场验证的边界保持不变：外部 OAuth/2FA 供应商、生产部门属性和负载、Docker daemon 实际镜像运行、服务器不可观察的纯 WebRTC turn。它们不计作失败，也不宣称通过。



### 最终 PC 视觉范围（2026-09-23）

用户最终验收范围收敛为 PC。资源 `index-DlPECH5X.js` 通过真实 4187 同源边缘复验：1366×768、1440×900、1920×1080 的四页均无页面级横向溢出，1920 主内容最大 1500px；顶部导航、active indicator、浅深主题、图表/表格/筛选均正常。管理员个人与管理接口为 200，普通用户个人接口为 200、管理 dimensions 为 403。稳定浏览期间没有 console/page/HTTP 错误。最终 PC 截图位于 `output/playwright/insights-final-*-pc-*.png`。手机端不在用户最终保证范围内，不作为发布阻塞。

### Release build 最后确认（2026-09-23）

最新 `index-DLO_wBUk.js` 在真实 4187 → release 后端通过限定 PC 集成：1366×768 部门九指标为 3+6 等宽布局，无截断，固定区间输出 Token 为接口真实值 11,238；网关请求质量五列无空槽，固定区间为 298 / 26 / 91.3% / 1250ms / 25ms；个人当天概览按两份订阅自动形成三个等宽列。漏斗返回 6→5→5→5→5，首次及留存层明确为 partial，页面说明待观察/覆盖缺失不算流失；nullable/unknown 层使用 `—` 和“数据不足”，不伪造 0。四页稳定期间无 console/page/HTTP 错误或页面级横向溢出。

迁移 240 的 `first_observed_call_at` 只记录已观察活动证据：老账户可回填 observed anchor，但没有可信首次边界时 `first_call_at` 保持 null，未知漏斗层 count/ratio 继续为 null。真实第三方 OAuth IDP 未访问；仅确认回跳 helper 的 provider 单元测试，不能表述为外部授权实测。

最终截图：`output/playwright/insights-release-departments-1366x768-light.png`、`output/playwright/insights-release-gateway-1366x768-light.png`。本轮 PC 集成无阻塞缺陷，验收结束。
