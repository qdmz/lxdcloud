#!/usr/bin/env bash
# ============================================================
# LXD Cloud 版本发布脚本
#   递增版本号 → 同步 VERSION / 前端 ?v= / 后端 main.Version → 更新 CHANGELOG
#   → 提交 → 打 tag vX.Y.Z → 推送分支与 tag
#
# 用法：bash scripts/release.sh [patch|minor|major]   （默认 patch）
#   环境变量：
#     NO_PUSH=1   只在本地提交和打 tag，不推送
#     REMOTE=xxx  推送的远程名（默认 origin）
# ============================================================
set -euo pipefail
cd "$(dirname "$0")/.."

part="${1:-patch}"
case "$part" in patch|minor|major) ;; *) echo "用法: $0 [patch|minor|major]"; exit 1 ;; esac
REMOTE="${REMOTE:-origin}"

cur="$(tr -d ' \n' < VERSION 2>/dev/null || echo 0.0.0)"
latest_tag="$(git tag -l 'v*' --sort=-v:refname | head -n1 | sed 's/^v//')"
# 以 VERSION 与最新 tag 中较大的为基准（兼容 v1.0 这类两段式 tag）
norm() { local v="$1"; IFS=. read -r a b c <<<"$v"; echo "${a:-0}.${b:-0}.${c:-0}"; }
cur="$(norm "$cur")"; latest_tag="$(norm "${latest_tag:-0.0.0}")"
base="$(printf '%s\n%s\n' "$cur" "$latest_tag" | sort -V | tail -n1)"
IFS=. read -r MA MI PA <<<"$base"

# 若 VERSION 尚未打过 tag（首次发布当前版本），直接使用该版本；否则按参数递增
if git rev-parse -q --verify "refs/tags/v$base" >/dev/null; then
  case "$part" in
    major) MA=$((MA+1)); MI=0; PA=0 ;;
    minor) MI=$((MI+1)); PA=0 ;;
    *)     PA=$((PA+1)) ;;
  esac
fi
new="$MA.$MI.$PA"
echo "当前: v$cur  最新 tag: v$latest_tag  →  发布: v$new"

if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
  echo "提示: 工作区有未提交的改动，将一并提交到 release 提交中"
fi

# 1) VERSION
echo "$new" > VERSION

# 2) 后端默认版本号（编译时也可用 -ldflags 覆盖）
sed -i -E "s/^var Version = \"[^\"]*\"/var Version = \"$new\"/" backend/cmd/lxdapi/main.go

# 3) 前端静态资源缓存版本号
find frontend -name '*.html' -print0 | xargs -0 sed -i -E "s/\?v=[0-9]+\.[0-9]+(\.[0-9]+)?\"/?v=$new\"/g"

# 4) README 版本徽章
sed -i -E "s#badge/version-[0-9]+\.[0-9]+\.[0-9]+-#badge/version-$new-#" README.md 2>/dev/null || true

# 5) CHANGELOG：顶部插入新版本段落（已存在则跳过）
if ! grep -q "^## v$new" CHANGELOG.md 2>/dev/null; then
  prev_tag="$(git tag -l 'v*' --sort=-v:refname | head -n1)"
  if [ -n "$prev_tag" ]; then
    notes="$(git log --no-merges --pretty='- %s' "$prev_tag"..HEAD | grep -v '^- release: ' || true)"
  else
    notes="$(git log --no-merges --pretty='- %s' | head -n 30)"
  fi
  [ -z "$notes" ] && notes="- 维护性更新"
  {
    echo "# 更新日志"
    echo
    echo "## v$new - $(date +%F)"
    echo
    echo "$notes"
    echo
    tail -n +2 CHANGELOG.md 2>/dev/null | sed '1{/^$/d}'
  } > CHANGELOG.md.tmp && mv CHANGELOG.md.tmp CHANGELOG.md
fi

git add -A
git commit -m "release: v$new" || echo "没有需要提交的改动"
git tag -a "v$new" -m "Release v$new"

if [ "${NO_PUSH:-0}" = "1" ]; then
  echo "已在本地发布 v$new（NO_PUSH=1，未推送）"
  exit 0
fi
git push "$REMOTE" HEAD
git push "$REMOTE" "v$new"
echo "已发布 v$new"
