# 校园墙 CampusWall

Galgame 风格的校园社区平台 —— 让每个声音都被听见，连接校园，分享青春。

## 项目简介

校园墙是一个面向高校学生的社区交流平台，采用 **Galgame / 手绘暖色系** 设计语言，提供子站（兴趣圈子）、帖子、二手交易、恋爱情报、树洞爆料、签到打卡、积分商城、身份组等丰富的校园生活功能。

- **后端**：Flask 2.3 + SQLite + JWT + bcrypt（Python）
- **前端**：原生 HTML/CSS/JS 单页应用（Lucide 图标、深色模式、响应式）
- **部署**：前端可部署到 Cloudflare Pages，后端部署到 PythonAnywhere / VPS

## 功能一览

| 模块 | 说明 |
|------|------|
| 子站系统 | 创建/加入/退出子站、站长管理（置顶/删除帖子/转让/移除成员）、封面图、公告、公开/私密切换、分类层级 |
| 帖子系统 | 图文/链接/投票三种类型、图片上传、点赞、收藏、评论、匿名发布、编辑历史、举报、分享 |
| 交易广场 | 发布闲置、分类筛选、关键词搜索、成色/价格展示、联系方式 |
| 恋爱情报 | 恋爱档案、匿名表白链接、任务板 |
| 树洞爆料 | 匿名倾诉、点赞、评论、热门排序 |
| 成长系统 | 每日签到（连签奖励）、金币/积分、等级经验条、身份组徽章、积分商城（改名卡/置顶卡/匿名卡/称号） |
| 社交 | 关注/粉丝、私信通知、推送流推荐 |
| 管理后台 | 数据仪表盘（含趋势图）、用户/子站/帖子管理、身份组管理、商城管理、举报中心、公告管理、操作日志 |

## 技术栈

- **后端**：Flask 2.3.3、flask-cors、bcrypt、PyJWT、gunicorn、SQLite
- **前端**：原生 HTML5 / CSS3 / ES6+、Lucide Icons、Chart.js（图表）、Google Fonts（ZCOOL KuaiLe / Ma Shan Zheng）
- **认证**：JWT Bearer Token（7 天有效期）

## 目录结构

```
campus-wall/
├── app/                      # Flask 应用
│   ├── __init__.py           # 应用工厂 create_app()
│   ├── models.py             # 基础数据模型（用户/子站/帖子/评论）
│   ├── models_ext.py         # 扩展模型（签到/商城/恋爱/树洞/交易/管理后台）
│   ├── seed.py               # 种子数据（演示内容）
│   └── routes/               # 路由蓝图
│       ├── auth.py           # 认证：注册/登录/资料/头像/密码
│       ├── posts.py          # 帖子：CRUD/点赞/评论/图片上传/编辑历史
│       ├── stations.py       # 子站：CRUD/成员/转让/分类/统计/封面上传
│       ├── social.py         # 社交：关注/粉丝/通知
│       ├── extended.py       # 扩展：身份组/签到/商城/恋爱/树洞/交易/推荐/后台/收藏/举报/公告
│       └── pages.py          # 页面路由
├── frontend/                 # 静态前端（Cloudflare Pages 部署）
│   ├── pages/                # 各页面 HTML（20+ 页面）
│   ├── static/               # CSS / JS / 图片
│   ├── functions/            # Pages Functions（/api/* 反向代理）
│   ├── _redirects            # 动态路由 → 静态页重写
│   └── README.md             # 前端部署说明
├── instance/                 # SQLite 数据库（运行时生成）
├── run.py                    # 本地启动入口
├── requirements.txt          # Python 依赖
├── .env.example              # 环境变量模板
├── DEPLOYMENT.md             # 部署指南
└── ROADMAP.md / TODO.md      # 路线图与任务清单
```

## 快速开始

### 1. 本地运行（推荐用 dev-server 联调前后端）

```bash
# 克隆仓库
git clone https://github.com/qingshanjiluo/campus-wall.git
cd campus-wall/campus-wall

# 创建虚拟环境并安装依赖
python -m venv venv
venv\Scripts\activate          # Windows
source venv/bin/activate        # Linux / macOS
pip install -r requirements.txt

# 启动联调服务器（Flask 从 frontend/ 读取静态页面，自动初始化数据库）
cd frontend
python dev-server.py
```

浏览器访问 <http://127.0.0.1:5001>

### 2. 纯后端运行

```bash
# 首次运行会自动建库并写入种子数据（含演示账号 admin/admin123）
python run.py
```

浏览器访问 <http://127.0.0.1:5000>

### 3. 演示账号

| 账号 | 密码 | 角色 |
|------|------|------|
| `admin` | `admin123` | 系统管理员 |
| `xiaohua` | `123456` | 普通用户 |

## 生产部署

前端 → **Cloudflare Pages**，后端 → **PythonAnywhere / VPS**。

### 后端部署

```bash
export JWT_SECRET="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export SECRET_KEY="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
export FLASK_ENV=production
export SITE_BASE_URL="https://your-domain.com"
# 可选：SMTP 配置（找回密码邮件，不配置则开发模式回显 token）

python run.py   # 首次初始化
gunicorn -w 3 -b 127.0.0.1:8000 "run:app"
```

> 详细步骤见 [DEPLOYMENT.md](DEPLOYMENT.md)

### 前端部署（Cloudflare Pages）

1. 推送代码到 GitHub
2. Cloudflare → Workers & Pages → Create → Connect to Git
3. Build configuration:
   - **Build command**: 留空
   - **Build output directory**: `frontend`
4. 设置环境变量 `API_BASE=https://你的用户名.pythonanywhere.com`
5. Deploy

前端通过 Pages Functions 将 `/api/*` 与 `/static/uploads/*` 反向代理到后端。

> 详见 [frontend/README.md](frontend/README.md)

## 环境变量

| 变量 | 必填 | 说明 |
|------|------|------|
| `JWT_SECRET` | 生产必填 | JWT 签名密钥，至少 32 位随机串 |
| `SECRET_KEY` | 生产必填 | Flask 会话密钥 |
| `FLASK_ENV` | 否 | `production` 生产 / `development` 开发 |
| `PORT` | 否 | 监听端口，默认 5000 |
| `SITE_BASE_URL` | 否 | 站点地址，用于找回密码链接 |
| `SMTP_HOST/PORT/USER/PASSWORD/TLS` | 否 | 邮件服务（找回密码） |
| `CORS_ORIGINS` | 否 | 允许的跨域来源，逗号分隔，默认 `*` |

## 文档

- [📖 使用文档](USAGE.md) — 各功能模块的操作指南
- [🔌 API 文档](API.md) — 全部 REST 接口说明
- [🚀 部署指南](DEPLOYMENT.md) — 详细部署步骤
- [🗺️ 路线图](ROADMAP.md) — 开发路线
- [✅ 任务清单](TODO.md) — 111 项任务（已完成 100%）

## 版权与许可

仅供学习交流使用。项目基于 MIT 精神开源，数据与内容归用户所有。
