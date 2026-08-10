# 部署:edge-nginx + 本地端口容器(参照 calendar-site)

服务器上 `edge-nginx` 容器(host 网络)独占公网 80/443,做 TLS 终结 + 按域名反代到
各站点的本地端口容器。本服务照此模式:

```
                    edge-nginx(host 网络,80/443,LE 证书)
                         │  server_name douyin.xuziyue.work
            ┌────────────┴────────────┐
   location /                location /api/
      ↓                          ↓
 douyin-web 容器            Go 后端 宿主机进程
 127.0.0.1:8083 (nginx)     127.0.0.1:8000 (server-go)
 纯静态托管 SPA              解析 + 流式中转(不落盘)
```

不占 443、不碰 `edge.conf`(只在 `conf.d/` 新增一个文件)、后端只绑 loopback。

## 文件

| 文件 | 作用 |
|------|------|
| `start.sh` | **服务器端**:构建前端 + 编译起 Go 后端 + 起前端容器 |
| `deploy.sh` | **本地端**:SSH 拉代码 + 同步密钥 + 跑 start.sh + 落 edge-nginx 配置并 reload |
| `docker-compose.yml` | 前端官方 nginx 镜像,`127.0.0.1:8083:80`,挂载 `server/static` + `nginx.conf` |
| `nginx.conf` | 前端容器纯静态 server block(SPA + 缓存,不处理 /api) |
| `edge-proxy/douyin.xuziyue.work.conf` | **edge-nginx 的 server block**:443 + 证书 + `/`→8083 + `/api/`→8000 |
| `.env.example` | `.env` 模板(SSH/地址,勿提交) |

## 一次性准备(服务器 `101.33.79.160`)

1. **Docker**(带 compose 插件)+ `sudo usermod -aG docker ubuntu`(重新登录生效)。
2. **Node**(前端构建用)。
3. **Go**(≥ 1.26,后端编译用):见 https://go.dev/dl/ 。
4. **clone 代码**到 `code_dir`;**DNS** `douyin.xuziyue.work` → 服务器 IP;证书已存在
   (`/etc/letsencrypt/live/douyin.xuziyue.work/`)。
5. **证书续期切 webroot**(一次性,见下「证书续期」)。

## 部署(本地一条命令)

仓库根 `.env` 按模板填好后:

```bash
bash docker/deploy.sh
```

完成后访问 `https://douyin.xuziyue.work/`。

## 证书续期(切 webroot,一次性)

`douyin.xuziyue.work` 的 renewal 原为 `authenticator=nginx`,宿主 nginx 停后无法续期。
edge-nginx 的 80 端口默认 server 已服务 `/.well-known/acme-challenge/`,切到 webroot 即可静默续期:

```bash
# 在服务器上(sudo)
sudo sed -i 's/^authenticator = nginx/authenticator = webroot/' /etc/letsencrypt/renewal/douyin.xuziyue.work.conf
sudo certbot renew --webroot -w /var/www/certbot --cert-name douyin.xuziyue.work --dry-run
```

dry-run 通过即说明续期链路正常。三个证书都可照此切 webroot。

## 日常

```bash
# 更新代码后重新部署
bash docker/deploy.sh

# 后端日志
tail -f logs/backend.log

# 停止
docker compose -f docker/docker-compose.yml down
kill "$(cat .backend.pid)"
```

## 备注

- 前端改了只需 `npm run build` + 重启前端容器(`deploy.sh` 自动完成);后端改了重启后端进程。
- edge-nginx 配置变更只在 `conf.d/` 新增/覆盖 `douyin.xuziyue.work.conf`,**不动 `edge.conf`**;
  `deploy.sh` 会先 `nginx -t` 校验全部配置再 reload,失败不 reload,不影响其他站。
