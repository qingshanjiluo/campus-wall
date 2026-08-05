# 数据库设计

## 概述

项目使用 PostgreSQL 作为关系数据库，通过 Prisma 7 ORM 进行数据访问。Prisma Schema 采用**模块化拆分**设计，每个业务实体一个 `.prisma` 文件。

## Schema 文件结构

```
prisma/
├── schema/
│   ├── schema.prisma              # 主配置（generator + datasource）
│   ├── user.prisma                # 用户、好友、关注
│   ├── galgame.prisma             # Galgame 核心及关联模型
│   ├── galgame-resource.prisma    # Galgame 资源（下载链接）
│   ├── galgame-comment.prisma     # Galgame 评论
│   ├── galgame-history.prisma     # Galgame 编辑历史
│   ├── galgame-link.prisma        # Galgame 外部链接
│   ├── galgame-pr.prisma          # Galgame PR（修改请求）
│   ├── galgame-website.prisma     # 网站模块
│   ├── galgame-toolset.prisma     # 工具集
│   ├── galgame_rating.prisma      # 评分系统
│   ├── topic.prisma               # 讨论话题
│   ├── topic-comment.prisma       # 话题评论
│   ├── topic-reply.prisma         # 话题回复
│   ├── topic-poll.prisma          # 话题投票
│   ├── message.prisma             # 消息系统
│   ├── chat.prisma                # 聊天系统
│   ├── doc.prisma                 # 文档/文章
│   ├── report.prisma              # 举报
│   ├── unmoe.prisma               # Unmoe 内容
│   ├── update-log.prisma          # 更新日志
│   └── todo.prisma                # 待办事项（管理员）
├── generated/                     # 生成的 Prisma Client
│   └── prisma/
└── prisma.ts                      # Prisma Client 实例
```

## 核心数据模型

### 用户系统

```
user
├── id (PK, autoincrement)
├── name (unique)
├── email (unique)
├── password (bcrypt hash)
├── ip, avatar, role, status
├── moemoepoint (积分系统)
├── bio
├── daily_check_in          # 每日签到计数
├── daily_image_count       # 每日图片上传计数
├── daily_toolset_upload_count
├── created, updated
│
├── user_friend (多对多，用户好友关系)
└── user_follow (多对多，关注关系)
```

**角色等级** (`role` 字段):
- 值越大权限越高
- 管理员: `role > 2`
- 普通用户: `role = 1`

**用户状态** (`status` 字段):
- `0` - 正常
- `1` - 已封禁

### Galgame 核心

```
galgame
├── id (PK, autoincrement)
├── vndb_id (unique, 如 "v12345")
├── name_en_us, name_ja_jp, name_zh_cn, name_zh_tw  # 多语言名称
├── intro_en_us, intro_ja_jp, intro_zh_cn, intro_zh_tw  # 多语言介绍
├── banner                   # 横幅图片
├── content_limit            # 内容限制 ("sfw"/"nsfw")
├── age_limit                # 年龄限制 ("all"/"r18")
├── original_language        # 原始语言
├── view                     # 浏览次数
├── status, user_id, series_id
│
├── galgame_resource[]       # 下载资源
├── galgame_comment[]        # 评论
├── galgame_link[]           # 外部链接
├── galgame_pr[]             # 修改请求
├── galgame_history[]        # 编辑历史
├── galgame_alias[]          # 别名
├── galgame_rating[]         # 评分
├── galgame_like[]           # 点赞
├── galgame_favorite[]       # 收藏
├── galgame_contributor[]    # 贡献者
│
├── galgame_official_relation[]  # 开发商（多对多）
├── galgame_engine_relation[]    # 引擎（多对多）
└── galgame_tag_relation[]       # 标签（多对多，含 spoiler_level）
```

### 讨论系统

```
topic (话题)
├── topic_reply[] (回复)
│   ├── topic_comment[] (评论)
│   ├── topic_reply_like[]
│   └── topic_reply_dislike[]
├── topic_like[]
├── topic_dislike[]
├── topic_upvote[]
├── topic_favorite[]
└── topic_poll[] (投票)
    └── topic_poll_vote[]
```

### 消息系统

```
message (用户间消息)
├── sender → user
└── receiver → user

system_message (系统消息)
├── sender → user (管理员)
└── receiver → all users

chat_room → chat_room_participant[] → chat_message[]
```

## Schema 设计规范

### 命名约定

- Model 名: `snake_case` (如 `galgame_resource`)
- 字段名: `snake_case` (如 `user_id`, `vndb_id`)
- 多语言字段: `{field}_{locale}` (如 `name_en_us`, `intro_zh_cn`)
- 外键: `{entity}_id` (如 `galgame_id`, `user_id`)

### 通用字段

所有模型包含时间戳：

```prisma
created DateTime @default(now())
updated DateTime @updatedAt
```

### 关联模式

**一对多**: 直接外键关联

```prisma
model galgame_resource {
  galgame_id Int
  galgame    galgame @relation(fields: [galgame_id], references: [id], onDelete: Cascade)
}
```

**多对多**: 使用中间表 + 复合主键

```prisma
model galgame_tag_relation {
  galgame_id Int
  tag_id     Int

  galgame galgame     @relation(fields: [galgame_id], references: [id], onDelete: Cascade)
  tag     galgame_tag @relation(fields: [tag_id], references: [id], onDelete: Cascade)

  spoiler_level Int @default(0)  // 中间表可携带额外字段

  @@id([galgame_id, tag_id])     // 复合主键
}
```

**唯一约束**: 防止重复关联

```prisma
model galgame_like {
  galgame_id Int
  user_id    Int
  @@unique([galgame_id, user_id])
}
```

### 级联删除

关联关系统一使用 `onDelete: Cascade`，父记录删除时自动清理子记录。可选外键使用 `onDelete: SetNull`。

## Prisma Client 使用

### 实例导入

```typescript
import { prisma } from '~/prisma/prisma'
```

### 查询模式

```typescript
// 关联查询 + 选择字段
const result = await prisma.galgame.findMany({
  where: { status: 0 },
  include: {
    user: {
      select: { id: true, name: true, avatar: true }
    },
    _count: {
      select: { like: true, comment: true }
    }
  },
  orderBy: { created: 'desc' },
  take: input.limit,
  skip: (input.page - 1) * input.limit
})

// 事务
return prisma.$transaction(async (prisma) => {
  // 多步操作
}, { timeout: 60000 })
```

## 相关脚本

| 命令 | 用途 |
|------|------|
| `pnpm prisma:generate` | 生成 Prisma Client |
| `pnpm prisma:push` | 推送 Schema 变更到数据库 |
| `pnpm prisma:pull` | 从数据库拉取 Schema |
| `pnpm prisma:studio` | 打开 Prisma Studio GUI |
