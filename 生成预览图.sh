#!/usr/bin/env bash
set -euo pipefail

echo "========================================"
echo "  Rizline 谱面预览图生成器"
echo "========================================"
echo

BIN=""
if [[ -f "./riz-chart-renderer" ]]; then
  BIN="./riz-chart-renderer"
elif [[ -f "./riz-chart-renderer.exe" ]]; then
  BIN="./riz-chart-renderer.exe"
else
  echo "正在编译..."
  go build
  if [[ -f "./riz-chart-renderer" ]]; then
    BIN="./riz-chart-renderer"
  elif [[ -f "./riz-chart-renderer.exe" ]]; then
    BIN="./riz-chart-renderer.exe"
  else
    echo "编译失败：未找到可执行文件。"
    exit 1
  fi
  echo
fi

if [[ ! -d "charts" ]]; then
  echo "错误：找不到 charts 目录！"
  exit 1
fi

mkdir -p output

echo
echo "选择渲染模式："
echo "1. 渲染单个谱面"
echo "2. 批量渲染所有谱面"
echo "3. 自定义参数渲染"
echo "4. 退出"
echo
read -r -p "请选择 (1-4): " mode

case "$mode" in
  1)
    echo
    echo "可用的谱面文件："
    found=0
    for f in charts/*.json; do
      [[ -e "$f" ]] || continue
      basename "$f"
      found=1
    done
    if [[ "$found" -eq 0 ]]; then
      echo "未找到任何谱面文件。"
      exit 1
    fi
    echo
    read -r -p "请输入谱面文件名: " CHART_FILE

    if [[ ! -f "charts/$CHART_FILE" ]]; then
      echo "文件不存在！"
      exit 1
    fi

    read -r -p "请输入输出文件名（留空使用默认）: " OUTPUT_FILE

    if [[ -z "$OUTPUT_FILE" ]]; then
      "$BIN" -chart "charts/$CHART_FILE" -output "output/${CHART_FILE%.json}_preview.png"
    else
      "$BIN" -chart "charts/$CHART_FILE" -output "output/$OUTPUT_FILE"
    fi

    echo
    echo "完成！文件保存在 output 目录"
    ;;

  2)
    echo
    echo "正在批量渲染所有谱面..."
    echo

    rm -f output/*_new.png

    found=0
    for f in charts/*.json; do
      [[ -e "$f" ]] || continue
      found=1
      name="$(basename "$f")"
      stem="${name%.json}"
      echo "正在渲染: $name"
      if "$BIN" -chart "$f" -output "output/${stem}_new.png"; then
        echo "  ✓ 完成"
      else
        echo "  ✗ 渲染失败"
      fi
      echo
    done

    if [[ "$found" -eq 0 ]]; then
      echo "未找到任何谱面文件。"
      exit 1
    fi

    echo
    echo "========================================"
    echo "  批量渲染完成！"
    echo "========================================"
    echo
    echo "输出位置: output/"
    ;;

  3)
    echo
    echo "自定义参数渲染"
    echo

    read -r -p "请输入谱面文件名: " CHART_FILE

    if [[ ! -f "charts/$CHART_FILE" ]]; then
      echo "文件不存在！"
      exit 1
    fi

    read -r -p "请输入输出文件名（不含.png）: " OUTPUT_FILE
    read -r -p "列宽度 (默认400，直接回车): " COL_WIDTH
    read -r -p "列高度 (默认4000，直接回车): " COL_HEIGHT
    read -r -p "音符缩放 (默认0.6，直接回车): " NOTE_SCALE

    CMD=("$BIN" -chart "charts/$CHART_FILE" -output "output/${OUTPUT_FILE}.png")

    if [[ -n "$COL_WIDTH" ]]; then CMD+=( -colwidth "$COL_WIDTH" ); fi
    if [[ -n "$COL_HEIGHT" ]]; then CMD+=( -colheight "$COL_HEIGHT" ); fi
    if [[ -n "$NOTE_SCALE" ]]; then CMD+=( -scale "$NOTE_SCALE" ); fi

    echo
    echo "执行命令: ${CMD[*]}"
    echo

    "${CMD[@]}"

    echo
    echo "完成！"
    ;;

  4)
    exit 0
    ;;

  *)
    echo "无效选择。"
    exit 1
    ;;
esac
