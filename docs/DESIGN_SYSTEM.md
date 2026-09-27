# 设计系统 · 纸上校园（Paper Campus）

> 校园墙的视觉与体验规范。任何新增页面/组件都应遵循本文件，以保持同一套语言。

## 1. 设计主张

**纸上校园 · 手写温度**：整个产品是「一叠被同学写过、贴过、折过角的纸」。
不是玻璃拟态、不是渐变卡片堆砌——是纸面、墨字、胶带、折角，以及页边随手写的批注。

一句话检验标准：**新做的东西看起来像不是 AI 生成的**。若某个组件让人联想到「AI 落地页」，
它就不合格。

## 2. 色彩

| 角色 | Token | 说明 |
|---|---|---|
| 纸底 | `--cream` / `--bg-primary` | 暖米灰，全站底 |
| 纸面 | `--warm-white` / `--bg-secondary` | 卡片、导航的实体纸面 |
| 墨字 | `--text-dark` / `--text-primary` | 标题与正文（不用纯黑） |
| 次级 | `--text-secondary` | 说明文字 |
| 弱化 | `--text-muted` | 时间戳、辅助信息 |
| 强调填充 | `--accent-fill` / `--accent-on-fill` | 主按钮/激活标签；**白字对比 ≥4.5:1** |
| 马克笔 | `--yellow` | 章节标题高亮 |
| 品牌玫瑰 | `--pink-4` | 手写批注、点缀 |

**禁用**：蓝紫/粉紫/荧光渐变、无意义的多色渐变、纯黑纯白大色块。
柔和的粉彩双色仅允许用于「贴纸」类元素（子站图标、头像占位）。

暗色：所有纸面 token 由 `[data-theme="dark"]` 提供深色纸（暖黑褐），纸纹改为浅色横线
（黑板笔记本感）。4 套皮肤（glass/galgame/minimal/cyberpunk）各自覆盖 `--accent-fill` 对，
以保证暗色下填充上的文字对比仍达标。

## 3. 字体

| 用途 | 字体 | 规则 |
|---|---|---|
| 标题/按钮/Logo | `ZCOOL KuaiLe` | 只在「被强调」处用，不用于正文 |
| 正文/输入/副标题 | `Noto Sans SC` | 默认字体，保证可读 |
| 手写批注 | `Ma Shan Zheng` | **只留给批注、引用、空状态**；不用于常规副标题（会稀释） |
| 印刷式 kicker | JetBrains Mono | 小号大写字母 + 字距，用于「第 01 期」类标签 |

字体通过各页 `preconnect` + `media="print" onload="this.media='all'"` **非阻塞**加载；
禁止在 CSS 里用 `@import` 拉外部字体（会形成 HTML→CSS→字体CSS→字体文件 的渲染阻塞链）。

## 4. 形状与材质

- **圆角**：卡片/纸面用 `--radius`；按钮 12px；禁止无理由的大圆角。
- **纸面**：实体底色 + 笔记本横线纹理
  `repeating-linear-gradient(to bottom, transparent 0 27px, rgba(43,37,32,.035) 27px 28px)`。
- **顶部胶带**：卡片 `::before`，72×18px，`rotate(-2deg)`，虚线侧边（把卡「钉」在墙上）。
- **右上折角**：卡片 `::after`，`border-top` 三角，hover 变 `--pink-2`。
- **章节标题**：墨色 + 马克笔高亮（`linear-gradient(92deg, var(--yellow) 0%, rgba(253,255,182,.45) 92%)`，
  `background-size: 100% 9px`），**不用发光 text-shadow**。
- **分隔**：`.paper-tear` 撕纸孔线（径向渐变重复 + 虚线），替代渐变装饰线 + 漂浮图标。

## 5. 动效

- 只用 `transform` / `opacity`，时长 0.2–0.35s，缓动 `var(--bounce)`。
- **禁止**无限漂浮/闪烁装饰、光泽扫过按钮、整屏大 blur 动画。
- 保留的功能性动画：加载旋转器、打字机光标。
- 必须响应 `@media (prefers-reduced-motion: reduce)`（已全局降级）。
- 页面不可见时暂停背景动效（`animations.js` 的 `visibilitychange`）。

## 6. 可访问性（红线）

- 正文/按钮文字对比 ≥ 4.5:1；大字号 ≥ 3:1。改动颜色后**必须重跑对比度审计**。
- 键盘焦点可见：`:focus-visible { outline: 3px solid var(--accent-fill); outline-offset: 2px; }`。
- 语义化标签 + `aria-label`（导航、图标按钮）。
- 装饰性元素一律 `aria-hidden="true"` + `pointer-events: none`。

## 7. 文案

- 空状态 = 「这里是什么」+「为什么是空的」+「下一步做什么」。
- 语气：克制、有温度，像同学写在纸上的话；**不用「快来抢沙发」「开启您的」式吆喝**。
- 错误提示要给下一步（刷新 / 换个词），不要只写「加载失败」。
- 禁止 Emoji（`icons.js` 里的 emoji→Lucide 映射表只是内部兼容键，不直接渲染）。

## 8. 性能（红线）

- 首屏 FCP ≤ 2s（实测中位 1.54s）。
- 装饰层避免 `filter: blur()` 大面积使用；纹理用**小尺寸可平铺**瓦片（96px/2 octaves）。
- 第三方脚本按需加载（如 admin 的 chart.js 用 `loadChartJs()` 惰性注入），不放 `<head>` 同步加载。
- 静态资源走 Nginx 缓存与 gzip/brotli（见 `docs/SERVER_ARCHITECTURE.md`）。

## 9. 响应式

- 断点：900px / 768px / 480px。
- **两栏内联 grid 必须在 ≤900px 折叠为单栏，并给子项 `min-width: 0`**（否则右列撑开导致横向溢出）。
- 移动端横向溢出必须为 0（用 `work/mobile_audit.py` 逐页扫描验证）。

## 10. 验证工具（`work/`，未纳入版本库）

| 脚本 | 作用 |
|---|---|
| `tools/js_gate.py` | JS/壳层/编码 门禁（每轮必跑） |
| `tools/e2e_live.ps1` | 84 步全链路 E2E（每轮必跑） |
| `qa_pages.py` / `qa_mobile.py` | PIL 截图量化 QA：暖纸主导、蓝紫占比、横向溢出 |
| `dark_audit.py` | 全站暗色模式验收（28 页） |
| `mobile_audit.py` | 全站移动端横向溢出验收（28 页） |
| `a11y_audit.py` | WCAG 对比度审计（含皮肤 × 明暗） |
| `perf_audit2.py` | 首屏 FCP / 传输量 / 请求数（中位数口径） |
| `final_emoji_audit.py` | 渲染级 Emoji 审计（遍历可见文本节点） |
