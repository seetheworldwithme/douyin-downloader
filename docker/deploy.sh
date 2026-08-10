#!/usr/bin/env bash
# 从本地经 SSH 部署到服务器(凭据读自仓库根的 .env):
#   1) 远端 git pull(有本地改动则 stash 后再拉,保住改动)
#   2) scp 同步 config.yml / .cookies.json(gitignored,含密钥)
#   3) 远端执行 docker/start.sh:构建前端 + 编译起 Go 后端 + 起前端容器
#   4) 把 edge-nginx 的 douyin server block 落到 conf.d,nginx -t 校验后 reload
#
# 用法:在仓库根执行 `bash docker/deploy.sh`
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_DIR/.env"

command -v sshpass >/dev/null 2>&1 || { echo "缺少 sshpass。macOS: brew install esolitos/ipa/sshpass"; exit 1; }
[[ -f "$ENV_FILE" ]] || { echo "缺少 $ENV_FILE(参考 docker/.env.example)"; exit 1; }

get() {
  grep -iE "^${1}:" "$ENV_FILE" | head -1 \
    | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d '\r' | sed 's/[[:space:]]*$//'
}

IP=$(get ip); REMOTE_USER=$(get user); PASS=$(get password); CODE_DIR=$(get code_dir); URL=$(get URL)
for v in IP REMOTE_USER PASS CODE_DIR; do
  [[ -n "${!v:-}" ]] || { echo ".env 缺少字段:$(echo $v | tr 'A-Z' 'a-z')"; exit 1; }
done

echo "→ 目标:${REMOTE_USER}@${IP}:${CODE_DIR}   访问:${URL:-<未设 URL>}"
SSH_OPTS=(-o PreferredAuthentications=password -o PubkeyAuthentication=no -o StrictHostKeyChecking=accept-new)

echo "== [1/4] 远端 git pull =="
sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "
  set -e
  cd '${CODE_DIR}'
  if git pull --ff-only; then
    echo 'pull OK'
  else
    echo '有本地改动,先 stash 再 pull(改动保留在 stash)'
    git stash && git pull --ff-only
  fi
  echo \"HEAD: \$(git rev-parse --short HEAD)\"
"

echo "== [2/4] 密钥配置(远端为权威,仅在缺失时推送)=="
for f in config.yml .cookies.json; do
  if sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "test -f '${CODE_DIR}/${f}'"; then
    echo "  远端已有 $f,保留不覆盖"
  elif [[ -f "$REPO_DIR/$f" ]]; then
    echo "  远端缺 $f,从本地推送"
    sshpass -p "$PASS" scp "${SSH_OPTS[@]}" "$REPO_DIR/$f" "${REMOTE_USER}@${IP}:${CODE_DIR}/${f}"
  else
    echo "  ⚠ 本地与远端均无 $f(后端需要它)"
  fi
done

echo "== [3/4] 远端执行 start.sh =="
REMOTE_CMD="bash -lc 'cd \"${CODE_DIR}\" && bash docker/start.sh'"
sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "${REMOTE_CMD}"

echo "== [4/4] 配置 edge-nginx 的 douyin server block + reload =="
sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "
  set -e
  cp '${CODE_DIR}/docker/edge-proxy/douyin.xuziyue.work.conf' /home/ubuntu/code/edge-proxy/conf.d/
  echo 'nginx -t 校验(含所有 conf.d,失败则不 reload):'
  docker exec edge-nginx nginx -t
  docker exec edge-nginx nginx -s reload
  echo 'edge-nginx 已 reload'
"

echo
echo "✓ 部署完成。访问:${URL:-http://<服务器域名或IP>/}"
echo "  健康检查:curl -sk ${URL:-http://<域名>/}api/v1/health"
