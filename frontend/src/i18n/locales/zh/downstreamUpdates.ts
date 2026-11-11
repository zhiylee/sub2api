export default {
  currentVersion: '当前自有版本',
  latestVersion: '自有最新版本',
  upToDate: '自有版本已是最新',
  checkFailed: '自有版本检查失败',
  checkWarning: '暂时无法检查自有发布信息，可能显示缓存结果，请刷新重试。',
  dockerHint: 'Docker 部署请切换自有镜像版本并重建容器；在线更新仅影响当前容器。',
  upstream: {
    title: '官方发布',
    baseVersion: '采用的官方基线',
    latestVersion: '官方最新版本',
    newRelease: '官方有新版',
    pendingIntegration: '官方有新版本待同步',
    releasedAt: '发布于 {date}',
    viewRelease: '查看官方发布说明',
    checkFailed: '暂时无法获取官方发布信息',
    cachedWarning: '官方检查失败，显示的是上次查询结果',
    unknownBase: '当前官方基线未知'
  }
}
