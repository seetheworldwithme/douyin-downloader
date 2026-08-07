# Docker 部署(nginx 镜像 + conda 后端)

不要 Dockerfile、不打包二进制。`start.sh` 在服务器上把三件事串起来:

```
npm run build ─→ server/static        前端产物
conda activate dogs → python run.py --serve  ─→ 127.0.0.1:8000   后端(宿主机)
docker compose up (官方 nginx) ─→ :80  托管前端 + 反代 /api
```

nginx 用 **host 网络**,与宿主机共享网络栈,可直接连 `127.0.0.1:8000`;
后端只绑 loopback,外网不可达,对外只暴露 80。

## 文件

| 文件 | 作用 |
|------|------|
| `start.sh` | **服务器端**一键启动:构建前端 + 起 conda 后端 + 起 nginx |
| `deploy.sh` | **本地端**经 SSH(读 `.env`)拉代码 + 同步密钥 + 远端跑 `start.sh` |
| `docker-compose.yml` | 仅官方 nginx 镜像,host 网络,挂载 `server/static` 与 `nginx.conf` |
| `nginx.conf` | SPA 兜底 + `/api` 流式反代(禁缓冲、长超时) |
| `.env.example` | `.env` 模板(SSH/地址配置,勿提交) |

## 一、服务器一次性准备(在 `101.33.79.160` 上)

1. **装 Docker**(带 compose 插件)+ 把当前用户加入 docker 组:
   ```bash
   sudo apt update && sudo apt install -y docker.io docker-compose-plugin
   sudo usermod -aG docker $USER   # 之后重新登录生效,免 sudo 跑 docker
   ```
2. **装 Node**(构建前端):`sudo apt install -y nodejs npm`(或用 nvm 装 18+ )。
3. **conda 环境 `dogs`** 装好依赖:
   ```bash
   conda create -n dogs python=3.12 -y      # 已存在则跳过
   conda activate dogs
   pip install -r requirements.txt fastapi uvicorn
   ```
4. **克隆代码**到 `.env` 里的 `code_dir`:
   ```bash
   git clone https://github.com/seetheworldwithme/douyin-downloader.git \
     /home/ubuntu/code/PythonProject/douyin-downloader
   ```
5. **放配置**(含密钥,不走 git):把本地 `config.yml`、`.cookies.json` 放到该目录
   (`deploy.sh` 会自动 scp 同步,首次也可手动放)。
6. **网络**:`douyin.xuziyue.work` 的 DNS A 记录指向服务器 IP;
   云厂商**安全组放行 80 端口**(8080/8000 不要对公网开)。

## 二、本地部署(推荐)

仓库根的 `.env`(已 gitignore)按 `.env.example` 填好,然后:

```bash
bash docker/deploy.sh
```

它会:`git pull` → `scp config.yml .cookies.json` → 远端 `bash docker/start.sh`。

## 三、或直接在服务器上启动

```bash
cd /home/ubuntu/code/PythonProject/douyin-downloader
bash docker/start.sh
```

完成后访问 `http://douyin.xuziyue.work/`。

## 日常操作

```bash
# 查看后端日志
tail -f logs/backend.log

# 停止
docker compose -f docker/docker-compose.yml down
kill "$(cat .backend.pid)"

# 更新代码后重新部署(本地一条命令)
bash docker/deploy.sh          # start.sh 会自动重启后端、重建前端、刷新 nginx
```

## 备注

- **为什么后端不在容器里**:你指定用服务器 conda 的 `dogs` 环境;放宿主机跑最简单,
  免去 Dockerfile 与二进制打包。代价:服务器需自带 conda/Node。
- **host 网络**:nginx 容器共享宿主机网络以直连 `127.0.0.1:8000`;若 80 端口被占,
  改 `nginx.conf` 的 `listen` 与(去 host 网络时)`docker-compose.yml` 的端口映射。
- **HTTPS**:当前仅 HTTP(80)。如需 HTTPS(安卓 APK 访问也需要),加 certbot 或在
  nginx 配 443 + 证书;后续可补。
