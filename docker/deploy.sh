#!/usr/bin/env bash
# 从本地经 SSH 部署到服务器(凭据读自仓库根的 .env):
#   1) 远端 git pull 拉最新代码
#   2) scp 同步 config.yml / .cookies.json(gitignored,含密钥)
#   3) 远端执行 docker/start.sh:构建前端 + 起 conda 后端 + 起 nginx
#
# 用法:在仓库根执行 `bash docker/deploy.sh`
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$REPO_DIR/.env"

if ! command -v sshpass >/dev/null 2>&1; then
  echo "缺少 sshpass。macOS: brew install esolitos/ipa/sshpass"; exit 1
fi
if [[ ! -f "$ENV_FILE" ]]; then
  echo "缺少 $ENV_FILE(参考 docker/.env.example)"; exit 1
fi

# 解析 notes 格式的 .env:  键:值
get() {
  grep -iE "^${1}:" "$ENV_FILE" | head -1 \
    | sed -E 's/^[^:]+:[[:space:]]*//' | tr -d '\r' | sed 's/[[:space:]]*$//'
}

IP=$(get ip)
REMOTE_USER=$(get user)
PASS=$(get password)
CODE_DIR=$(get code_dir)
URL=$(get URL)

for v in IP REMOTE_USER PASS CODE_DIR; do
  if [[ -z "${!v:-}" ]]; then echo ".env 缺少字段:$(echo $v | tr 'A-Z' 'a-z')"; exit 1; fi
done

echo "→ 目标:${REMOTE_USER}@${IP}:${CODE_DIR}"
echo "  访问:${URL:-<.env 未设 URL>}"
SSH_OPTS=(-o StrictHostKeyChecking=accept-new)

echo "== [1/3] 远端 git pull =="
sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "
  set -e
  if [ -d '${CODE_DIR}/.git' ]; then
    cd '${CODE_DIR}' && git pull --ff-only
  else
    echo '远端 ${CODE_DIR} 不是 git 仓库。请先在服务器上执行:'
    echo \"  git clone <repo-url> ${CODE_DIR}\"
    exit 1
  fi
"

echo "== [2/3] 同步密钥配置 =="
for f in config.yml .cookies.json; do
  if [[ -f "$REPO_DIR/$f" ]]; then
    echo "  → $f"
    sshpass -p "$PASS" scp "${SSH_OPTS[@]}" "$REPO_DIR/$f" "${REMOTE_USER}@${IP}:${CODE_DIR}/${f}"
  else
    echo "  ⚠ 本地无 $f,跳过(后端启动需要 config.yml 与 .cookies.json)"
  fi
done

echo "== [3/3] 远端执行 start.sh =="
# login shell(-l)让 conda 初始化生效,start.sh 才能激活 dogs 环境
REMOTE_CMD="bash -lc 'cd \"${CODE_DIR}\" && bash docker/start.sh'"
sshpass -p "$PASS" ssh "${SSH_OPTS[@]}" "${REMOTE_USER}@${IP}" "${REMOTE_CMD}"

echo
echo "✓ 部署完成。访问:${URL:-http://<服务器IP或域名>/}"
