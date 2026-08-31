# CCE 无中断滚动部署手册

本文记录 Sub2API 在 Kubernetes/CCE 环境中的安全发布流程，适用于需要保持线上请求连续性的版本验证和升级。示例中的命名空间、Deployment、镜像仓库和提交 SHA 都应替换为目标环境的实际值。

## 发布原则

- 代码先推送到目标分支，由 CI 构建不可变镜像；不要在生产主机上临时编译并直接替换容器。
- 镜像使用 digest 发布，不使用 `latest` 或可变 tag。
- Deployment 必须使用 `RollingUpdate`，并设置 `maxUnavailable: 0`、`maxSurge: 1`。
- 新 Pod 通过 startup/readiness probe 后，才允许旧 Pod 退出；不要先执行 `kubectl delete pod`、`docker compose down` 或强制重启。
- 发布前记录当前镜像 digest、Pod、Deployment revision 和健康状态，发布失败时保留旧 ReplicaSet 以便回滚。

## CI 与镜像

当前仓库的 CCE 验证工作流是 GitHub Actions：

```text
.github/workflows/cce-sse-ws-image.yml
```

它监听 `codex/cce-sub2api-v183-sse-ws` 分支，构建并推送：

```text
ghcr.io/<owner>/sub2api:cce-v183-sse-ws-<commit-sha>
```

工作流还会上传同名镜像归档，保留 7 天。GitCode/AtomGit 并不是该工作流的默认构建入口；除非仓库另有 CI 配置，否则推送到 GitCode 不会自动触发这里的构建。

### 网络慢时的处理

生产 CCE 节点通常应从国内 ACR 拉取。若节点访问 GHCR 超时：

1. 等待 GitHub Actions 成功，确认镜像归档可下载。
2. 使用已认证的 GitHub CLI 下载归档，例如 `gh run download <run-id> ...`。
3. 校验归档 SHA-256 后传到有权限的运维节点。
4. 在运维节点执行 `docker load`，再给镜像打 ACR 的唯一 tag 并 `docker push`。
5. Kubernetes 只引用 ACR digest。

不要把 registry token、云账号密码或归档内容写进日志、提交或公开文档。

## 发布前检查

```bash
kubectl -n <namespace> get deploy <deployment> -o wide
kubectl -n <namespace> get pods -l app.kubernetes.io/name=sub2api -o wide
kubectl -n <namespace> rollout history deploy/<deployment>
kubectl -n <namespace> get events --sort-by=.lastTimestamp | tail -20
```

确认 Deployment 至少满足：

```yaml
spec:
  replicas: 1
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0
      maxSurge: 1
```

同时确认 `/health`、readiness endpoint、数据库和 Redis 均正常。发布前不要只看 Pod 名称排序，旧 Pod 在滚动期间可能仍处于 `Terminating`；检查时应明确选择 `Running` 且 `READY=true` 的新 Pod。

## WS 连接容量配置

连接池参数必须满足配置校验约束：

```text
max_idle_per_account <= max_conns_per_account
min_idle_per_account <= max_idle_per_account
```

推荐的验证配置示例：

```text
GATEWAY_OPENAI_WS_MAX_CONNS_PER_ACCOUNT=8
GATEWAY_OPENAI_WS_MAX_IDLE_PER_ACCOUNT=8
GATEWAY_OPENAI_WS_MIN_IDLE_PER_ACCOUNT=0
GATEWAY_OPENAI_WS_DYNAMIC_MAX_CONNS_BY_ACCOUNT_CONCURRENCY_ENABLED=true
GATEWAY_OPENAI_WS_OAUTH_MAX_CONNS_FACTOR=2.0
GATEWAY_OPENAI_WS_APIKEY_MAX_CONNS_FACTOR=1.0
```

其中账号推理并发仍由账号调度层限制，factor 只决定可建立的物理 WebSocket 数量。设置 `max_conns_per_account=8` 而保留 `max_idle_per_account=128` 会导致新 Pod 启动即退出并进入 `CrashLoopBackOff`。调整容量参数时，应在同一份 Deployment 更新中同步调整 idle 上限。

## 无中断发布

使用 digest 更新镜像：

```bash
kubectl -n <namespace> set image deploy/<deployment> \
  sub2api=<acr-registry>/sub2api@sha256:<image-digest>
kubectl -n <namespace> annotate deploy/<deployment> \
  sub2api.2ray.wang/source-revision=<commit-sha> --overwrite
kubectl -n <namespace> rollout status deploy/<deployment> --timeout=600s
```

容量环境变量建议和镜像在一次变更中更新；如果分两次更新，每次都必须等待 rollout 成功，不要在中间删除旧 Pod。

### 失败处理

先查看新 ReplicaSet 和容器日志：

```bash
kubectl -n <namespace> get pods -l app.kubernetes.io/name=sub2api -o wide
kubectl -n <namespace> describe pod <new-pod>
kubectl -n <namespace> logs <new-pod> --previous --tail=120
```

只要旧 Pod 仍为 `Running/Ready`，线上通常仍有服务。修正镜像或环境变量后重新触发滚动更新；不要强制终止旧 Pod。若必须回滚，使用保留的 revision：

```bash
kubectl -n <namespace> rollout undo deploy/<deployment> --to-revision=<revision>
kubectl -n <namespace> rollout status deploy/<deployment> --timeout=600s
```

## 发布后验证

```bash
kubectl -n <namespace> get deploy <deployment> -o wide
kubectl -n <namespace> get pods -l app.kubernetes.io/name=sub2api -o wide
kubectl -n <namespace> exec <running-pod> -- wget -q -O- http://127.0.0.1:8080/health
curl -fsS https://<public-host>/health
kubectl -n <namespace> logs <running-pod> --since=5m | \
  grep -Ei 'panic|fatal|startup failed|listen tcp|migration failed|Failed to load config' || true
```

应看到：Deployment `READY/UP-TO-DATE/AVAILABLE` 均达到期望值；新 Pod `Running`、`READY=true`、重启次数为 0；Pod 内和外部健康端点均返回 HTTP 200；没有启动、迁移或监听失败日志。发布后至少观察一个健康检查周期，确认没有新的重启或 readiness 失败。

## 本次发布的经验

- GitHub Actions 归档比从 GHCR 直接拉取更适合网络受限的 CCE 节点；归档传输完成后可在国内 ACR 复用。
- 远端 Docker daemon 可能使用 legacy builder 且配置 `bridge=none`。根 `Dockerfile` 中的 `BUILDPLATFORM` 在旧 builder 下可能解析失败，构建前应确认 builder 版本；必要时使用兼容的部署 Dockerfile 或 CI 产物。
- “新 Pod 健康后再删旧 Pod”不仅依赖 rollout 命令，还依赖 `maxUnavailable=0`、readiness/startup probe 和足够的 `terminationGracePeriodSeconds`。
- 配置校验失败时，Kubernetes 会继续保留旧 ReplicaSet；这是无中断修复的关键，不应为了清理状态而先删除旧 Pod。
- 生产验证应优先做健康、版本、日志和连接池容量检查；使用记录/billing、灰度策略和临时 SSE 回退不应混入这类 WS 修复发布。
