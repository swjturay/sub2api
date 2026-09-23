# Insights 运行与接入

独立 React 静态应用部署到原站同域 `/insights/`。后端仍为 Sub2API；所有 `/api/v1/insights/*` 和 `/api/v1/admin/insights/*` 请求沿用原 JWT 和管理员鉴权。浏览器只保存原站已有会话字段，URL 不携带 token。

## 镜像与路由

后端使用本分支正常 Dockerfile，React 使用 `insights/Dockerfile`。已有 Ingress 或反向代理需添加优先级更高的 `/insights/` 前缀路由，转至 React 服务 80 端口；其余请求仍转原 Sub2API。保持路径前缀，勿重写成 `/`，并保留 SPA 深层路径回退。后端、前端可独立构建，须按同一版本的数据契约发布。

Docker Compose 可使用本仓库附带的覆盖配置：

```sh
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.insights.yml config
docker compose -f deploy/docker-compose.yml -f deploy/docker-compose.insights.yml up -d --build
```

继承原部署的数据库、Redis、固定 JWT/TOTP 密钥及 `.env`，不要为已有站点重新初始化账号。覆盖配置的同域入口默认绑定 `127.0.0.1:8081`，在现有 TLS 入口之后使用。原站 `/login`、`/api/*`、模型转发及 WebSocket 仍转向原后端；SSE 关闭代理缓冲，WebSocket 保留 Upgrade。TLS、外部访问域名与可信代理配置继续由原入口管理。仅构建与本地配置验证不代表已上线。

## 部门绑定

部门来源为现有 `user_attribute_definitions` 与 `user_attribute_values`。仅接受 `text`、`textarea`、单选 `select`；`研发/基础平台` 是一个完整部门值，不自动展开层级。缺失值为未分配，多选或无效定义会明确提示。

若存在唯一启用的 `department`/`dept` 字段，可自动发现。其他键需在现网只读核实字段 id、key、type、options 后，将已核实 id 写入 `insights_settings.department_attribute_id`。例如使用参数化 SQL 由运维执行：

```sql
INSERT INTO insights_settings(key,value,updated_at)
VALUES ('department_attribute_id',to_jsonb($1::bigint),NOW())
ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value,updated_at=NOW();
```

不预设钉钉字段 id/key，不创建新部门体系。普通用户不能修改这一系统配置。

## 采集与覆盖

HTTP 生成调用、流式终态与 WS 每个生成 turn 写入独立最终结果；内部重试不增加客户端调用数。用量、最终结果、错误使用同一个统计记录时间。旧 usage 可用于原有 Token/用量统计，但不补造历史成功率或历史首次请求。

采集默认保存已观察事实，并标注覆盖不完整。全部实际服务实例、入口与终态钩子部署并验证后，运维才能设置 `INSIGHTS_TRUSTED_COLLECTION=true`（配置文件 `insights.trusted_collection: true`）。这是明确的部署完成标记，不因某个进程启动自动宣称全量覆盖。无历史可信标记时只从真实启用时点开始；已有可信区间可保留，队列丢弃、写入失败或未观测终态会将状态降为 partial。修复缺口后重新建立的可信区间不回溯填补旧数据。

如果已经审计确认原计费账本 `usage_logs` 自某个自然日起连续完整，可同时设置 `INSIGHTS_TRUSTED_USAGE_HISTORY_FROM=YYYY-MM-DD`（配置文件 `insights.trusted_usage_history_from`）把该日期至实时可信区间之间的 **Token 与实际金额** 标记为可确认历史。该日期按平台时区解释，只允许在 `INSIGHTS_TRUSTED_COLLECTION=true` 时启用，且不会跨越已经记录的采集缺口。此配置只认证账本中真实存在的用量和零用量日期，不会补造历史请求成功率、失败数、错误详情、首次请求或留存事实。变更前应以只读 SQL 核对最早记录、日期连续性、实例写入路径和数据清理历史。

后台按分钟维护近期日汇总，并按天分段初始化历史；用户生命周期也按批次更新。已有明确调用的旧用户计入首次请求人数；真实首次日期未知时保留独立标记，不把首次观察记录冒充终身首次锚点。缺乏判断依据的留存层显示未知，后来的覆盖缺口不撤销已确认的里程碑。正常停止时先排空事实队列，再关闭数据库连接。

## 保留与回滚

目标保留：usage/最终调用明细 365 天，错误明细 30 天，用户/模型日汇总当前与前两个自然年，首次请求和留存达成时刻保留账号生命周期。页面返回实际配置和覆盖范围；已有更短保留不等于可恢复缺失历史。每日额度始终来自原订阅账本。

迁移 `239_insights_v1.sql` 新增独立表，`240_insights_observed_activity.sql` 保存“已知曾请求”的持久证据，现有身份、计费与路由表不迁移归属。上线前核实真实部门定义、运行保留设置、历史完整性及查询负载；当前开发验证使用隔离 PostgreSQL 的合成数据，未验证生产容量。

回滚时撤下 `/insights/` 入口和导航对应前后端版本，恢复原镜像及路由；保留新增表以便回退排查和再次升级，勿在回滚中删除原有业务数据。没有新增导出、共享链接或真实模型测试调用。
