"""基础数据模型层 —— 移植自 app/models.py，改为异步 D1 调用。"""
import json
from datetime import datetime, timedelta, timezone

import db
from auth import hash_password, verify_password
from db import IntegrityError


async def init_db():
    await db.execute_script(INIT_SQL)


INIT_SQL = """-- 由 migrations/0001_init.sql 提供完整建表，此处占位避免误用
"""


# ── User helpers ──

async def create_user(username, email, password):
    pw_hash = hash_password(password)
    try:
        return await db.execute(
            'INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)',
            (username, email, pw_hash))
    except IntegrityError:
        return None


async def get_user_by_username(username):
    return await db.query('SELECT * FROM users WHERE username = ?', (username,), one=True)


async def get_user_by_email(email):
    return await db.query('SELECT * FROM users WHERE email = ?', (email,), one=True)


async def get_user_by_id(uid):
    return await db.query('SELECT * FROM users WHERE id = ?', (uid,), one=True)


async def change_password(uid, new_password):
    pw_hash = hash_password(new_password)
    await db.execute('UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?', (pw_hash, uid))


async def update_user(uid, **kwargs):
    allowed = {'username', 'email', 'avatar', 'bio', 'mood', 'title'}
    fields = {k: v for k, v in kwargs.items() if k in allowed and v is not None}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [uid]
    try:
        await db.execute(f'UPDATE users SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
        return True
    except IntegrityError:
        return False


async def get_user_stats(uid):
    posts = (await db.query('SELECT COUNT(*) as c FROM posts WHERE author_id = ?', (uid,), one=True))['c']
    followers = (await db.query('SELECT COUNT(*) as c FROM follows WHERE following_id = ?', (uid,), one=True))['c']
    following = (await db.query('SELECT COUNT(*) as c FROM follows WHERE follower_id = ?', (uid,), one=True))['c']
    return {'posts': posts, 'followers': followers, 'following': following}


def public_user(user):
    """对外暴露的用户信息（去掉敏感字段）。"""
    if not user:
        return None
    return {
        'id': user['id'], 'username': user['username'], 'email': user.get('email', ''),
        'avatar': user.get('avatar', ''), 'bio': user.get('bio', ''),
        'role': user.get('role', 'user'), 'coins': user.get('coins', 0),
        'points': user.get('points', 0), 'level': user.get('level', 1),
        'exp': user.get('exp', 0), 'checkin_streak': user.get('checkin_streak', 0),
        'identity_group': user.get('identity_group', ''), 'title': user.get('title', ''),
        'mood': user.get('mood', ''), 'created_at': user.get('created_at', ''),
    }


# ── Station helpers ──

async def create_station(name, description, icon, tags, owner_id, category=''):
    tags_str = json.dumps(tags, ensure_ascii=False) if isinstance(tags, list) else tags
    sid = await db.execute(
        'INSERT INTO stations (name, description, icon, tags, owner_id, category) VALUES (?, ?, ?, ?, ?, ?)',
        (name, description, icon or 'school', tags_str, owner_id, category))
    if sid:
        await db.execute('INSERT INTO station_members (user_id, station_id, role) VALUES (?, ?, ?)',
                         (owner_id, sid, 'owner'))
        await db.execute('UPDATE stations SET user_count = 1 WHERE id = ?', (sid,))
    return sid


async def get_stations(limit=20, offset=0, sort='newest', tag=None, category=None):
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
    return await db.query(sql, args)


async def get_station_by_id(sid):
    return await db.query('SELECT * FROM stations WHERE id = ?', (sid,), one=True)


async def get_station_categories():
    rows = await db.query("SELECT DISTINCT category FROM stations WHERE category != '' ORDER BY category")
    cats = []
    for r in rows:
        parts = r['category'].split('/')
        cats.append({
            'path': r['category'], 'parent': parts[0] if len(parts) > 1 else '',
            'name': parts[-1], 'depth': len(parts) - 1,
        })
    return cats


async def search_stations(keyword):
    return await db.query(
        'SELECT * FROM stations WHERE name LIKE ? OR description LIKE ? ORDER BY user_count DESC',
        (f'%{keyword}%', f'%{keyword}%'))


async def join_station(user_id, station_id):
    try:
        await db.execute('INSERT INTO station_members (user_id, station_id) VALUES (?, ?)', (user_id, station_id))
        await db.execute('UPDATE stations SET user_count = user_count + 1 WHERE id = ?', (station_id,))
        return True
    except IntegrityError:
        return False


async def leave_station(user_id, station_id):
    await db.execute('DELETE FROM station_members WHERE user_id = ? AND station_id = ?', (user_id, station_id))
    await db.execute('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?', (station_id,))


async def get_station_membership(user_id, station_id):
    """单查询同时回答「是否成员 + 什么角色」（None = 非成员）。

    替代旧的 is_station_member + get_station_member_role 两次扫描同一行，
    子站详情热路径的 D1 行数减半。
    """
    row = await db.query('SELECT role FROM station_members WHERE user_id = ? AND station_id = ?',
                         (user_id, station_id), one=True)
    return row['role'] if row else None


async def get_station_memberships(user_id, station_ids):
    """批量成员关系：{station_id: role}，供子站列表一次取回全部行。"""
    ids = [i for i in station_ids if i is not None]
    if not ids:
        return {}
    ph = ', '.join('?' for _ in ids)
    rows = await db.query(
        'SELECT station_id, role FROM station_members WHERE user_id = ? AND station_id IN (%s)' % ph,
        [user_id] + list(ids))
    return {r['station_id']: r['role'] for r in rows}


async def is_station_member(user_id, station_id):
    return (await get_station_membership(user_id, station_id)) is not None


async def get_station_member_role(user_id, station_id):
    return await get_station_membership(user_id, station_id)


async def get_station_members(station_id):
    return await db.query(
        '''SELECT sm.role, sm.joined_at, u.id as user_id, u.username, u.avatar, u.bio
           FROM station_members sm JOIN users u ON sm.user_id = u.id
           WHERE sm.station_id = ?
           ORDER BY (sm.role = 'owner') DESC, sm.joined_at ASC''', (station_id,))


async def get_user_stations(user_id):
    return await db.query(
        '''SELECT s.*, sm.role, sm.joined_at
           FROM station_members sm JOIN stations s ON sm.station_id = s.id
           WHERE sm.user_id = ?
           ORDER BY sm.joined_at DESC''', (user_id,))


async def get_station_stats(sid, days=7):
    """子站发帖统计：一条 GROUP BY 取代 days+1 次 COUNT（响应形状不变）。"""
    days = max(1, min(30, int(days)))
    rows = await db.query(
        "SELECT date(created_at) AS d, COUNT(*) AS c FROM posts "
        "WHERE station_id = ? AND is_deleted = 0 AND created_at >= date('now', ?) "
        "GROUP BY d", (sid, '-%d days' % days))
    by_day = {r['d']: r['c'] for r in rows}
    labels, counts = [], []
    for i in range(days - 1, -1, -1):
        d = (datetime.now() - timedelta(days=i)).strftime('%Y-%m-%d')
        labels.append(d[5:])
        counts.append(by_day.get(d, 0))
    today = by_day.get(datetime.now(timezone.utc).strftime('%Y-%m-%d'), 0)
    return {'labels': labels, 'posts': counts, 'today_posts': today}


async def remove_station_member(user_id, station_id):
    await db.execute('DELETE FROM station_members WHERE user_id = ? AND station_id = ? AND role != ?',
                     (user_id, station_id, 'owner'))
    await db.execute('UPDATE stations SET user_count = MAX(0, user_count - 1) WHERE id = ?', (station_id,))


async def transfer_station_ownership(sid, new_owner_id):
    await db.execute('UPDATE station_members SET role = ? WHERE station_id = ? AND role = ?',
                     ('member', sid, 'owner'))
    await db.execute('INSERT OR IGNORE INTO station_members (user_id, station_id, role) VALUES (?, ?, ?)',
                     (new_owner_id, sid, 'member'))
    await db.execute('UPDATE station_members SET role = ? WHERE user_id = ? AND station_id = ?',
                     ('owner', new_owner_id, sid))
    await db.execute('UPDATE stations SET owner_id = ? WHERE id = ?', (new_owner_id, sid))


async def update_station(sid, **fields):
    allowed = ('name', 'description', 'icon', 'tags', 'is_public', 'cover', 'only_owner_posts', 'announcement', 'category')
    pairs = [(k, fields[k]) for k in allowed if k in fields]
    if not pairs:
        return False
    sql = 'UPDATE stations SET ' + ', '.join(f'{k} = ?' for k, _ in pairs) + ', updated_at = CURRENT_TIMESTAMP WHERE id = ?'
    await db.execute(sql, [v for _, v in pairs] + [sid])
    return True


async def delete_station(sid):
    await db.execute('UPDATE posts SET station_id = NULL WHERE station_id = ?', (sid,))
    await db.execute('DELETE FROM station_members WHERE station_id = ?', (sid,))
    await db.execute('DELETE FROM stations WHERE id = ?', (sid,))


# ── Post helpers ──

ALLOWED_POST_TYPES = ('text', 'image', 'link', 'vote')


def parse_post_extra(post):
    try:
        extra = json.loads((post or {}).get('extra') or '{}')
        return extra if isinstance(extra, dict) else {}
    except (ValueError, TypeError):
        return {}


def build_post_extra(post_type, data):
    """按帖子类型构建 extra JSON（link/vote 的类型化载荷）。"""
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
    """展开 extra 到前端消费字段：vote_options(换行串)/vote_counts(索引为键)/user_voted/link_url。
    同时做匿名帖脱敏（与 _anonymize_post 合并，供多路由复用）。"""
    if not post:
        return post
    # 匿名脱敏
    if post.get('is_anonymous'):
        is_owner = viewer and post.get('author_id') == viewer.get('id')
        is_admin = viewer and viewer.get('role') == 'admin'
        if not is_owner and not is_admin:
            post['author_name'] = '匿名用户'
            post['author_avatar'] = '/static/images/default-avatar.svg'
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


async def create_post(title, content, author_id, station_id, image='', is_anonymous=0, post_type='text', images=None, extra=None):
    pid = await db.execute(
        'INSERT INTO posts (title, content, author_id, station_id, image, is_anonymous, post_type, images, extra) '
        'VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)',
        (title, content, author_id, station_id, image, 1 if is_anonymous else 0,
         post_type, json.dumps(images or [], ensure_ascii=False),
         json.dumps(extra or {}, ensure_ascii=False)))
    if pid:
        await db.execute('UPDATE stations SET post_count = post_count + 1 WHERE id = ?', (station_id,))
    return pid


async def get_post_by_id(pid):
    return await db.query(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  u.identity_group as author_identity_group,
                  s.name as station_name, s.icon as station_icon
           FROM posts p
           JOIN users u ON p.author_id = u.id
           LEFT JOIN stations s ON p.station_id = s.id
           WHERE p.id = ? AND p.is_deleted = 0''', (pid,), one=True)


POST_SELECT = '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                    u.identity_group as author_identity_group,
                    s.name as station_name, s.icon as station_icon
             FROM posts p
             JOIN users u ON p.author_id = u.id
             LEFT JOIN stations s ON p.station_id = s.id
             WHERE p.is_deleted = 0'''


async def get_posts(station_id=None, author_id=None, limit=20, offset=0, sort='newest', post_type=None):
    sql = POST_SELECT
    args = []
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
    return await db.query(sql, args)


async def get_post_count(station_id=None, author_id=None, post_type=None):
    sql = 'SELECT COUNT(*) as c FROM posts WHERE is_deleted = 0'
    args = []
    if station_id:
        sql += ' AND station_id = ?'
        args.append(station_id)
    if author_id:
        sql += ' AND author_id = ?'
        args.append(author_id)
    if post_type:
        sql += ' AND post_type = ?'
        args.append(post_type)
    return (await db.query(sql, args, one=True))['c']


async def increment_views(pid):
    await db.execute('UPDATE posts SET views = views + 1 WHERE id = ?', (pid,))


async def get_liked_posts(user_id, limit=50, offset=0):
    return await db.query(
        '''SELECT p.*, u.username as author_name, u.avatar as author_avatar,
                  u.identity_group as author_identity_group,
                  s.name as station_name, s.icon as station_icon
           FROM likes l
           JOIN posts p ON l.target_id = p.id AND p.is_deleted = 0
           JOIN users u ON p.author_id = u.id
           LEFT JOIN stations s ON p.station_id = s.id
           WHERE l.user_id = ? AND l.target_type = 'post'
           ORDER BY l.created_at DESC LIMIT ? OFFSET ?''', (user_id, limit, offset))


async def update_post(pid, **kwargs):
    allowed = {'title', 'content', 'image', 'is_pinned', 'extra'}
    fields = {k: v for k, v in kwargs.items() if k in allowed}
    if not fields:
        return False
    sets = ', '.join(f'{k} = ?' for k in fields)
    vals = list(fields.values()) + [pid]
    await db.execute(f'UPDATE posts SET {sets}, updated_at = CURRENT_TIMESTAMP WHERE id = ?', vals)
    return True


async def record_post_version(post_id, title, content, image, editor_id):
    await db.execute('INSERT INTO post_versions (post_id, title, content, image, edited_by) VALUES (?,?,?,?,?)',
                     (post_id, title, content, image, editor_id))


async def get_post_versions(post_id, limit=20):
    return await db.query(
        '''SELECT pv.*, u.username as editor_name
           FROM post_versions pv LEFT JOIN users u ON pv.edited_by = u.id
           WHERE pv.post_id = ?
           ORDER BY pv.created_at DESC LIMIT ?''', (post_id, limit))


async def delete_post(pid):
    post = await db.query('SELECT station_id FROM posts WHERE id = ?', (pid,), one=True)
    await db.execute('UPDATE posts SET is_deleted = 1 WHERE id = ?', (pid,))
    if post:
        await db.execute('UPDATE stations SET post_count = MAX(0, post_count - 1) WHERE id = ?',
                         (post['station_id'],))


# ── Comment helpers ──

async def create_comment(content, author_id, post_id, parent_id=None):
    cid = await db.execute(
        'INSERT INTO comments (content, author_id, post_id, parent_id) VALUES (?, ?, ?, ?)',
        (content, author_id, post_id, parent_id))
    if cid:
        await db.execute('UPDATE posts SET comments_count = comments_count + 1 WHERE id = ?', (post_id,))
    return cid


async def get_comments(post_id, limit=50, offset=0):
    return await db.query(
        '''SELECT c.*, u.username as author_name, u.avatar as author_avatar
           FROM comments c JOIN users u ON c.author_id = u.id
           WHERE c.post_id = ? AND c.is_deleted = 0
           ORDER BY c.created_at ASC LIMIT ? OFFSET ?''', (post_id, limit, offset))


async def delete_comment(cid):
    comment = await db.query('SELECT post_id FROM comments WHERE id = ?', (cid,), one=True)
    if comment:
        await db.execute('UPDATE comments SET is_deleted = 1 WHERE id = ?', (cid,))
        await db.execute('UPDATE posts SET comments_count = MAX(0, comments_count - 1) WHERE id = ?',
                         (comment['post_id'],))


# ── Like helpers ──

async def toggle_like(user_id, target_type, target_id):
    existing = await db.query('SELECT id FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
                              (user_id, target_type, target_id), one=True)
    if existing:
        await db.execute('DELETE FROM likes WHERE id = ?', (existing['id'],))
        if target_type == 'post':
            await db.execute('UPDATE posts SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (target_id,))
        elif target_type == 'comment':
            await db.execute('UPDATE comments SET likes_count = MAX(0, likes_count - 1) WHERE id = ?', (target_id,))
        return False
    else:
        await db.execute('INSERT INTO likes (user_id, target_type, target_id) VALUES (?, ?, ?)',
                         (user_id, target_type, target_id))
        if target_type == 'post':
            await db.execute('UPDATE posts SET likes_count = likes_count + 1 WHERE id = ?', (target_id,))
        elif target_type == 'comment':
            await db.execute('UPDATE comments SET likes_count = likes_count + 1 WHERE id = ?', (target_id,))
        return True


async def is_liked(user_id, target_type, target_id):
    return (await db.query('SELECT 1 FROM likes WHERE user_id = ? AND target_type = ? AND target_id = ?',
                           (user_id, target_type, target_id), one=True)) is not None


# ── Follow helpers ──

async def toggle_follow(follower_id, following_id):
    if follower_id == following_id:
        return None
    existing = await db.query('SELECT id FROM follows WHERE follower_id = ? AND following_id = ?',
                              (follower_id, following_id), one=True)
    if existing:
        await db.execute('DELETE FROM follows WHERE id = ?', (existing['id'],))
        return False
    else:
        await db.execute('INSERT INTO follows (follower_id, following_id) VALUES (?, ?)',
                         (follower_id, following_id))
        return True


async def is_following(follower_id, following_id):
    return (await db.query('SELECT 1 FROM follows WHERE follower_id = ? AND following_id = ?',
                           (follower_id, following_id), one=True)) is not None


async def get_followers(user_id, limit=20, offset=0):
    return await db.query(
        '''SELECT u.id, u.username, u.avatar, u.bio
           FROM follows f JOIN users u ON f.follower_id = u.id
           WHERE f.following_id = ? ORDER BY f.created_at DESC LIMIT ? OFFSET ?''', (user_id, limit, offset))


async def get_following(user_id, limit=20, offset=0):
    return await db.query(
        '''SELECT u.id, u.username, u.avatar, u.bio
           FROM follows f JOIN users u ON f.following_id = u.id
           WHERE f.follower_id = ? ORDER BY f.created_at DESC LIMIT ? OFFSET ?''', (user_id, limit, offset))


# ── Notification helpers ──

async def create_notification(user_id, from_user_id, ntype, content, link=''):
    if not user_id:
        return
    await db.execute(
        'INSERT INTO notifications (user_id, from_user_id, type, content, link) VALUES (?, ?, ?, ?, ?)',
        (user_id, from_user_id, ntype, content, link))


async def get_notifications(user_id, limit=20, offset=0, unread_only=False):
    sql = '''SELECT n.*, u.username as from_username, u.avatar as from_avatar
             FROM notifications n LEFT JOIN users u ON n.from_user_id = u.id
             WHERE n.user_id = ?'''
    args = [user_id]
    if unread_only:
        sql += ' AND n.is_read = 0'
    sql += ' ORDER BY n.created_at DESC LIMIT ? OFFSET ?'
    args += [limit, offset]
    return await db.query(sql, args)


async def get_unread_count(user_id):
    return (await db.query('SELECT COUNT(*) as c FROM notifications WHERE user_id = ? AND is_read = 0',
                           (user_id,), one=True))['c']


async def mark_notifications_read(user_id, notification_ids=None):
    if notification_ids:
        placeholders = ','.join('?' * len(notification_ids))
        await db.execute(f'UPDATE notifications SET is_read = 1 WHERE user_id = ? AND id IN ({placeholders})',
                         [user_id] + notification_ids)
    else:
        await db.execute('UPDATE notifications SET is_read = 1 WHERE user_id = ?', (user_id,))