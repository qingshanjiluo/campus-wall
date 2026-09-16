import os
import json as _json
import sqlite3
import bcrypt
from datetime import datetime

DB_PATH = os.environ.get('DATABASE_PATH') or os.path.join(
    os.path.dirname(os.path.dirname(__file__)), 'instance', 'campushub.db')
_db_dir = os.path.dirname(DB_PATH)
if _db_dir:
    os.makedirs(_db_dir, exist_ok=True)


def get_db():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode=WAL")
    conn.execute("PRAGMA foreign_keys=ON")
    return conn


def dict_from_row(row):
    return dict(row) if row else None


def query_db(sql, args=(), one=False):
    conn = get_db()
    cur = conn.execute(sql, args)
    rv = cur.fetchall()
    conn.close()
    return (dict_from_row(rv[0]) if rv else None) if one else [dict_from_row(r) for r in rv]


def execute_db(sql, args=()):
    conn = get_db()
    try:
        cur = conn.execute(sql, args)
        conn.commit()
        lastid = cur.lastrowid
        conn.close()
        return lastid
    except Exception as e:
        conn.close()
        raise e


def init_db():
    os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)
    conn = get_db()
    c = conn.cursor()

    c.execute('''CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        username TEXT UNIQUE NOT NULL,
        email TEXT UNIQUE NOT NULL,
        password_hash TEXT NOT NULL,
        avatar TEXT DEFAULT '/static/images/default-avatar.svg',
        bio TEXT DEFAULT '',
        role TEXT DEFAULT 'user',
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS stations (
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
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    # 兼容旧库：补充缺失列
    for col, coltype, default in [
        ('cover', 'TEXT', "''"),
        ('only_owner_posts', 'INTEGER', '0'),
        ('announcement', 'TEXT', "''"),
        ('category', 'TEXT', "''"),
    ]:
        try:
            c.execute(f"ALTER TABLE stations ADD COLUMN {col} {coltype} DEFAULT {default}")
        except sqlite3.OperationalError:
            pass

    c.execute('''CREATE TABLE IF NOT EXISTS posts (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        title TEXT NOT NULL,
        content TEXT NOT NULL,
        image TEXT DEFAULT '',
        author_id INTEGER REFERENCES users(id),
        station_id INTEGER REFERENCES stations(id),
        views INTEGER DEFAULT 0,
        likes_count INTEGER DEFAULT 0,
        comments_count INTEGER DEFAULT 0,
        is_pinned INTEGER DEFAULT 0,
        is_deleted INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS comments (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        content TEXT NOT NULL,
        author_id INTEGER REFERENCES users(id),
        post_id INTEGER REFERENCES posts(id),
        parent_id INTEGER REFERENCES comments(id),
        likes_count INTEGER DEFAULT 0,
        is_deleted INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS post_versions (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        post_id INTEGER REFERENCES posts(id),
        title TEXT NOT NULL,
        content TEXT NOT NULL,
        image TEXT DEFAULT '',
        edited_by INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS likes (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        target_type TEXT NOT NULL,
        target_id INTEGER NOT NULL,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, target_type, target_id)
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS follows (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        follower_id INTEGER REFERENCES users(id),
        following_id INTEGER REFERENCES users(id),
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(follower_id, following_id)
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS station_members (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        station_id INTEGER REFERENCES stations(id),
        role TEXT DEFAULT 'member',
        joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        UNIQUE(user_id, station_id)
    )''')

    c.execute('''CREATE TABLE IF NOT EXISTS notifications (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user_id INTEGER REFERENCES users(id),
        from_user_id INTEGER REFERENCES users(id),
        type TEXT NOT NULL,
        content TEXT NOT NULL,
        link TEXT DEFAULT '',
        is_read INTEGER DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )''')

    conn.commit()
    conn.close()


# ── User helpers ──

def create_user(username, email, password):
    salt = bcrypt.gensalt()
    pw_hash = bcrypt.hashpw(password.encode('utf-8'), salt).decode('utf-8')
    try:
        return execute_db(
            'INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)',
            (username, email, pw_hash)
        )
    except sqlite3.IntegrityError:
        return None


def get_user_by_username(username):
    return query_db('SELECT * FROM users WHERE username = ?', (username,), one=True)


def get_user_by_email(email):
    return query_db('SELECT * FROM users WHERE email = ?', (email,), one=True)


def get_user_by_id(uid):
    return query_db('SELECT * FROM users WHERE id = ?', (uid,), one=True)


def verify_password(password, pw_hash):
    return bcrypt.checkpw(password.encode('utf-8'), pw_hash.encode('utf-8'))


def change_password(uid, new_password):
    salt = bcrypt.gensalt()
    pw_hash = bcrypt.hashpw(new_password.encode('utf-8'), salt).decode('utf-8')
    execute_db('UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?', (pw_hash, uid))


def update_user(uid, **kwargs):
    allowed = {'username', 'email', 'avatar', 'bio', 'mood', 'title'}
    fields = {k: v for k, v in kwargs.items() if k in allowed and v is not None}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [uid]
    execute_db(f'UPDATE users SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
    return True


def get_user_stats(uid):
    # 口径=对外可见：排除已删除与未过审（此前连 is_deleted 都没过滤）
    posts = query_db(
        "SELECT COUNT(*) as c FROM posts WHERE author_id = ? AND is_deleted = 0 AND status = 'approved'",
        (uid,), one=True)['c']
    followers = query_db('SELECT COUNT(*) as c FROM follows WHERE following_id = ?', (uid,), one=True)['c']
    following = query_db('SELECT COUNT(*) as c FROM follows WHERE follower_id = ?', (uid,), one=True)['c']
    return {'posts': posts, 'followers': followers, 'following': following}


# ── Station helpers ──

def create_station(name, description, icon, tags, owner_id, category=''):
    import json
    tags_str = json.dumps(tags, ensure_ascii=False) if isinstance(tags, list) else tags
    sid = execute_db(
        'INSERT INTO stations (name, description, icon, tags, owner_id, category) VALUES (?, ?, ?, ?, ?, ?)',
        (name, description, icon or 'school', tags_str, owner_id, category)
    )
    if sid:
        execute_db('INSERT INTO station_members (user_id, station_id, role) VALUES (?, ?, ?)',
                    (owner_id, sid, 'owner'))
        execute_db('UPDATE stations SET user_count = 1 WHERE id = ?', (sid,))
    return sid


def get_stations(limit=20, offset=0, sort='newest', tag=None, category=None):
    sql = 'SELECT * FROM stations WHERE 1=1'
    args = []
    if tag:
        sql += ' AND tags LIKE ?'
        args.append(f'%{tag}%')
    if category:
        sql += ' AND (category = ? OR category LIKE ?)'
        args += [category, f'{category}/%']
    if sort == 'popular':
        sql += ' ORDER BY user_count DESC'
    elif sort == 'posts':
        sql += ' ORDER BY post_count DESC'
    else:
        sql += ' ORDER BY created_at DESC'
    sql += ' LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)


def get_station_by_id(sid):
    return query_db('SELECT * FROM stations WHERE id = ?', (sid,), one=True)


def get_station_categories():
    """返回所有使用的分类（含层级，按 / 分隔）"""
    rows = query_db(
        "SELECT DISTINCT category FROM stations WHERE category != '' ORDER BY category")
    cats = []
    for r in rows:
        parts = r['category'].split('/')
        cats.append({
            'path': r['category'],
            'parent': parts[0] if len(parts) > 1 else '',
            'name': parts[-1],
            'depth': len(parts) - 1,
        })
    return cats


def search_stations(keyword):
    return query_db(
        'SELECT * FROM stations WHERE name LIKE ? OR description LIKE ? ORDER BY user_count DESC',
        (f'%{keyword}%', f'%{keyword}%')
    )


def join_station(user_id, station_id):
    try:
        execute_db('INSERT INTO station_members (user_id, station_id) VALUES (?, ?)', (user_id, station_id))
        execute_db('UPDATE stations SET user_count = user_count + 1 WHERE id = ?', (station_id,))
        return True
    except sqlite3.IntegrityError:
        return False


def leave_station(user_id, station_id):
    conn = get_db()
    cur = conn.execute('DELETE FROM station_members WHERE user_id = ? AND station_id = ?', (user_id, station_id))
    deleted = cur.rowcount
    conn.commit()
    if deleted > 0:
        conn.execute('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?', (station_id,))
        conn.commit()
    conn.close()


def is_station_member(user_id, station_id):
    return query_db(
        'SELECT 1 FROM station_members WHERE user_id = ? AND station_id = ?',
        (user_id, station_id), one=True
    ) is not None


def get_station_member_role(user_id, station_id):
    """获取用户在子站中的角色（owner/member/None）"""
    row = query_db(
        'SELECT role FROM station_members WHERE user_id = ? AND station_id = ?',
        (user_id, station_id), one=True)
    return row['role'] if row else None


def get_station_membership(user_id, station_id):
    """一行查询取回本人与该子站的关系角色（None=未加入）。"""
    return get_station_member_role(user_id, station_id)


def get_station_memberships(user_id, station_ids):
    """批量成员关系：{station_id: role}（列表热路径 O(N)→O(1)）。"""
    ids = [i for i in station_ids if i is not None]
    if not ids:
        return {}
    ph = ', '.join('?' for _ in ids)
    rows = query_db(
        'SELECT station_id, role FROM station_members WHERE user_id = ? AND station_id IN (%s)' % ph,
        [user_id] + list(ids))
    return {r['station_id']: r['role'] for r in rows}


def get_station_members(station_id):
    """子站成员列表（含用户信息）"""
    return query_db(
        '''SELECT sm.role, sm.joined_at, u.id as user_id, u.username, u.avatar, u.bio
           FROM station_members sm JOIN users u ON sm.user_id = u.id
           WHERE sm.station_id = ?
           ORDER BY (sm.role = 'owner') DESC, sm.joined_at ASC''',
        (station_id,))


def get_user_stations(user_id):
    """用户加入的子站列表"""
    return query_db(
        '''SELECT s.*, sm.role, sm.joined_at
           FROM station_members sm JOIN stations s ON sm.station_id = s.id
           WHERE sm.user_id = ?
           ORDER BY sm.joined_at DESC''',
        (user_id,))


def get_station_stats(sid, days=7):
    """子站近 days 天发帖趋势与基础指标"""
    from datetime import datetime, timedelta
    days = max(1, min(30, int(days)))
    labels, counts = [], []
    for i in range(days - 1, -1, -1):
        d = (datetime.now() - timedelta(days=i)).strftime('%Y-%m-%d')
        labels.append(d[5:])
        counts.append(query_db(
            "SELECT COUNT(*) as c FROM posts WHERE station_id = ? AND date(created_at) = ? AND is_deleted = 0",
            (sid, d), one=True)['c'])
    today_posts = query_db(
        "SELECT COUNT(*) as c FROM posts WHERE station_id = ? AND date(created_at) = date('now') AND is_deleted = 0",
        (sid,), one=True)['c']
    return {
        'labels': labels,
        'posts': counts,
        'today_posts': today_posts,
    }


def remove_station_member(user_id, station_id):
    """从子站移除成员（不能移owner）"""
    conn = get_db()
    cur = conn.execute(
        'DELETE FROM station_members WHERE user_id = ? AND station_id = ? AND role != ?',
        (user_id, station_id, 'owner'))
    deleted = cur.rowcount
    conn.commit()
    if deleted > 0:
        conn.execute('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?', (station_id,))
        conn.commit()
    conn.close()
    return deleted > 0


def transfer_station_ownership(sid, new_owner_id):
    """转让子站所有权：新owner的role升为owner，旧owner降为member"""
    conn = get_db()
    # 旧 owner 降为 member
    conn.execute(
        'UPDATE station_members SET role = ? WHERE station_id = ? AND role = ?',
        ('member', sid, 'owner'))
    # 新 owner 若不在成员表则加入，否则升级
    conn.execute(
        'INSERT OR IGNORE INTO station_members (user_id, station_id, role) VALUES (?, ?, ?)',
        (new_owner_id, sid, 'member'))
    conn.execute(
        'UPDATE station_members SET role = ? WHERE user_id = ? AND station_id = ?',
        ('owner', new_owner_id, sid))
    conn.execute('UPDATE stations SET owner_id = ? WHERE id = ?', (new_owner_id, sid))
    conn.commit()
    conn.close()


def update_station(sid, **fields):
    """更新子站信息"""
    allowed = ('name', 'description', 'icon', 'tags', 'is_public', 'cover', 'only_owner_posts', 'announcement', 'category')
    pairs = [(k, fields[k]) for k in allowed if k in fields]
    if not pairs:
        return False
    sql = 'UPDATE stations SET ' + ', '.join(f'{k} = ?' for k, _ in pairs) + ', updated_at = CURRENT_TIMESTAMP WHERE id = ?'
    execute_db(sql, [v for _, v in pairs] + [sid])
    return True


def delete_station(sid):
    """删除子站（级联软处理：置空帖子子站归属，删除成员关系）"""
    conn = get_db()
    conn.execute('UPDATE posts SET station_id = NULL WHERE station_id = ?', (sid,))
    conn.execute('DELETE FROM station_members WHERE station_id = ?', (sid,))
    conn.execute('DELETE FROM stations WHERE id = ?', (sid,))
    conn.commit()
    conn.close()


# ── Post helpers ──

ALLOWED_POST_TYPES = ('text', 'image', 'link', 'vote')


def parse_post_extra(post):
    import json as _json
    try:
        extra = _json.loads((post or {}).get('extra') or '{}')
        return extra if isinstance(extra, dict) else {}
    except (ValueError, TypeError):
        return {}


def build_post_extra(post_type, data):
    """按帖子类型构建 extra JSON（link/vote 的类型化载荷）。同步自 backend-worker。"""
    import json as _json
    data = data or {}
    extra = {}
    if post_type == 'link':
        url = str(data.get('link_url') or '').strip()[:500]
        if url and not url.lower().startswith(('javascript:', 'data:', 'vbscript:')):
            extra['link_url'] = url
    elif post_type == 'vote':
        raw = data.get('vote_options') or []
        if isinstance(raw, str):
            raw = raw.split('\n')
        options = [str(o).strip()[:100] for o in raw if str(o).strip()][:10]
        if len(options) >= 2:
            extra['options'] = options
            extra['counts'] = {}
            extra['voters'] = {}
    return extra


def shape_post(post, viewer=None):
    """展开 extra 到前端消费字段（vote_options/vote_counts/user_voted/link_url），
    并做匿名脱敏。响应形状与 Worker 版逐字段一致。"""
    if not post:
        return post
    if post.get('is_anonymous'):
        is_owner = viewer and post.get('author_id') == viewer.get('id')
        is_admin = viewer and viewer.get('role') == 'admin'
        if not is_owner and not is_admin:
            post['author_name'] = '匿名用户'
            post['author_avatar'] = '/static/images/default-avatar.svg'
    # images 列是 JSON 串：展开为数组供前端画廊渲染（兼容脏数据/单图旧字段）
    raw_images = post.get('images')
    if isinstance(raw_images, str):
        try:
            parsed = _json.loads(raw_images) if raw_images.strip() else []
        except (ValueError, TypeError):
            parsed = []
        post['images'] = [u for u in parsed if isinstance(u, str) and u] if isinstance(parsed, list) else []
    elif not isinstance(raw_images, list):
        post['images'] = []
    extra = parse_post_extra(post)
    ptype = post.get('post_type')
    if ptype == 'vote' and extra.get('options'):
        options = extra['options']
        post['vote_options'] = '\n'.join(options)
        counts = extra.get('counts') or {}
        post['vote_counts'] = {str(i): int(counts.get(str(i), 0)) for i in range(len(options))}
        voters = extra.get('voters') or {}
        post['user_voted'] = bool(viewer and str(viewer.get('id')) in voters)
    elif ptype == 'link':
        post['link_url'] = extra.get('link_url', '')
    return post


def cast_vote(post, user_id, option_index):
    """对 vote 帖投票（一人一票）。返回 (extra_dict, error_message)。"""
    import json as _json
    extra = parse_post_extra(post)
    options = extra.get('options') or []
    if not options:
        return None, '投票选项缺失'
    if not isinstance(option_index, int) or option_index < 0 or option_index >= len(options):
        return None, '选项无效'
    voters = extra.get('voters') or {}
    if str(user_id) in voters:
        return None, '你已经投过票了'
    counts = extra.get('counts') or {}
    counts[str(option_index)] = int(counts.get(str(option_index), 0)) + 1
    voters[str(user_id)] = option_index
    extra['counts'] = counts
    extra['voters'] = voters
    execute_db('UPDATE posts SET extra = ? WHERE id = ?',
               (_json.dumps(extra, ensure_ascii=False), post['id']))
    return extra, None


def create_post(title, content, author_id, station_id, image='', is_anonymous=0,
                post_type='text', images=None, extra=None, status='approved'):
    import json as _json
    pid = execute_db(
        'INSERT INTO posts (title, content, author_id, station_id, image, is_anonymous, post_type, images, extra, status) '
        'VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)',
        (title, content, author_id, station_id, image, 1 if is_anonymous else 0,
         post_type if post_type in ALLOWED_POST_TYPES else 'text',
         _json.dumps(images or [], ensure_ascii=False),
         _json.dumps(extra or {}, ensure_ascii=False),
         status if status in ('approved', 'pending') else 'approved')
    )
    if pid:
        execute_db('UPDATE stations SET post_count = post_count + 1 WHERE id = ?', (station_id,))
    return pid


def get_post_by_id(pid):
    return query_db(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  u.identity_group as author_identity_group,
                  s.name as station_name, s.icon as station_icon
           FROM posts p
           JOIN users u ON p.author_id = u.id
           JOIN stations s ON p.station_id = s.id
           WHERE p.id = ? AND p.is_deleted = 0''', (pid,), one=True)


def get_posts(station_id=None, author_id=None, limit=20, offset=0, sort='newest', post_type=None, status='approved'):
    sql = '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                    u.identity_group as author_identity_group,
                    s.name as station_name, s.icon as station_icon
             FROM posts p
             JOIN users u ON p.author_id = u.id
             JOIN stations s ON p.station_id = s.id
             WHERE p.is_deleted = 0'''
    args = []
    if status:  # 审核过滤：默认仅 approved；status=None 供管理端全量
        if author_id:
            # 本人列表（作者视角）额外放出自己的 pending/rejected 便于跟踪状态
            sql += ' AND p.status IN (?, \'pending\', \'rejected\')'
            args.append(status)
        else:
            sql += ' AND p.status = ?'
            args.append(status)
    if station_id:
        sql += ' AND p.station_id = ?'
        args.append(station_id)
    if author_id:
        sql += ' AND p.author_id = ?'
        args.append(author_id)
    if post_type:
        sql += ' AND p.post_type = ?'
        args.append(post_type)
    if sort == 'popular':
        sql += ' ORDER BY p.likes_count DESC'
    elif sort == 'comments':
        sql += ' ORDER BY p.comments_count DESC'
    else:
        sql += ' ORDER BY p.is_pinned DESC, p.created_at DESC'
    sql += ' LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)


def update_post_status(pid, status):
    """审核流转：仅允许 approved/pending/rejected 三态。"""
    if status not in ('approved', 'pending', 'rejected'):
        return False
    execute_db('UPDATE posts SET status = ? WHERE id = ?', (status, pid))
    return True


def get_post_count(station_id=None, author_id=None, post_type=None, status='approved'):
    sql = 'SELECT COUNT(*) as c FROM posts WHERE is_deleted = 0'
    args = []
    if status and not author_id:  # 本人列表可见自己的 pending（与 get_posts 一致）
        sql += ' AND status = ?'
        args.append(status)
    if station_id:
        sql += ' AND station_id = ?'
        args.append(station_id)
    if author_id:
        sql += ' AND author_id = ?'
        args.append(author_id)
    if post_type:
        sql += ' AND post_type = ?'
        args.append(post_type)
    return query_db(sql, args, one=True)['c']


def increment_views(pid):
    execute_db('UPDATE posts SET views = views + 1 WHERE id = ?', (pid,))


def get_liked_posts(user_id, limit=50, offset=0):
    """获取用户赞过的帖子（含作者/子站信息）"""
    return query_db(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  u.identity_group as author_identity_group,
                  s.name as station_name, s.icon as station_icon
           FROM likes l
           JOIN posts p ON l.target_id = p.id AND p.is_deleted = 0
           JOIN users u ON p.author_id = u.id
           JOIN stations s ON p.station_id = s.id
           WHERE l.user_id = ? AND l.target_type = 'post'
           ORDER BY l.created_at DESC LIMIT ? OFFSET ?''',
        (user_id, limit, offset))


def update_post(pid, **kwargs):
    allowed = {'title', 'content', 'image', 'is_pinned', 'post_type', 'extra', 'images', 'status'}
    fields = {k: v for k, v in kwargs.items() if k in allowed}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [pid]
    execute_db(f'UPDATE posts SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
    return True


def record_post_version(post_id, title, content, image, editor_id):
    """记录帖子编辑历史（每次编辑前保存旧版本）"""
    execute_db(
        'INSERT INTO post_versions (post_id, title, content, image, edited_by) VALUES (?,?,?,?,?)',
        (post_id, title, content, image, editor_id))


def get_post_versions(post_id, limit=20):
    return query_db(
        '''SELECT pv.*, u.username as editor_name
           FROM post_versions pv LEFT JOIN users u ON pv.edited_by = u.id
           WHERE pv.post_id = ?
           ORDER BY pv.created_at DESC LIMIT ?''',
        (post_id, limit))


def delete_post(pid):
    post = query_db('SELECT station_id FROM posts WHERE id = ?', (pid,), one=True)
    execute_db('UPDATE posts SET is_deleted = 1 WHERE id = ?', (pid,))
    if post:
        execute_db('UPDATE stations SET post_count = MAX(0, post_count - 1) WHERE id = ?', (post['station_id'],))


# ── Comment helpers ──

def create_comment(content, author_id, post_id, parent_id=None):
    cid = execute_db(
        'INSERT INTO comments (content, author_id, post_id, parent_id) VALUES (?, ?, ?, ?)',
        (content, author_id, post_id, parent_id)
    )
    if cid:
        execute_db('UPDATE posts SET comments_count = comments_count + 1 WHERE id = ?', (post_id,))
    return cid


def get_comments(post_id, limit=50, offset=0):
    return query_db(
        '''SELECT c.*, u.username as author_name, u.avatar as author_avatar
           FROM comments c JOIN users u ON c.author_id = u.id
           WHERE c.post_id = ? AND c.is_deleted = 0
           ORDER BY c.created_at ASC LIMIT ? OFFSET ?''',
        (post_id, limit, offset)
    )


def delete_comment(cid):
    comment = query_db('SELECT post_id FROM comments WHERE id = ?', (cid,), one=True)
    if comment:
        execute_db('UPDATE comments SET is_deleted = 1 WHERE id = ?', (cid,))
        execute_db('UPDATE posts SET comments_count = MAX(0, comments_count - 1) WHERE id = ?', (comment['post_id'],))


# ── Like helpers ──

def toggle_like(user_id, target_type, target_id):
    existing = query_db(
        'SELECT id FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
        (user_id, target_type, target_id), one=True
    )
    if existing:
        execute_db('DELETE FROM likes WHERE id = ?', (existing['id'],))
        if target_type == 'post':
            execute_db('UPDATE posts SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (target_id,))
        elif target_type == 'comment':
            execute_db('UPDATE comments SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (target_id,))
        return False  # unliked
    else:
        execute_db('INSERT INTO likes (user_id, target_type, target_id) VALUES (?, ?, ?)',
                    (user_id, target_type, target_id))
        if target_type == 'post':
            execute_db('UPDATE posts SET likes_count = likes_count + 1 WHERE id = ?', (target_id,))
        elif target_type == 'comment':
            execute_db('UPDATE comments SET likes_count = likes_count + 1 WHERE id = ?', (target_id,))
        return True  # liked


def is_liked(user_id, target_type, target_id):
    return query_db(
        'SELECT 1 FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
        (user_id, target_type, target_id), one=True
    ) is not None


def is_liked_batch(user_id, target_type, target_ids):
    """批量点赞状态：一条 IN 查询（列表热路径 O(N)→O(1)，同步自 backend-worker）。"""
    ids = [i for i in target_ids if i is not None]
    if not ids:
        return set()
    ph = ', '.join('?' for _ in ids)
    rows = query_db(
        'SELECT target_id FROM likes WHERE user_id = ? AND target_type = ? AND target_id IN (%s)' % ph,
        [user_id, target_type] + list(ids))
    return {r['target_id'] for r in rows}



# ── Follow helpers ──

def toggle_follow(follower_id, following_id):
    if follower_id == following_id:
        return None
    existing = query_db(
        'SELECT id FROM follows WHERE follower_id = ? AND following_id = ?',
        (follower_id, following_id), one=True
    )
    if existing:
        execute_db('DELETE FROM follows WHERE id = ?', (existing['id'],))
        return False
    else:
        execute_db('INSERT INTO follows (follower_id, following_id) VALUES (?, ?)', (follower_id, following_id))
        return True


def is_following(follower_id, following_id):
    return query_db(
        'SELECT 1 FROM follows WHERE follower_id = ? AND following_id = ?',
        (follower_id, following_id), one=True
    ) is not None


def get_followers(user_id, limit=20, offset=0):
    return query_db(
        '''SELECT u.id, u.username, u.avatar, u.bio
           FROM follows f JOIN users u ON f.follower_id = u.id
           WHERE f.following_id = ? ORDER BY f.created_at DESC LIMIT ? OFFSET ?''',
        (user_id, limit, offset)
    )


def get_following(user_id, limit=20, offset=0):
    return query_db(
        '''SELECT u.id, u.username, u.avatar, u.bio
           FROM follows f JOIN users u ON f.following_id = u.id
           WHERE f.follower_id = ? ORDER BY f.created_at DESC LIMIT ? OFFSET ?''',
        (user_id, limit, offset)
    )


# ── Notification helpers ──

def create_notification(user_id, from_user_id, ntype, content, link=''):
    execute_db(
        'INSERT INTO notifications (user_id, from_user_id, type, content, link) VALUES (?, ?, ?, ?, ?)',
        (user_id, from_user_id, ntype, content, link)
    )


def get_notifications(user_id, limit=20, offset=0, unread_only=False):
    sql = '''SELECT n.*, u.username as from_username, u.avatar as from_avatar
             FROM notifications n LEFT JOIN users u ON n.from_user_id = u.id
             WHERE n.user_id = ?'''
    args = [user_id]
    if unread_only:
        sql += ' AND n.is_read = 0'
    sql += ' ORDER BY n.created_at DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return query_db(sql, args)


def get_unread_count(user_id):
    return query_db(
        'SELECT COUNT(*) as c FROM notifications WHERE user_id = ? AND is_read = 0',
        (user_id,), one=True
    )['c']


def mark_notifications_read(user_id, notification_ids=None):
    if notification_ids:
        placeholders = ','.join('?' * len(notification_ids))
        execute_db(
            f'UPDATE notifications SET is_read = 1 WHERE user_id = ? AND id IN ({placeholders})',
            [user_id] + notification_ids
        )
    else:
        execute_db('UPDATE notifications SET is_read = 1 WHERE user_id = ?', (user_id,))
