# Rizline 谱面渲染器

> 免责声明：本项目仅用于学习与技术交流，所有相关知识产权均属于南京鸽游公司。
>
> 说明：本项目在开发过程中借助了 AI 辅助。

一个基于 Go 的 Rizline 音游谱面预览图生成工具。

## 功能特性

- 将 Rizline 的 JSON 谱面渲染为 PNG 图片
- 支持音符类型：Tap、Drag、Hold
- 支持带缓动插值的判定线渲染
- 支持主题颜色与 ChallengeTime 过渡效果
- 支持多列预览输出与基础抗锯齿（2x SSAA）

## 安装

```bash
cd riz-chart-renderer
go mod download
go build
```

## 使用方法

### 渲染单个谱面

```bash
riz-chart-renderer -chart charts/BurstyLollipop_EZ.json -output output/ez_preview.png
```

### 自定义列宽/列高/缩放

```bash
riz-chart-renderer -chart charts/ULTRASYNERGYMATRIX_IN.json -output output/in_preview.png -colwidth 420 -colheight 3600 -scale 0.7
```

### 参数说明（与当前代码一致）

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `-chart` | 谱面 JSON 文件路径（必填） | `""` |
| `-output` | 输出 PNG 文件路径 | `output/{谱面名}_preview.png` |
| `-colwidth` | 每列宽度（像素，0 表示自动） | `0` |
| `-colheight` | 每列高度（像素，0 表示自动） | `0` |
| `-scale` | 音符缩放（0 表示自动） | `0` |

> 说明：当 `-colwidth/-colheight/-scale` 为 `0` 时，会使用渲染器内部默认配置（例如列宽 400、列高 4000、音符缩放 0.6）。

## 批量渲染

当前可通过 `生成预览图.bat` 进行交互式批量渲染（菜单模式）。

## 音符类型（与代码一致）

- **Type 0**：Tap
- **Type 1**：Drag
- **Type 2**：Hold

## 缓动类型（easeType）

- **0**：Linear
- **1**：In Quad
- **2**：Out Quad
- **3**：InOut Quad
- **4**：In Cubic
- **5**：Out Cubic
- **6**：InOut Cubic
- **7**：In Quart
- **8**：Out Quart
- **9**：InOut Quart
- **10**：In Quint
- **11**：Out Quint
- **12**：InOut Quint
- **13**：Zero
- **14**：One
- **15**：In Circle
- **16**：Out Circle
- **17**：Sin
- **18**：Cos
