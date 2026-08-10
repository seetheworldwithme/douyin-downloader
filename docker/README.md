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
| `start.sh`(仓库根) | **服务器端一键**:构建前端 + 编译起 Go 后端 + 起前端容器 |
| `docker-compose.yml` | 前端官方 nginx 镜像,`127.0.0.1:8083:80`,挂载 `server/static` + `nginx.conf` |
| `nginx.conf` | 前端容器纯静态 server block(SPA + 缓存,不处理 /api) |
| `edge-proxy/douyin.xuziyue.work.conf` | **edge-nginx 的 server block**:443 + 证书 + `/`→8083 + `/api/`→8000 |

## 一次性准备(服务器 `101.33.79.160`)

1. **Docker**(带 compose 插件)+ `sudo usermod -aG docker ubuntu`(重新登录生效)。
2. **Node**(≥ 20,前端构建用)。
3. **Go**(≥ 1.26,后端编译用):见 https://go.dev/dl/ 。
4. **clone 代码**到服务器;**DNS** `douyin.xuziyue.work` → 服务器 IP;证书已存在
   (`/etc/letsencrypt/live/douyin.xuziyue.work/`)。
5. **证书续期切 webroot**(一次性,见下「证书续期」)。

## 部署(服务器上一条命令)

在服务器上(代码已 clone 到本地),仓库根执行:

```bash
git pull          # 拉最新代码(可选)
bash start.sh
```

`start.sh` 会:① `npm run build` 构建前端 → `server/static`;② 编译并后台启动 Go 后端
(`127.0.0.1:8000`);③ `docker compose up -d` 启动前端 nginx 容器(`127.0.0.1:8083`)。
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
# 更新代码后重新部署(服务器上)
git pull && bash start.sh

# 后端日志
tail -f logs/backend.log

# 停止
docker compose -p douyin -f docker/docker-compose.yml down
kill "$(cat .backend.pid)"
```

## 备注

- 前端或后端改了,服务器上 `bash start.sh` 即可重新构建并重启(后端重启会先 kill 旧 PID)。
- **edge-nginx 是独立仓库**(`/home/ubuntu/code/edge-proxy/`),`start.sh` 不管理它。若改了
  `docker/edge-proxy/douyin.xuziyue.work.conf`,需手动复制到 edge-nginx 的 `conf.d/` 再 reload:
  `docker exec edge-nginx nginx -t && docker exec edge-nginx nginx -s reload`(先 `-t` 校验,失败不 reload)。
