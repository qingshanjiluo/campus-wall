# 会话寿命：滑动会话（2026-06-12）

> 本仓自有工程笔记（**非** infra 镜像）。记录 kungal 本地会话的生命周期模型与
> 2026-06 的「每周掉线」修复。moyu（kun-galgame-patch）有一份对称实现，见其
> `docs/proj/session-lifetime.md`。

## 背景：kungal 的会话是 BFF 不透明会话，不是 JWT

虽然 `CLAUDE.md` 把 `internal/middleware/auth.go` 简称为「仅验签 JWT」，实际机制是
**Backend-for-Frontend（BFF）会话**：

- 登录（`POST /api/auth/oauth/callback`）走完 OAuth code 交换后，后端在 **Redis** 里
  存一份 `SessionData`（OAuth access/refresh token + 用户身份），key 为
  `kungal:session:<token>`；浏览器只拿到一个 **httpOnly 不透明 cookie**
  `kungal_session`（值就是那个随机 token）。OAuth 的 access/refresh token **永不
  落到浏览器**。
- 每个请求,`Auth` 中间件用 cookie 取 Redis 会话；access token 临近过期（30s 内）
  就用会话里的 refresh token 同步刷新（SETNX 单飞锁 + `waitForRefresh` 等待者），
  刷新失败按「可恢复 / 不可恢复」分流（见 `refreshSession`）。

## 修复的 Bug：固定 7 天 cookie ⇒ 每周必掉线

旧实现里 `kungal_session` 的 `MaxAge` 在**登录时一次性写死 7 天，之后任何请求都不
重发**。于是无论用户多活跃，**登录满 7 天浏览器就丢掉 cookie → 掉线**。Redis 会话
TTL 当时也是固定 7 天。上游 OAuth 其实支持 90 天滑动 refresh token（见 infra
`oauth_clients.refresh_token_ttl_seconds` 默认 7776000），是本地这层把它砍到了一周。

## 现在：90 天滑动窗口 + 上游 refresh token 兜底

会话改成**滑动**（rolling/sliding session，业界标准做法，参考 OWASP 会话管理与
ASP.NET Core `SlidingExpiration`）：

1. **窗口 = 90 天**，对齐上游 refresh token 默认寿命。`SessionTTL = 90 * 24 * time.Hour`
   同时用于 Redis TTL 和 cookie `MaxAge`（登录、刷新、续期处统一引用）。
2. **活跃即续期**：`renewSlidingSession` 在 `Auth` / `OptionalAuth` 校验通过后调用，
   把 cookie 和 Redis TTL 一起向前滑动。
3. **「过半才续签」节流**：不是每个请求都 `Set-Cookie`。用一个 marker key
   `kungal:session-renew:<token>`（TTL = 半窗口）做节流——`SetNX` 只在上一个 marker
   过期后才成功（即距上次续期 > `SessionTTL/2`），成功才续。
4. **续期只 `EXPIRE`、不重写会话内容**：续期对 Redis 会话 key 做 `EXPIRE`（只动
   TTL），**绝不重写 blob**。这样它和「刷新 token 时重写会话」彻底无竞态，不会把刚
   轮换出来的新 refresh token 覆盖回旧的。
5. **绝对上限 = 上游 refresh token，fail-closed**：本地不另设硬上限。活跃用户两边都
   滑动（refresh 每 ~15 分钟轮换 +90d，会话随之续）→ 实际不会掉线；闲置用户约 90 天
   后，本地 Redis 过期与上游 refresh token 失效**同时发生**，`refreshSession` 删会话、
   用户重新登录。

净效果：**活跃用户不再每周掉线**；闲置用户约 90 天后自然过期（与上游一致）。

## 平滑迁移

`renewSlidingSession` 不依赖会话里的任何新字段（marker 在独立 key 上）。线上既有的
7 天 cookie 会话，在下次请求时被续成 90 天滑动窗口，**无需迁移脚本**。

## 安全权衡

被盗 cookie 的有效期从 7 天变长到「最长 90 天滑动」——但这只是对齐了上游 OAuth 授权
本就允许的窗口，没有放大授权范围；cookie 仍是 httpOnly + Secure（prod）+ SameSite=Lax，
且封禁 / refresh token 失效会在下次刷新即时 fail-closed。后续可选硬化：定期轮换
session id（OWASP renewal timeout）——本次未做。

## 改了哪些文件

| 文件 | 改动 |
|---|---|
| `internal/middleware/auth.go` | `SessionTTL` 7d→90d；新增 `sessionRenewPrefix`、`SecureCookies`、`renewSlidingSession`；`Auth`/`OptionalAuth` 调用续期；`refreshSession` 的 Redis TTL 改用 `SessionTTL` |
| `internal/user/service/auth_service.go` | 登录写会话用 `middleware.SessionTTL` |
| `internal/user/handler/oauth_handler.go` | 登录 cookie `MaxAge` 用 `int(middleware.SessionTTL.Seconds())` |
| `internal/app/router.go` | 启动时 `middleware.SecureCookies = Server.Mode == "prod"` |

marker key 用独立前缀 `kungal:session-renew:`，不与 `kungal:session:*` 混淆。
