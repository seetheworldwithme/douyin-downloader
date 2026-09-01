# douyin-downloader Claude 指引

完整项目规则见 `AGENTS.md`。

## 架构

- **后端**:Go 服务在 `server-go/`(module `github.com/xuziyue/douyin-downloader`)。
  入口 `server-go/cmd/server/main.go`;REST API 在 `server-go/internal/server/`。
- **前端**:Vue 3 + Element Plus + Vite PWA 在 `web/`,构建产物输出到
  `server-go/static/`(由 Go 服务的 SPA 回退和 `docker/` 里的 nginx 容器托管)。
- **安卓**:Kotlin + Compose 原生应用在 `android-app/`(独立于 web 前端,调用
  同一套 REST API),打包脚本 `build-android-app.sh`(Windows Git Bash),产物
  `release/apk/douyin-downloader.apk`,详见 `docs/APK.md`。
- **配置**:`config.yml`(YAML)+ `.cookies.json` / `config/cookies.json`,由 Go
  服务读取。默认值在 `server-go/internal/config/default_config.go`。

## 常用命令

```bash
# 后端:构建并本地运行(纯 Go SQLite,CGO_ENABLED=0 可用)
cd server-go && go build -o ../.bin/server ./cmd/server && ../.bin/server -config ../config.yml

# 后端:测试 / 静态检查
cd server-go && go test ./...
cd server-go && go vet ./... && go build ./...

# 前端:开发(自动代理 /api → 127.0.0.1:8000)/ 生产构建
cd web && npm run dev
cd web && npm run build        # 产物 → server-go/static/,PWA 构建需 Node 20+

# 安卓:一键打包(Windows Git Bash)
bash build-android-app.sh      # debug 包;release 需 BUILD_VARIANT=release

# 部署:在服务器上从仓库根目录一键执行
bash start.sh                  # 重建前端 → 重编 Go → 重启进程 → 重启 nginx 容器
```

## Skills

- **ponytail(马尾辫)说明**:懒人高级开发风格,专治过度设计。输入 `/ponytail` 激活
  (等级:`/ponytail lite|full|ultra`,默认 full)。核心是一把"阶梯":这东西需要
  存在吗 → 仓库里已有吗 → 标准库能做吗 → 平台原生特性吗 → 已装依赖吗 → 一行能
  写完吗 → 最后才写最小实现。产出最短可用 diff,先代码后说明(最多三行),刻意
  偷懒处用 `ponytail:` 注释标记天花板和升级路径。配套命令:
  `/ponytail-review`(对 diff 查过度设计,只找可删项)、`/ponytail-audit`
  (全仓库过度设计审计)、`/ponytail-debt`(收集 `ponytail:` 债务清单)。
  对代码做简洁性修改 / 删冗余 / 反过度设计时用 `/ponytail`。

- **ponytail(马尾辫)使用**:你在修改代码的时候需要你使用这个 skill，不用过度设计以及修改代码，只要能完成功能即可。

## 注意事项

- 后端只实现「单条链接解析 + 流式下载」流程(视频 + 图集);批量模式(用户主页 /
  点赞 / 合集等)未实现,新功能加在 `server-go/internal/server/` 下。
- 修改代码时遵循 `/ponytail`:不过度设计,能删则删,最短可用 diff。
