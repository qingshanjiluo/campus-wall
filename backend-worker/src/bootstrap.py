"""自举模块（由 tools/gen_bootstrap.py 生成，请勿手改 SQL 部分）。

职责：
1. SCHEMA_SQL —— 内嵌 D1 建表语句（与 migrations/ 保持同步）
2. ensure_ready() —— 每个 isolate 首次请求时：建表（幂等）+ 空库自动播种
"""
import seed as seedmod

_READY = False

SCHEMA_SQL = '''
-- ── migrations/0001_init.sql ──
-- CampusWall D1 初始化迁移：包含全部基础表 + 扩展表

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT UNIQUE NOT NULL,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    avatar TEXT DEFAULT '/static/images/default-avatar.svg',
    bio TEXT DEFAULT '',
    role TEXT DEFAULT 'user',
    coins INTEGER DEFAULT 0,
    points INTEGER DEFAULT 0,
    level INTEGER DEFAULT 1,
    exp INTEGER DEFAULT 0,
    checkin_streak INTEGER DEFAULT 0,
    last_checkin TEXT DEFAULT '',
    identity_group TEXT DEFAULT '',
    title TEXT DEFAULT '',
    mood TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS stations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    cover TEXT DEFAULT '',
    icon TEXT DEFAULT 'school',
    tags TEXT DEFAULT '[]',
    user_count INTEGER DEFAULT 0,
    post_count INTEGER DEFAULT 0,
    owner_id INTEGER REFERENCES users(id),
    is_public INTEGER DEFAULT 1,
    only_owner_posts INTEGER DEFAULT 0,
    announcement TEXT DEFAULT '',
    category TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    image TEXT DEFAULT '',
    images TEXT DEFAULT '[]',
    author_id INTEGER REFERENCES users(id),
    station_id INTEGER REFERENCES stations(id),
    views INTEGER DEFAULT 0,
    likes_count INTEGER DEFAULT 0,
    comments_count INTEGER DEFAULT 0,
    is_pinned INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    post_type TEXT DEFAULT 'text',
    extra TEXT DEFAULT '{}',
    is_anonymous INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL,
    author_id INTEGER REFERENCES users(id),
    post_id INTEGER REFERENCES posts(id),
    parent_id INTEGER REFERENCES comments(id),
    likes_count INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS post_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER REFERENCES posts(id),
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    image TEXT DEFAULT '',
    edited_by INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS likes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, target_type, target_id)
);

CREATE TABLE IF NOT EXISTS follows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    follower_id INTEGER REFERENCES users(id),
    following_id INTEGER REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(follower_id, following_id)
);

CREATE TABLE IF NOT EXISTS station_members (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    station_id INTEGER REFERENCES stations(id),
    role TEXT DEFAULT 'member',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, station_id)
);

CREATE TABLE IF NOT EXISTS notifications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    from_user_id INTEGER REFERENCES users(id),
    type TEXT NOT NULL,
    content TEXT NOT NULL,
    link TEXT DEFAULT '',
    is_read INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ============ 扩展表 ============

CREATE TABLE IF NOT EXISTS identity_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    icon TEXT DEFAULT 'tag',
    color TEXT DEFAULT '#fb6f92',
    description TEXT DEFAULT '',
    permissions TEXT DEFAULT '[]',
    min_level INTEGER DEFAULT 1,
    is_default INTEGER DEFAULT 0,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS site_announcements (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT DEFAULT '',
    created_by INTEGER REFERENCES users(id),
    is_active INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS checkins (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    checkin_date TEXT NOT NULL,
    streak INTEGER DEFAULT 1,
    coins_earned INTEGER DEFAULT 0,
    points_earned INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, checkin_date)
);

CREATE TABLE IF NOT EXISTS coin_transactions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    amount INTEGER NOT NULL,
    balance_after INTEGER DEFAULT 0,
    type TEXT NOT NULL,
    description TEXT DEFAULT '',
    ref_type TEXT DEFAULT '',
    ref_id INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS shop_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    icon TEXT DEFAULT 'gift',
    price_coins INTEGER DEFAULT 0,
    price_points INTEGER DEFAULT 0,
    item_type TEXT DEFAULT 'badge',
    item_data TEXT DEFAULT '{}',
    stock INTEGER DEFAULT -1,
    is_active INTEGER DEFAULT 1,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS shop_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER REFERENCES users(id),
    item_id INTEGER REFERENCES shop_items(id),
    quantity INTEGER DEFAULT 1,
    total_coins INTEGER DEFAULT 0,
    total_points INTEGER DEFAULT 0,
    status TEXT DEFAULT 'completed',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS romance_profiles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER UNIQUE REFERENCES users(id),
    nickname TEXT DEFAULT '',
    gender TEXT DEFAULT '',
    age INTEGER DEFAULT 0,
    department TEXT DEFAULT '',
    hobbies TEXT DEFAULT '[]',
    looking_for TEXT DEFAULT '',
    photo TEXT DEFAULT '',
    is_visible INTEGER DEFAULT 1,
    likes_received INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS romance_links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    from_user_id INTEGER REFERENCES users(id),
    to_user_id INTEGER REFERENCES users(id),
    link_type TEXT DEFAULT 'crush',
    description TEXT DEFAULT '',
    is_anonymous INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS romance_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    creator_id INTEGER REFERENCES users(id),
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    task_type TEXT DEFAULT 'matchmake',
    target_user_id INTEGER DEFAULT 0,
    reward_coins INTEGER DEFAULT 0,
    status TEXT DEFAULT 'open',
    assigned_to INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gossip (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    content TEXT NOT NULL,
    images TEXT DEFAULT '[]',
    station_id INTEGER REFERENCES stations(id),
    likes_count INTEGER DEFAULT 0,
    comments_count INTEGER DEFAULT 0,
    is_anonymous INTEGER DEFAULT 1,
    is_hot INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS gossip_comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    gossip_id INTEGER REFERENCES gossip(id),
    content TEXT NOT NULL,
    author_name TEXT DEFAULT '匿名',
    likes_count INTEGER DEFAULT 0,
    is_deleted INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS kanban_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    message TEXT NOT NULL,
    message_type TEXT DEFAULT 'greeting',
    trigger_type TEXT DEFAULT 'time',
    trigger_data TEXT DEFAULT '{}',
    is_active INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS admin_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_id INTEGER REFERENCES users(id),
    action TEXT NOT NULL,
    target_type TEXT DEFAULT '',
    target_id INTEGER DEFAULT 0,
    detail TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS trade_posts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    post_id INTEGER REFERENCES posts(id),
    user_id INTEGER REFERENCES users(id),
    price REAL DEFAULT 0,
    original_price REAL DEFAULT 0,
    condition TEXT DEFAULT 'good',
    category TEXT DEFAULT '',
    status TEXT DEFAULT 'available',
    contact TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS favorites (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, target_type, target_id)
);

CREATE TABLE IF NOT EXISTS reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    reporter_id INTEGER NOT NULL,
    target_type TEXT NOT NULL,
    target_id INTEGER NOT NULL,
    reason TEXT DEFAULT '',
    detail TEXT DEFAULT '',
    status TEXT DEFAULT 'pending',
    handler_id INTEGER,
    handled_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_settings (
    user_id INTEGER PRIMARY KEY,
    notify_comment INTEGER DEFAULT 1,
    notify_like INTEGER DEFAULT 1,
    notify_follow INTEGER DEFAULT 1,
    notify_system INTEGER DEFAULT 1,
    theme TEXT DEFAULT 'auto',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- 上传文件（图片以 base64 存 D1；R2 启用后可切换为 R2 对象存储）
CREATE TABLE IF NOT EXISTS uploads (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT UNIQUE NOT NULL,
    filename TEXT DEFAULT '',
    content_type TEXT DEFAULT 'application/octet-stream',
    data TEXT NOT NULL,
    size INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_uploads_key ON uploads(key);
'''


def _iter_statements(sql):
    """按分号拆分并剔除行注释（D1 exec 对纯注释段报 no statement）。"""
    for chunk in sql.split(';'):
        lines = [ln for ln in chunk.splitlines() if not ln.strip().startswith("--")]
        body = "\n".join(lines).strip()
        if body:
            yield body


async def ensure_ready():
    """幂等自举 + KV 模式每请求缓存轮换。失败抛出异常由入口统一转 500。

    读预算策略（D1 免费层 50k 行/日）：
    - 每个请求先 _db.begin_request()（KV 模式轮换分片缓存；D1 模式空操作）。
    - isolate 首请求只做一条廉价探针 SELECT 1 FROM users LIMIT 1：
      探到 users 行（已建表且已播种）→ 完全跳过迁移语句重放。
    - 仅当探针为空/表不存在时才重放建表并播种；
      KV 模式下分片初始化自带全量 schema，无需重放。
    """
    global _READY
    import db as _db
    _db.begin_request()
    if _READY:
        return
    probe = []
    try:
        probe = await _db.query('SELECT 1 AS x FROM users LIMIT 1')
    except Exception:
        probe = []
    if not probe:
        if _db.backend() != 'kv':
            for stmt in _iter_statements(SCHEMA_SQL):
                await _db.execute(stmt)
        await seedmod.seed()
    _READY = True
