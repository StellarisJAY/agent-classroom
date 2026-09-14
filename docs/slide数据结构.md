# Slide 数据结构

> 版本 v1.0（已确认）

## 1. 概述

Slide 环节的内容拆分为两部分，分列存储、分阶段生成：

| 存储列 | 内容 | 生成阶段 |
|---|---|---|
| `section.content` (jsonb) | 画布属性 + 页面元素（视觉内容）| 阶段一：生成元素 |
| `section.steps` (jsonb) | 讲解步骤（旁白 + 高亮/划线/框选动作）| 阶段二：参考元素生成讲解 |

- 元素采用**绝对坐标定位**（相对画布），前端等比缩放渲染。
- 元素**一次性全部显示**，步骤仅做强调动作，不控制渐进出现。
- 步骤动作支持**多动作数组**（可同时高亮多个元素）。
- `image` 元素在阶段一由模型产出 `prompt`（此时 `src` 为空），随后由**图片生成环节**调用文生图模型生成图片、写入对象存储并回填 `src`；生成失败的元素直接剔除，不阻断流程。

## 2. `content` — 视觉内容

```json
{
  "width": 1280,
  "height": 720,
  "background": "#ffffff",
  "accent": "#14b8a6",
  "elements": [ ... ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `width` | number | 是 | 画布宽 px，推荐 1280 |
| `height` | number | 是 | 画布高 px，推荐 720（16:9）|
| `background` | hex | 是 | 背景色 |
| `accent` | hex | 是 | 强调色（高亮/划线/框选默认色、形状默认描边）|
| `elements` | array | 是 | 页面元素，有序，后者覆盖前者（天然 z 序）|

## 3. 页面元素（`elements[]`）

公共字段：`id`（slide 内唯一字符串，供动作引用）、`x` / `y`（左上角坐标，相对画布）。

### 3.1 text

```json
{ "id": "e1", "type": "text", "x": 80, "y": 60, "width": 1120,
  "content": "数组是同类型元素的集合",
  "style": { "fontSize": 28, "align": "left", "bold": false, "color": "#334155" } }
```

| 字段 | 说明 |
|---|---|
| `content` | 纯文本，支持 `\n` 换行（不做内联 markdown，粗体/斜体走 style）|
| `width` | 换行宽度 px；`height` 自动 |
| `style.fontSize` | 字号 px |
| `style.align` | `left` \| `center` \| `right` |
| `style.bold` | 加粗 |
| `style.color` | 文字色 hex（可选）|

### 3.2 formula

```json
{ "id": "e2", "type": "formula", "x": 80, "y": 220,
  "content": "\\int_a^b f(x)\\,dx", "fontSize": 32 }
```

| 字段 | 说明 |
|---|---|
| `content` | KaTeX 源码字符串 |
| `fontSize` | 字号 px |

### 3.3 shape

```json
{ "id": "e3", "type": "shape", "x": 700, "y": 320, "width": 160, "height": 48,
  "shape": "rect", "fill": "#e6fffa", "stroke": "#14b8a6", "label": "arr[0]" }
```

| 字段 | 说明 |
|---|---|
| `shape` | `rect` \| `circle` \| `line` \| `arrow` |
| `width` / `height` | 尺寸 px（必填）|
| `fill` | 填充色 hex |
| `stroke` | 描边色 hex |
| `label` | 图形内文字（可选）|

### 3.4 list

```json
{ "id": "e4", "type": "list", "x": 80, "y": 320, "width": 500,
  "ordered": true, "items": ["声明", "初始化", "访问"], "fontSize": 26 }
```

| 字段 | 说明 |
|---|---|
| `ordered` | 有序 `true` / 无序 `false` |
| `items` | `string[]`，纯文本条目 |
| `width` | 列表宽 px |
| `fontSize` | 字号 px |

### 3.5 image

```json
{ "id": "e5", "type": "image", "x": 700, "y": 160, "width": 480, "height": 360,
  "prompt": "A labeled diagram of a one-dimensional array in memory", "src": "/uploads/courses/.../e5.png" }
```

| 字段 | 说明 |
|---|---|
| `prompt` | 文生图提示词，由阶段一生成；建议英文、描述性、避免要求画面内出现精确文字 |
| `src` | 图片可访问 URL。阶段一留空，图片生成环节回填；生成失败的元素会被剔除 |
| `width` / `height` | 图片显示尺寸 px（必填，决定生成尺寸的宽高比）|

### 3.6 mermaid

```json
{ "id": "e6", "type": "mermaid", "x": 640, "y": 200, "width": 560,
  "content": "flowchart TD\n    A[开始] --> B{判断}\n    B -->|是| C[处理] --> D[结束]", "fontSize": 16 }
```

| 字段 | 说明 |
|---|---|
| `content` | mermaid 流程图源码（`flowchart TD` / `flowchart LR` 等）；换行以 `\n` 转义写入 JSON 字符串 |
| `width` | 流程图显示宽度 px（必填，SVG 宽度 100% 拉伸、高度按内容比例自适应）|
| `fontSize` | 图内文字字号 px（默认 16）|

用于呈现算法流程、操作步骤、状态流转等过程性知识（节点带箭头连接）；优先于多个 shape + arrow 手工拼图。渲染由前端引入 mermaid 库异步完成（懒加载），源码渲染失败时该元素不显示、不影响其余内容。

### 3.7 chart

```json
{ "id": "e7", "type": "chart", "x": 640, "y": 200, "width": 560, "height": 380,
  "chart": "bar", "title": "各季度销量对比",
  "categories": ["Q1", "Q2", "Q3", "Q4"],
  "series": [
    { "name": "线上", "values": [120, 200, 150, 180] },
    { "name": "线下", "values": [60, 90, 70, 110] }
  ] }
```

| 字段 | 说明 |
|---|---|
| `chart` | `bar`（数量对比）\| `line`（趋势）\| `pie`（占比） |
| `title` | 图表标题（可选，前端渲染，不必另配 text 元素）|
| `categories` | `string[]` 类目名；bar/line 作横轴类目，pie 作扇区名 |
| `series` | `{ "name": 系列名, "values": number[] }[]`；每个 `values` 长度必须与 `categories` 相等；pie 固定单系列 |
| `width` / `height` | 图表显示尺寸 px（必填，缺省 560×360）|

用于呈现可量化的数据对比 / 趋势 / 占比。这是封闭的"宽表"数据结构——模型只产出枚举类型与数据，坐标轴、图例、配色（由 `accent` 派生的色板）等全部样式由前端适配器确定性翻译为 ECharts 配置（按需懒加载 + SVG 渲染），模型不直接产出图表库配置。渲染失败时该元素不显示、不影响其余内容。校验时数据长度与类目不对齐的系列会被剔除，pie 多余系列被丢弃。

## 4. `steps` — 讲解步骤

```json
[
  { "text": "首先，数组是同类型元素的集合。",
    "actions": [{ "type": "highlight", "targetElementId": "e1" }] },
  { "text": "重点看第一个元素。",
    "actions": [{ "type": "box", "targetElementId": "e3" }] }
]
```

| 字段 | 说明 |
|---|---|
| `text` | 讲解旁白（底部老师栏逐字显示 + 可选 TTS）|
| `actions` | 动作数组，可为空（纯旁白）|

动作（`actions[]`）：

| 字段 | 说明 |
|---|---|
| `type` | `underline` \| `highlight` \| `box` \| `laser` \| `draw` \| `clearBoard` |
| `targetElementId` | 引用 `elements[].id`（underline/highlight/box 必填；laser 可选） |
| `x` / `y` | 仅 laser：画布系绝对坐标（与 `targetElementId` 至少其一）|
| `drawing` | 仅 `draw`：笔画数据，见下 |

> 白板的显示视图（纯幻灯片 / 半透明遮罩 / 不透明白板）由**用户手动切换**，不在动作编排中；模型只产出绘画、擦拭与激光动作。

### 4.1 laser —— 激光指示（瞬时）

```json
{ "type": "laser", "targetElementId": "e3" }
{ "type": "laser", "x": 640, "y": 300 }
```

仅在**当前步骤**内显示（红点 + 淡出动效），切换步骤即消失，不进入笔画序列。元素引用与坐标至少其一。

### 4.2 draw —— 白板绘画

```json
{ "type": "draw", "drawing": {
    "kind": "pen", "size": "medium", "color": "#ef4444",
    "points": [[100, 200], [150, 240], [220, 260]]
} }
```

| drawing 字段 | 类型 | 说明 |
|---|---|---|
| `kind` | 枚举 | `pen`（随手笔迹，points ≤64 点）\| `line` \| `arrow`（均恰好 2 点）\| `rect` \| `circle`（需包围盒 x/y/width/height）\| `text`（content 必填）|
| `size` | 枚举 | `thin` \| `medium` \| `thick`（缺省 medium，映射线宽 2/4/6）|
| `color` | hex | 可选，缺省画布 `accent` |
| `points` | `[x,y][]` | 画布系绝对坐标，服务端 clamp 进画布 |
| `x/y/width/height` | number | rect/circle 包围盒 |
| `content` / `fontSize` | string/int | text 内容与字号（缺省 24）|

坐标与 slide 画布（默认 1280×720）同系，故跨环节的笔画可直接对齐。

### 4.3 clearBoard —— 清空白板

```json
{ "type": "clearBoard" }
```

白板笔画**跨环节累积**；`clearBoard` 在重放序列中执行到时清空全部已累积笔画，由生成模型自主决定擦除时机（开启新主题、前序板书不再需要时）。

### 4.4 笔画回放模型（关键约定）

白板内容是**位置的纯派生量**：取「全局序号 `sectionIndex*1000 + stepIndex` ≤ 当前位置」的全部 draw 动作按序重放、遇 `clearBoard` 先清空重算。因此步骤回退时笔画自然收缩、前进/跳转整画布重绘而**不会重复作画**；切换到下一环节不清空画布，笔画跨环节延续，由 `clearBoard` 控制擦除。显示视图（纯幻灯片 / 遮罩 / 白板）为用户手动选择的会话级状态，不影响笔画序列。

## 5. 完整示例

```json
{
  "content": {
    "width": 1280,
    "height": 720,
    "background": "#ffffff",
    "accent": "#14b8a6",
    "elements": [
      { "id": "e1", "type": "text", "x": 80, "y": 60, "width": 1120,
        "content": "一维数组的声明与初始化",
        "style": { "fontSize": 40, "align": "left", "bold": true, "color": "#0f172a" } },
      { "id": "e2", "type": "text", "x": 80, "y": 140, "width": 800,
        "content": "数组是同类型元素的有序集合",
        "style": { "fontSize": 28, "align": "left", "color": "#334155" } },
      { "id": "e3", "type": "formula", "x": 80, "y": 220,
        "content": "\\text{int arr[5] = \\{1,2,3,4,5\\};}", "fontSize": 32 },
      { "id": "e4", "type": "list", "x": 80, "y": 320, "width": 500,
        "ordered": true, "items": ["声明", "初始化", "访问"], "fontSize": 26 },
      { "id": "e5", "type": "shape", "x": 700, "y": 320, "width": 160, "height": 48,
        "shape": "rect", "fill": "#e6fffa", "stroke": "#14b8a6", "label": "arr[0]" }
    ]
  },
  "steps": [
    { "text": "首先，数组是同类型元素的集合。",
      "actions": [{ "type": "highlight", "targetElementId": "e2" }] },
    { "text": "声明语法如下。",
      "actions": [{ "type": "underline", "targetElementId": "e3" }] },
    { "text": "重点看第一个元素。",
      "actions": [{ "type": "box", "targetElementId": "e5" }] }
  ]
}
```

## 6. 关键约定

- **无独立 `title` 字段**：slide 视觉标题即一个 `fontSize` 较大的 `text` 元素；环节标题已存于 `section.title`，二者不重复。
- **画布缩放**：前端按 `width/height` 比例 `object-fit: contain` 缩放，坐标换算为百分比渲染，支持响应式。
- **z 序**：`elements` 数组顺序即叠放顺序，靠后者在上。
- **两阶段生成**：先产出 `content`（元素），智能体再参考元素产出 `steps`（讲解），稳定性更高。
- **图片生成**：阶段一产出的 `image` 元素（含 `prompt`）在进入阶段二前完成图片生成与 `src` 回填，故阶段二看到的元素清单已含可用图片。
