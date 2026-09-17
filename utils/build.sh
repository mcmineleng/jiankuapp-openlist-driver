#!/bin/bash

set -euo pipefail

# ========== 可配置项 ==========
APP_ENTRIES=(
  "dir-json.go"
  "full-json.go"
)

LDFLAGS="-s -w"
BUILD_ROOT="./build"
NO_UPX=0
BUILD_ALL=0
FILTER=""

# ========== 参数解析 ==========
while [[ $# -gt 0 ]]; do
  case "$1" in
    all)
      BUILD_ALL=1
      shift
      ;;
    --noupx)
      NO_UPX=1
      shift
      ;;
    -h|--help)
      echo "用法: $0 [all|平台过滤] [--noupx]"
      echo ""
      echo "示例:"
      echo "  $0                     # 编译当前系统架构"
      echo "  $0 all                 # 编译所有平台"
      echo "  $0 linux               # 只编译 linux"
      echo "  $0 linux/amd64         # 只编译 linux/amd64"
      echo "  $0 all --noupx         # 所有平台，跳过 UPX"
      echo "  $0 --noupx             # 当前平台，跳过 UPX"
      exit 0
      ;;
    *)
      FILTER="$1"
      shift
      ;;
  esac
done

# ========== 平台定义 ==========
ALL_PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "linux/arm"
  "linux/loong64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
  "windows/arm64"
  "freebsd/amd64"
  "android/arm64"
  "android/amd64"
)

# ========== 平台选择 ==========
if [ "$BUILD_ALL" -eq 1 ]; then
  PLATFORMS=("${ALL_PLATFORMS[@]}")
elif [ -n "$FILTER" ]; then
  if [[ "$FILTER" == */* ]]; then
    PLATFORMS=()
    for p in "${ALL_PLATFORMS[@]}"; do
      [[ "$p" == "$FILTER" ]] && PLATFORMS+=("$p")
    done
  else
    PLATFORMS=()
    for p in "${ALL_PLATFORMS[@]}"; do
      [[ "$p" == "$FILTER"/* ]] && PLATFORMS+=("$p")
    done
  fi

  if [ ${#PLATFORMS[@]} -eq 0 ]; then
    echo "❌ 未找到匹配的平台: $FILTER"
    echo "可用平台:"
    printf "  %s\n" "${ALL_PLATFORMS[@]}"
    exit 1
  fi
else
  # ✅ 默认：当前系统架构
  CURRENT_PLATFORM="${GOOS:-$(go env GOOS)}/${GOARCH:-$(go env GOARCH)}"
  PLATFORMS=("$CURRENT_PLATFORM")
fi

# ========== 开始编译 ==========
TOTAL_PLATFORMS=${#PLATFORMS[@]}
TOTAL_APPS=${#APP_ENTRIES[@]}

echo "========================================="
echo "  交叉编译工具"
echo "  输出根目录: $BUILD_ROOT"
echo "  平台数: $TOTAL_PLATFORMS"
echo "  程序数: $TOTAL_APPS"
[ "$NO_UPX" -eq 1 ] && echo "  UPX: 已跳过"
[ "$BUILD_ALL" -eq 0 ] && [ -z "$FILTER" ] && \
  echo "  模式: 当前系统架构 ($(go env GOOS)/$(go env GOARCH))"
echo "========================================="
echo ""

for platform in "${PLATFORMS[@]}"; do
  IFS='/' read -r GOOS GOARCH <<< "$platform"

  PLATFORM_DIR="${BUILD_ROOT}/${GOOS}/${GOARCH}"
  mkdir -p "$PLATFORM_DIR"

  echo "📦 平台: $GOOS/$GOARCH"
  echo "   输出目录: $PLATFORM_DIR"
  echo ""

  for entry in "${APP_ENTRIES[@]}"; do
    APP_NAME="${entry%.go}"

    if [ "$GOOS" = "windows" ]; then
      output="${PLATFORM_DIR}/${APP_NAME}.exe"
    else
      output="${PLATFORM_DIR}/${APP_NAME}"
    fi

    echo "  🔨 编译: $entry → $output"

    CGO_ENABLED=0 \
    GOOS="$GOOS" \
    GOARCH="$GOARCH" \
      go build -v -trimpath -ldflags="$LDFLAGS" -o "$output" "$entry"

    # ---------- UPX ----------
    if [ "$NO_UPX" -eq 0 ] && command -v upx &>/dev/null; then
      case "$GOOS" in
        darwin|freebsd)
          : # 不支持
          ;;
        *)
          upx -q "$output" 2>/dev/null || true
          ;;
      esac
    fi

    ls -lh "$output"
    echo ""
  done

  echo "✅ $GOOS/$GOARCH 完成"
  echo ""
done

echo "========================================="
echo "  全部编译完成!"
echo "  输出目录: $BUILD_ROOT"
echo "========================================="
