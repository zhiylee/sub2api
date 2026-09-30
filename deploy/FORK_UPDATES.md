# 自定义版本与后台更新

本 fork 的后台更新源为 `zhiylee/sub2api`，版本格式为
`v<社区版本>-zy.<修订号>`，当前版本为 `v0.2.11-zy.1`。

- 同一社区基线每次发布递增修订号；切换社区基线后从 `zy.1` 开始。
- 修订号按数字比较：`zy.2 < zy.10`；社区版本优先于修订号。
- 社区原版在同一基线上视为修订零，但原版的后台仍查询社区仓库，首次必须手动切换到本 fork 的镜像。
- 不覆盖已发布的 tag 或版本镜像。发布说明同时记录社区 tag、commit 和保留的补丁。

## 发布

多个二开功能的集成方式和社区升级步骤见 [二开维护规则](../DOWNSTREAM.md)。
正式发布统一从 `release/<社区版本>` 分支创建 tag，功能分支先合入该分支。

先在 fork 的 Actions 页面启用工作流。x86_64 服务器可设置仓库 Actions
变量 `SIMPLE_RELEASE=true`；ARM64 服务器使用完整发布。

两种发布模式均上传平台二进制包和 `checksums.txt`。简化模式只支持
Linux amd64。后台不会从 GHCR 下载镜像，而是下载 GitHub Release 的二进制包。
Release 及附件必须能被部署服务器访问；当前附件下载不携带私有仓库的认证令牌。

例如在 `release/0.2.11` 完成下一次修改、提交并验证后：

```bash
git switch release/0.2.11
git push -u origin release/0.2.11
# 等待当前提交的 CI 通过后，再推送发布 tag。
git tag -a v0.2.11-zy.2 -m "基于社区 v0.2.11，发布本次集成的二开功能"
git push origin v0.2.11-zy.2
```

tag 推送触发的 Release 工作流会使用这个 tag 中的发布工具。
发布工具只对精确的 `X.Y.Z-zy.N`（N 为正整数）设置
`prerelease: false` 和 `make_latest: true`，其他预发布标签保留上游策略。
直接运行原始 GoReleaser YAML 不会经过这项处理，正式发布应使用 Release 工作流。
fork 发布成功后 VERSION 同步到对应的 `release/<社区版本>` 分支。

发布成功后应有：

- 正式 GitHub Release，例如 `v0.2.11-zy.1`，标记为 Latest。
- `sub2api_0.2.11-zy.1_linux_amd64.tar.gz`（完整发布另有其他平台）。
- `checksums.txt`，其中包含对应压缩包的 SHA256。
- 镜像 `ghcr.io/zhiylee/sub2api:0.2.11-zy.1`。

## 首次部署到 Docker Compose

备份数据库和配置。在现有部署目录添加 `docker-compose.fork.yml`：

```yaml
services:
  sub2api:
    image: ghcr.io/zhiylee/sub2api:0.2.11-zy.1
```

以下假定原文件名为 `docker-compose.yml`、数据库和 Redis 已运行。
如果使用 `docker-compose.local.yml`，相应替换第一个文件名。
私有 GHCR 镜像需要先登录；不要改变原来的项目名和数据目录。

```bash
docker compose -f docker-compose.yml -f docker-compose.fork.yml pull sub2api
docker compose -f docker-compose.yml -f docker-compose.fork.yml up -d --no-deps sub2api
docker compose -f docker-compose.yml -f docker-compose.fork.yml logs --tail=100 sub2api
```

后台应显示版本 `0.2.11-zy.1`。旧社区更新缓存会自动失效。

## 后续后台更新

从 `v0.2.9-zy.1` 升级到本次 `v0.2.11-zy.1`，或发布下一版本
（如 `v0.2.11-zy.2`）后，在后台强制检测更新，确认链接指向
`zhiylee/sub2api`，然后更新并点击重启。默认 Compose 的
`restart: unless-stopped` 会在程序退出后重新启动容器。确认新版本显示正确，
并验证一次实际 API 请求。回退列表同样按社区基线和修订号排序。

后台更新只替换容器内 `/app/sub2api`；前端已嵌入该二进制。
它不会修改 Compose、镜像、外部资源或系统依赖。
后台更新后将 Compose 镜像版本同步到对应版本，并在维护窗口用上述命令拉取和
重建，以固定部署状态；否则未来重建仍会使用旧镜像。
涉及外部资源或运行时依赖的更新，直接采用镜像升级。

数据库迁移在启动时执行。回退二进制或镜像不会撤销迁移，升级前需要保留数据库备份。

## 快速验证

不启动完整应用或构建 Docker 镜像，只验证受影响的更新逻辑与发布配置：

```bash
cd backend
go test -tags=unit ./internal/service/update_service.go ./internal/service/update_service_test.go
cd ..
python3 -m unittest discover -s .github/release-tools -p test_release_matrix.py
sh deploy/tests/docker-runtime-resources-test.sh
```
