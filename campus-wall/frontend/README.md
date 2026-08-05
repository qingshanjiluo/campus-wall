# 校园墙 · Cloudflare Pages 前端（静态版）

本目录是校园墙的 **纯静态前端**，专为 Cloudflare Pages 部署设计。
后端 Flask API 部署在 PythonAnywhere 等平台，前端通过 **Pages Functions** 反向代理访问后端。

## 目录结构

```
frontend/
├── pages/                    # 各页面静态 HTML（无 Jinja2）
│   ├── index.html            # 首页
│   ├── post.html             # 帖子详情（URL 解析 /post/<id>）
│   ├── station.html          # 子站页（/station/<id>）
│   ├── profile.html          # 个人主页（/profile/<id>）
│   ├── admin.html            # 管理后台
│   └── ...                   # 其余页面
├── static/                   # CSS / JS / 图片（路径与 Flask 版一致）
│   ├── css/style.css
│   ├── js/api.js             # API 层（相对路径 /api/...）
│   ├── js/common.js          # 公共框架注入（导航/看板娘/模态框/toast）
│   ├── js/app.js             # 主逻辑（用户状态/表单/交互）
│   └── js/animations.js
├── functions/                # Cloudflare Pages Functions
│   └── api/[[path]].ts       # /api/* → 后端代理
│   └── static/uploads/[[path]].ts  # /static/uploads/* → 后端图片代理
├── _redirects                # 动态路由 → 静态页 重写
└── _headers                  # 安全头 + 缓存
```

## 部署步骤（Cloudflare Pages）

1. 将本项目推到 GitHub（`frontend/` 目录已包含）
2. Cloudflare 控制台 → **Workers & Pages** → **Create** → **Pages** → **Connect to Git**
3. 选择 `campus-wall` 仓库
4. 构建配置：
   - **Framework preset**: `None`
   - **Build command**: `(留空)`
   - **Build output directory**: `frontend`
5. 设置环境变量：
   - `API_BASE=https://你的用户名.pythonanywhere.com`（你的后端地址）
6. 保存并 **Deploy**

部署后访问 `xxx.pages.dev` 即可使用。

## 绑定自定义域名

Pages 项目 → **Custom domains** → **Set up a custom domain** → 填 `campuswall.example.com`

## 环境变量说明

| 变量 | 说明 |
|------|------|
| `API_BASE` | 后端 API 根地址。设置后 `/api/*` 和 `/static/uploads/*` 请求被代理到该地址 |

> 若不设置 `API_BASE`，前端会请求同源的 `/api/*`（用于本地联调或前端后端同源部署）。
