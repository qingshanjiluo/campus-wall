# 校园墙 CampusWall

![Backend CI](https://github.com/qingshanjiluo/campus-wall/actions/workflows/backend-ci.yml/badge.svg?branch=master)

> 连接校园，分享青春 —— 面向高校学生的社区站：子站（表白墙/失物招领/二手交易…）、
> 瀑布流、投票/链接帖、匿名树洞、**角色关系图**、**匿名爆料台**、**公开论坛**、私信、
> 恋爱匹配、签到积分商店、二手市场、**广告位占位**、敏感词+人工审核、管理后台、深色模式。

| | |
|---|---|
| 🚀 **正式部署** | 普通服务器自托管（Nginx + Gunicorn + Flask + SQLite）——[部署手册](docs/DEPLOYMENT-SERVER.md) |
| 🧪 **演示环境** | https://campus-wall-673.pages.dev（Cloudflare 轨，仅作参考，非主线） |
| 👤 **演示账号** | 管理员 `admin / admin123` · 普通用户 `xiaohua / 123456` |
| ✅ **质量门** | 本地全栈 E2E **83 步** 绿 · 内联/外链 JS `node --check` 0 失败 |
| 📦 **仓库** | qingshanjiluo/campus-wall（master = 生产） |

## 架构（主线）

```
用户浏览器
   │ 80/443
┌──▼────────────────────────── Nginx（TLS·gzip·静态缓存·真实IP透传）──────────┐
│  /static/uploads/ → volume 直出(30d)   /api/ + 页面 → proxy_pass           │
└──┬─────────────────────────────────────────────────────────────────────────┘
┌──▼───────────── Gunicorn 4w×2t（--preload）────────────────────────────────┐
│  Flask app（campus-wall/app）17+1 蓝图 · JWT · bcrypt · 限流 · 敏感词审核   │
│  SQLite WAL（DATABASE_PATH=/data volume，可平滑换 PostgreSQL）              │
└────────────────────────────────────────────────────────────────────────────┘
  前端 = campus-wall/frontend 静态前端（Nginx 直出静态资源 + Flask 页路由）
```

- **一键上线**：`cd campus-wall/deploy && cp .env.example .env`（填密钥）→ `docker compose up -d --build`
- **每日备份**：compose 内 cron sidecar（sqlite3 在线快照 + uploads，滚动 7 份）
- **更新发布**：`bash deploy/scripts/deploy.sh`（拉码→重建→冒烟）

## 目录速览

```
campus-wall/app/          Flask 后端主线（routes/24 蓝图、models*.py、seed.py）
campus-wall/frontend/     静态前端 28 页（线上唯一前端版本）
campus-wall/deploy/       Dockerfile · compose · nginx · systemd · 备份/发布脚本 · .env.example
backend-worker/           Cloudflare Python Worker + D1（演示轨 & API 契约蓝本）
tools/e2e_live.ps1        83 步全功能 E2E（-BaseUrl 可指 Flask/Worker 任一后端）
docs/                     DEPLOYMENT-SERVER · SERVER_ARCHITECTURE · LAUNCH_CHECKLIST · frontend-audit
```

## 开发 & 验证

```bash
# 起本地全栈（自动建表+种子）
cd campus-wall && python run.py            # http://127.0.0.1:5000

# 全功能 E2E（GO 门）
powershell -ExecutionPolicy Bypass -File tools/e2e_live.ps1 -BaseUrl http://127.0.0.1:5000

# 后端契约蓝本原生断言（可选）
cd backend-worker && python tools/run_native_tests.py --mode all --budget

# 生产环境（Linux 服务器）
cd campus-wall/deploy && docker compose up -d --build
```

## 功能地图（后端均已 E2E 覆盖）

| 域 | 亮点 |
|---|---|
| 帖子 | text/image/**vote/link** 四类型；投票一人一票；编辑保留票数；编辑历史 |
| 角色图 | **/world 多人共同维护的角色-关系图谱**，力导向图可视化（拖拽/点选/关系高亮/双向虚线） |
| 爆料 | **/expose 匿名爆料**：post_type=expose 恒匿名（连管理员只见匿名名）+ 先审后发 |
| 论坛 | **/forum 公开论坛**：子站=版块矩阵 + 跨版块最新热议聚合 |
| 广告 | **广告位占位**：site_config 可配，导航/页脚 ad-slot，admin 可开关与改文案 |
| 审核 | 敏感词三级（block 拒/review 队列/直发）· admin 审核 Tab · 作者状态徽标 |
| 私信 | 会话列表/未读徽标/深链 `?with=uid` · 移动端单栏 · 发信即通知 |
| 社区 | 子站(私密/仅站长发贴) · 树洞马甲 · 二手 · 恋爱匹配 · 签到/任务/商店 |
| 管理 | 统计+趋势图 · 用户/子站/帖子/身份组/商城/公告/举报/广告位/操作日志 |
| 话题/热搜 | 帖子话题（显式+`#`内联识别）· 近 7 天热度榜 · 话题聚合过滤 · admin 管理 |
| 任务系统 | 系统任务自动发奖 · 官方任务 · 积分悬赏（预扣-提交-审核-放款）· 名额/防重领 |
| 活动系统 | 发布（日期/名额/打卡金币）· 报名容量闸 · 到场打卡发奖 |
| 暗阁 | 付费可见（未购者仅 30 字摘要，解锁收入归作者）· 积分推流（推荐流置前）|
| 聊天室 | 多房间（公开/私有）· @提及通知 · 消息撤回 · 增量轮询 |
| AI 生态 | 可插拔适配器（规则引擎默认/OpenAI 兼容可选）· AI 管理员审核建议 · AI 用户互动 · AI 接管私信 |
| 个性化 | 四套皮肤 × 明暗 · 导航选择器 · 服务端偏好 · 数据导出（JSON/CSV）|
| 治理扩展 | 等级规则引擎自动升级 · 身份组自动授予 · 访客模式门控 · 站点定制注入（CSS/HTML/JS）· 插件钩子 |
| 安全 | JWT+bcrypt · 接口限流(生产开) · 上传防伪+Pillow 压缩+EXIF 剥离 · 匿名脱敏 |

## 文档

- 🧭 正式部署：[docs/DEPLOYMENT-SERVER.md](docs/DEPLOYMENT-SERVER.md)
- 🏗 架构定案：[docs/SERVER_ARCHITECTURE.md](docs/SERVER_ARCHITECTURE.md)
- ✅ 上线清单：[docs/LAUNCH_CHECKLIST.md](docs/LAUNCH_CHECKLIST.md)
- 🗂 方案与决策：[PLAN.md](PLAN.md) · 结构考古：[PROJECT_MAP.md](PROJECT_MAP.md)
- 🧪 Cloudflare 演示轨说明：[campus-wall/DEPLOYMENT.md](campus-wall/DEPLOYMENT.md)（已降级为参考）

## 已知限制 / 演进路线

- SQLite 单写者：WAL 下支撑 ~千日活无虞；起量按 `query_db/execute_db` 单点切 PostgreSQL。
- 限流默认 memory storage：多机部署时 `RATELIMIT_STORAGE_URI` 指 redis 即可。
- Pages 演示轨依赖 `*.pages.dev`（大陆可达性波动），正式站走自有域名+服务器。
