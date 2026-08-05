# CampusWall 校园墙 — 设计需求与功能分析文档

> 生成时间：2026-07-18 | 基于仓库 `qingshanjiluo/campus-wall` 全量代码与参考资料

---

## 一、项目定位

**校园墙 (CampusWall)** 是一个面向大学生的校园社区平台，以"子站"为核心组织形式，融合了 Galgame（视觉小说）的交互美学。项目灵感来源于 Paper2Galgame（将论文转化为视觉小说对话的 AI 应用），将这种日系二次元风格移植到了校园社交场景。

**一句话定义**：一个带有 Galgame 审美气质的校园匿名/半匿名社区。

---

## 二、技术栈

| 层级 | 技术选型 |
|------|----------|
| **后端框架** | Flask 2.3.3 (Python) |
| **数据库** | SQLite (`instance/campushub.db`) |
| **认证** | JWT (PyJWT 2.13.0) + bcrypt 5.0.0 |
| **跨域** | flask-cors 4.0.1 |
| **前端模板** | Jinja2 (Flask templates) |
| **CSS 动画** | GSAP 3.12.2 + ScrollTrigger / anime.js |
| **字体** | Google Fonts (ZCOOL KuaiLe, Ma Shan Zheng, Noto Sans SC, Nunito, Press Start 2P) |
| **参考前端** | React 19 + Tailwind CSS + Vite (paper2galgame 参考项目) |

---

## 三、核心功能模块

### 3.1 用户认证系统

| 功能 | 路由 | 状态 |
|------|------|------|
| 用户注册 | `POST /api/auth/register` | ✅ 已实现 |
| 用户登录 | `POST /api/auth/login` | ✅ 已实现 |
| 获取当前用户 | `GET /api/auth/me` | ✅ 已实现 |
| 用户登出 | `POST /api/auth/logout` | ✅ 已实现（客户端删 token） |

**注册规则**：
- 用户名 ≥ 3 字符，唯一
- 邮箱需含 `@`，唯一
- 密码 ≥ 6 字符，bcrypt 加密存储

**登录特性**：支持用户名或邮箱登录

**Token 机制**：JWT，有效期 7 天，Bearer 方式传递

### 3.2 子站系统 (Sub-Stations)

子站是平台的核心组织单元，类似贴吧的"吧"或 Discord 的"服务器"。

| 功能 | 路由 | 状态 |
|------|------|------|
| 获取子站列表 | `GET /api/stations` | ✅ 已实现 |
| 搜索子站 | `GET /api/search?q=` | ✅ 已实现 |
| 创建子站 | — | ❌ 未实现（前端按钮显示"开发中"） |

**子站数据结构**：
```
id, name, description, cover, tags (JSON), user_count, post_count, owner_id, created_at
```

**搜索支持**：按名称和描述模糊匹配，按用户数降序排列

### 3.3 帖子系统 (Posts)

| 功能 | 路由 | 状态 |
|------|------|------|
| 获取帖子列表 | `GET /api/posts?limit=N` | ✅ 已实现 |
| 发帖 | — | ❌ 未实现 |
| 点赞/评论 | — | ❌ 未实现 |

**帖子数据结构**：
```
id, title, content, author, author_id, views, likes, comments, station_id, created_at
```

帖子列表会关联子站名称，内容截断至 100 字符。

### 3.4 社交功能（数据库已建表，接口未实现）

| 功能 | 表名 | 状态 |
|------|------|------|
| 用户关注 | `follows` | 🔲 表已创建，无接口 |
| 通知系统 | `notifications` | 🔲 表已创建，无接口 |
| 用户资料编辑 | `users` (bio, avatar) | 🔲 函数已有 (`update_user_profile`)，无路由 |

### 3.5 用户系统

| 字段 | 说明 |
|------|------|
| `avatar` | 默认 `/static/images/default-avatar.png` |
| `bio` | 个人简介 |
| `role` | 角色，默认 `user`（推测有 admin） |
| `created_at` / `updated_at` | 时间戳 |

---

## 四、UI/UX 设计风格分析

### 4.1 设计语言：Galgame 美学 × 校园温暖感

项目明确追求 **Galgame（视觉小说）** 的交互风格，体现在：

#### 色彩体系
```css
--cream: #FFF8F0;         /* 温暖底色，"阳光晒过的纸张" */
--sunset-pink: #FFB5BA;   /* 落日粉 */
--sunset-orange: #FFD6A5; /* 落日橙 */
--sunset-yellow: #FDFFB6; /* 落日黄 */
--soft-lavender: #E2D5F5; /* 柔和薰衣草紫 */
--soft-blue: #C9E4F5;     /* 柔和蓝 */
--soft-mint: #B5EAD7;     /* 薄荷绿 */
```
整体色调：**暖色为主、低饱和度、柔和渐变**，营造"被阳光晒过的纸张"质感。

#### 字体选择
| 字体 | 用途 | 风格 |
|------|------|------|
| ZCOOL KuaiLe | 标题 | 圆润可爱，手绘感 |
| Ma Shan Zheng | 副标题/引言 | 手写书法感 |
| Noto Sans SC | 正文 | 清晰易读 |
| Nunito | 数字/英文 | 圆润现代 |
| Press Start 2P | 像素标签 | 复古游戏风 |

#### 视觉特效
- **毛玻璃导航栏**：`backdrop-filter: blur(20px)` + 半透明背景
- **多层视差背景**：浮动光晕 (`bg-glow`) + 水彩斑点 + 纸张纹理
- **对话气泡式帖子**：SVG 手绘引线，模拟 Galgame 对话框
- **加载动画**：打字机效果逐字浮现，模拟视觉小说开场
- **滚动动画**：GSAP ScrollTrigger，元素进入视口时淡入上浮
- **鼠标视差**：角色和气泡跟随鼠标微移

### 4.2 两种 UI 风格原型

仓库中包含两套完整的设计原型：

#### 风格 A：Galgame 章节式（kimi.txt）
- 纯 CSS 绘制角色立绘（头发、呆毛、蝴蝶结、腮红）
- 打字机效果的对话气泡
- 手绘涂鸦背景层（星星、云朵、虚线、圆点）
- 章节式滚动："第一章·邂逅" → "第二章·探索" → "第三章·交流" → "终章"
- 子站卡片使用不规则 blob 形状 (`border-radius: 60% 40%...`)
- 使用 anime.js 驱动动画

#### 风格 B：温暖卡片式（kimi2.txt / 当前实现）
- 毛玻璃卡片 + 圆角设计
- 三栏布局：侧边导航 | 主内容 | 个人信息
- 对话气泡式帖子卡片（SVG 引线）
- 背景光晕浮动动画
- 使用 GSAP + ScrollTrigger
- **当前 Flask 模板采用此风格**

### 4.3 参考设计（deepseek_html）
- Tailwind CSS 驱动
- 暗色/亮色主题切换
- 赛博朋克主题变体
- 更现代的卡片网格布局

---

## 五、页面结构

### 5.1 首页 (`/`)

| 区域 | 内容 |
|------|------|
| **加载动画** | 打字机效果："正在打开社团活动室的门..." → "阳光透过百叶窗洒了进来..." → "欢迎来到校园墙 ✨" |
| **Hero 区** | 大标题"连接校园 分享青春" + CTA 按钮（创建子站/浏览子站） |
| **子站市场** | 搜索框 + 分类标签（热门/最新/推荐/学习/游戏/音乐）+ 子站卡片网格 |
| **最新动态** | 帖子列表（标题/内容预览/作者/时间/互动数据） |
| **页脚** | 版权信息 + 关于/协议/隐私链接 |

### 5.2 子站详情页
- 路由：未实现
- 前端已有点击事件占位 (`showToast('进入 xxx')`)

### 5.3 个人中心
- 参考设计（kimi2.txt）中已设计完整布局
- 三栏：侧边导航 | 我的子站 | 个人信息
- 未接入后端

---

## 六、数据库设计

### 已创建的表

| 表名 | 字段数 | 说明 |
|------|--------|------|
| `users` | 9 | 用户表（id, username, email, password_hash, avatar, bio, role, created_at, updated_at） |
| `sub_stations` | 9+ | 子站表（原有 + owner_id 扩展） |
| `posts` | 10+ | 帖子表（原有 + author_id 扩展） |
| `follows` | 4 | 关注关系表（follower_id, following_id, created_at） |
| `notifications` | 6 | 通知表（user_id, type, content, is_read, link, created_at） |

### 数据库初始化逻辑
- `init_db()` 创建 users/follows/notifications 表
- `ALTER TABLE` 为 sub_stations/posts 追加外键字段（兼容旧数据）
- 使用 `sqlite3.Row` 实现字典式访问

---

## 七、已实现 vs 未实现功能清单

### ✅ 已实现
- [x] 用户注册/登录/登出
- [x] JWT 认证中间件 (`token_required` 装饰器)
- [x] 子站列表查询 API
- [x] 子站搜索 API
- [x] 帖子列表查询 API（带分页 limit）
- [x] 首页完整 UI（加载动画、Hero、子站市场、帖子列表）
- [x] 毛玻璃导航栏 + GSAP 滚动动画
- [x] 响应式布局（移动端适配）

### ❌ 未实现
- [ ] 创建子站
- [ ] 发帖/删帖/编辑帖子
- [ ] 点赞/评论/分享
- [ ] 子站详情页
- [ ] 个人中心页面
- [ ] 用户资料编辑（头像、简介）
- [ ] 关注/取关
- [ ] 通知系统
- [ ] 管理员权限
- [ ] 图片上传
- [ ] 搜索帖子（当前只搜子站）
- [ ] 帖子分类/标签筛选
- [ ] 分页加载/无限滚动
- [ ] 深色模式
- [ ] 部署配置（生产环境密钥、数据库迁移）

---

## 八、参考资源仓库分析

仓库中包含两个参考子项目：

### 8.1 `references/paper2galgame/`
- **Paper2Galgame**：将学术论文 PDF 转化为 Galgame 对话体验的 AI 应用
- 技术栈：React 19 + Tailwind CSS + Google Gemini API
- 核心功能：PDF 上传 → AI 角色扮演对话 → 视觉小说式呈现
- **对本项目的启发**：Galgame 交互美学（立绘、对话框、打字机效果、章节式叙事）

### 8.2 `references/paper2gal_scrape/` + `paper2gal_src/`
- Paper2Galgame 的线上版本抓包文件和源码
- 已编译的生产版本（JS/CSS minified）

### 8.3 `参考/` 目录
- 设计参考视频字幕（UI/UX 技巧、网站动画、CDN 介绍等）
- Kimi 生成的两版完整 HTML 原型
- DeepSeek 生成的 HTML 原型

---

## 九、安全注意事项

1. **硬编码密钥**：`app.secret_key` 和 `JWT_SECRET` 使用默认值，生产环境必须更换
2. **API Key 泄露风险**：参考项目中 Google Gemini API Key 前端硬编码
3. **SQL 注入**：当前使用参数化查询，基本安全
4. **密码存储**：使用 bcrypt 加密，符合最佳实践
5. **CORS 配置**：`supports_credentials=True`，需确认生产环境域名白名单

---

## 十、总结

CampusWall 是一个**设计驱动**的校园社区项目，目前处于**早期原型阶段**：

- **后端**：核心认证和数据查询已就绪，但业务逻辑（CRUD）大部分未实现
- **前端**：视觉设计完成度很高，Galgame 美学贯穿始终，但功能交互多为占位
- **设计**：两套完整 UI 原型 + 一个参考项目，风格统一，细节丰富
- **下一步**：优先实现子站创建、发帖、个人中心三大核心流程，然后迭代社交功能
