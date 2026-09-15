# CampusWall 上线检查清单（R4 · 服务器自托管主线）

> 状态图例：`[x]` 已验证 · `[~]` 进行中 · `[ ]` 待办（需要真实服务器/域名等外部资源）
> GO 门 = 第 1、2 节全绿 + 第 3 节在目标服务器完成。

## 0. 架构（正式方案）

```
Nginx(80/443, TLS, gzip, uploads 直出, /api 透传真实IP)
  └─ Gunicorn 4w×2t --preload :8000
       └─ Flask app（campus-wall/app · 18 蓝图 · JWT/bcrypt/限流/敏感词审核）
            └─ SQLite WAL（/data volume · DATABASE_PATH 可指任意盘 · 单点可换 PG）
前端 = campus-wall/frontend 静态 22 页（Flask 页路由 + Nginx 静态直出双通道）
部署资产 = campus-wall/deploy/（Dockerfile · compose · nginx · systemd · backup · deploy.sh）
```

Cloudflare Pages/Worker 轨降级为演示环境（backend-worker 保留作 API 契约蓝本），
运维复现见 campus-wall/DEPLOYMENT.md（文件头已声明定位）。

## 1. 后端与契约（本地全栈可证明）

- [x] Flask 主线 E2E **49/49 全绿**（tools/e2e_live.ps1 -BaseUrl http://127.0.0.1:5000）：
  注册/登录/资料 · 子站(加入/退出/成员) · 帖子四类型（**vote 一人一票/重投拒/link 消毒/
  images 数组**）· 点赞/收藏 · 评论 · 签到 · 商店/金币 · 恋爱档案（含 UPDATE 回归）·
  树洞 · 二手 · 搜索/推荐 · **私信 5 步**（发送/会话/已读/未读/坏参数拒）·
  **审核 6 步**（review 入队/公开隐藏/作者可见/approve 发布/block 拒/重复处理拒）·
  上传（multipart→落盘→可访问往返）· admin 统计/用户 · 通知
- [x] Worker 契约蓝本原生断言 205 绿（--mode all --budget），差异已全部移植回 Flask
- [x] 热路径批量化：feed/子站帖列表 is_liked_batch、子站列表 membership 批量、admin stats 4 查询+memo
- [x] 隐私闸：匿名帖在 列表/详情/子站页/搜索 全通道脱敏；pending/rejected 对外 404
- [x] 安全：上传内容校验(PIL verify 防伪扩展名)+EXIF 剥离+长边压缩 · 生产限流
  （register 8/h、login 12/min、发帖 20/min、上传 10-15/min、私信 30/min、树洞/交易 10/min）
  · CORS 经 env 收紧 · JWT 生产强制密钥（缺失拒启动）
- [x] 页面路由矩阵（/、干净路由、/post|station|profile/<id>、未知 404）11/11

## 2. 前端

- [x] 审核 Tab（admin.html）：队列卡片 + 通过/驳回(prompt 理由) + 空态
- [x] 私信页 messages.html：双栏/移动单栏切换、未读徽标、?with= 深链、Enter 发送、
  profile「私信」按钮、导航菜单项、通知 dm 图标+跳转
- [x] 发帖 202 pending 文案（create.html + app.js 快速发帖）、作者视角状态徽标（列表卡+详情横幅）
- [x] 深色模式：`[data-theme]` 双表覆盖 + localStorage + prefers-color-scheme + 导航开关
- [x] XSS/toast/路由门禁/编辑历史等（见 frontend-audit.md 处置表）
- [x] JS 语法门：29 外链 + 全页面内联 node --check 0 失败
- [ ] 真实浏览器 22 页走查（桌面+移动宽度、明暗双主题）——部署完成后执行
- [ ] P2 遗留（低优先）：重复函数收敛、trade 状态机前端接入、lucide 版本锁定

## 3. 部署与运维（目标服务器执行）

- [x] 部署包入库：compose 缺密钥**直接拒启**；app 健康检查驱动 nginx 依赖
- [x] 备份链路：backup.py sqlite3.backup() 在线一致快照 + uploads tar + 滚动 7 份（本地实跑 ✓）
- [x] 发布脚本：deploy.sh（拉码→build→滚动重启→5 次冒烟）
- [x] HTTPS 路径：certbot webroot + 443 段样例 + 自动续期 sidecar
- [x] 无 Docker 备选：systemd 单元（ProtectSystem/PrivateTmp/降权用户）
- [ ] 购买/准备服务器 + 域名备案（如需大陆访问）
- [ ] 服务器上 `docker compose up -d --build` 成功 → 本机跑
      `tools/e2e_live.ps1 -BaseUrl https://域名` **49/49** → GO
- [ ] 改演示 admin 密码 / 或清 instance 库重新种子（手册 §3）
- [ ] 首日观察：`docker compose logs`、备份文件生成、429 是否误伤

## 4. 如实声明的限制（上线公告口径）

1. SQLite 单写者：WAL+批量化设计目标为千级日活；更高并发按 models.py 单点切 PostgreSQL。
2. 限流 storage 默认 memory：gunicorn 4 worker 各自计数（近似值）；多机时 env 指 redis。
3. 敏感词为子串规则库（内置 50+，`SENSITIVE_WORDS_FILE` 热扩展），非语义级——
   需配合管理端审核队列人工兜底（已具备）。
4. 图片上传 ≤16MB/张、长边压至 1600px；无 CDN 水印/缩略图矩阵（规模上来后再议）。
5. demo 种子账号 admin/admin123 上线**必须**改密（手册已置顶警告）。

## 5. 演示账号（seed）

| 账号 | 密码 | 角色 |
|------|------|------|
| admin | admin123 | 管理员（审核/后台全权限） |
| xiaohua | 123456 | 普通用户 |

种子：8 用户 / 15 子站(lucide 图标) / 25 帖 / 16 评论 + 商店/签到/交易/身份组样例。
