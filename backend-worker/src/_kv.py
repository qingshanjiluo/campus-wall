"""Workers KV 存储后端（db.py 的 KV 分支实现）。

设计（详见 README「存储后端」一节）：
- 分片键 ``db-shard:<name>``，值为 JSON ``{"gen": int, "db": base64(整库SQL转储)}``。
- 每个分片都是一个包含全部 27 张表结构的完整 SQLite 库（只有本分片拥有的表有数据），
  使每个分片可独立保持一致。
- 每条语句先用词边界扫描出引用的表（剔除字符串字面量/注释，避免 'post' 之类取值
  误路由），按表→分片归属路由：
  * 单分片：直接在该分片连接上执行，写成功则转储回写（gen+1）。
  * 跨分片（如 posts JOIN users，或 UPDATE stations 带 station_members 子查询）：
    以第一个相关分片的连接为底，把其它分片"拥有的、被引用的"表整表导入底库
    （scratch merge），在合并库上执行；写操作再把各分片拥有的表回写回各自分片并持久化。
    这样同一语句在所有相关分片上得到一致结果，读路径也与 D1 完全一致。
- 每请求一次 begin_request()（由 bootstrap.ensure_ready 驱动）轮换 epoch：首次访问的分片
  从 KV 重新下载（比较 raw 值，未变化则复用已解析连接），实现读己之写+跨 isolate 可见。

引擎选择见 _engine.py：生产（Cloudflare Python Workers，官方文档确认自带 sqlite3）与
测试都走真 SQLite；若放置纯 Python 引擎到 vendor/puresql 则可无缝替换。
"""
import base64
import json
import re

import context
import _engine

KEY_PREFIX = 'db-shard:'

# 表 → 分片归属（与 migrations/0001_init.sql 的 27 张表一一对应）
TABLE_SHARDS = {
    # content：帖子与互动
    'posts': 'content',
    'comments': 'content',
    'likes': 'content',
    # users：users 及全部用户维度表
    'users': 'users',
    'checkins': 'users',
    'notifications': 'users',
    'follows': 'users',
    'station_members': 'users',
    'coin_transactions': 'users',
    'shop_orders': 'users',
    'favorites': 'users',
    'user_settings': 'users',
    'trade_posts': 'users',
    'romance_profiles': 'users',
    'romance_links': 'users',
    'romance_tasks': 'users',
    # stations / uploads 独立分片
    'stations': 'stations',
    'uploads': 'uploads',
    # global：其余全局表
    'identity_groups': 'global',
    'site_announcements': 'global',
    'shop_items': 'global',
    'post_versions': 'global',
    'reports': 'global',
    'gossip': 'global',
    'gossip_comments': 'global',
    'kanban_messages': 'global',
    'admin_log': 'global',
}

SHARD_ORDER = ('content', 'users', 'stations', 'uploads', 'global')

_TABLES = list(TABLE_SHARDS)
_TABLE_RE = re.compile(r'\b(' + '|'.join(sorted(_TABLES, key=len, reverse=True)) + r')\b',
                       re.IGNORECASE)
_STRING_RE = re.compile(r"'(?:[^']|'')*'")
_COMMENT_RE = re.compile(r'--[^\n]*|/\*.*?\*/', re.DOTALL)
_WRITE_RE = re.compile(r'^\s*(INSERT|UPDATE|DELETE|REPLACE)\b', re.IGNORECASE)
_DDL_RE = re.compile(
    r'^\s*(CREATE|ALTER|DROP|PRAGMA|BEGIN|COMMIT|ROLLBACK|SAVEPOINT|RELEASE|VACUUM|ANALYZE)\b',
    re.IGNORECASE)


# ── SQL 分析 ─────────────────────────────────────────────────

def referenced_tables(sql):
    """按词边界扫描语句引用的已知表（先剔除字符串与注释）。保持出现顺序。"""
    s = _COMMENT_RE.sub(' ', sql)
    s = _STRING_RE.sub("''", s)
    out, seen = [], set()
    for m in _TABLE_RE.finditer(s):
        t = m.group(1).lower()
        if t in TABLE_SHARDS and t not in seen:
            seen.add(t)
            out.append(t)
    return out


def shards_for(tables):
    owners = {TABLE_SHARDS[t] for t in tables}
    return [s for s in SHARD_ORDER if s in owners]


def _owned_by(shard, tables):
    return [t for t in tables if TABLE_SHARDS[t] == shard]


# ── 分片连接缓存（请求级 epoch） ───────────────────────────────

_cache = {}    # shard -> {'raw', 'gen', 'conn', 'epoch'}
_epoch = 0


def begin_request():
    """每个请求开始时调用：缓存过期 → 本请求首次使用某分片时从 KV 重新拉取。"""
    global _epoch
    _epoch += 1


def _reset_state():
    """测试/维护用：丢弃全部缓存（下次语句强制从 KV 重新下载，模拟全新 isolate）。"""
    global _cache, _epoch
    _cache = {}
    _epoch = 0


def shard_gens():
    """当前缓存中的各分片 gen（测试断言用）。"""
    return {name: ent.get('gen') for name, ent in _cache.items()}


def engine_name():
    return _engine.engine_name()


# ── KV 原语 ──────────────────────────────────────────────────

def _kv():
    env = context.env
    kv = getattr(env, 'KV', None) if env is not None else None
    if kv is None:
        raise RuntimeError('KV 绑定不存在')
    return kv


async def _kv_get(key):
    val = await _kv().get(key)
    if val is None:
        return None
    if isinstance(val, (bytes, bytearray)):
        return bytes(val)
    # workers-py 下 get() 返回 JsString(str 子类)；未知代理类型统一 str()
    return val if isinstance(val, str) else str(val)


async def _kv_put(key, value):
    await _kv().put(key, value)


# ── 分片装载 / 持久化 ────────────────────────────────────────

def _fresh_conn():
    import bootstrap
    conn = _engine.connect()
    conn.executescript(bootstrap.SCHEMA_SQL)
    return conn


async def _load_shard(name):
    ent = _cache.get(name)
    if ent is not None and ent.get('epoch') == _epoch:
        return ent
    raw = await _kv_get(KEY_PREFIX + name)
    if ent is not None and raw is not None and ent.get('raw') == raw and ent.get('conn') is not None:
        ent['epoch'] = _epoch
        return ent
    conn, gen = None, 0
    if raw is not None:
        try:
            payload = json.loads(raw)
            conn = _engine.loads(payload['db'])
            gen = int(payload.get('gen', 0))
        except Exception:
            conn = None  # 数据损坏 → 自愈为空分片（后续写会覆盖）
    created = conn is None
    if created:
        conn = _fresh_conn()
    ent = {'raw': raw, 'gen': 0 if created else gen, 'conn': conn, 'epoch': _epoch}
    _cache[name] = ent
    if created:
        await _persist(name)
    return ent


async def _persist(name):
    ent = _cache[name]
    gen = int(ent.get('gen') or 0) + 1
    script = _engine.dumps(ent['conn'])
    payload = json.dumps({'gen': gen,
                          'db': base64.b64encode(script.encode('utf-8')).decode('ascii')})
    await _kv_put(KEY_PREFIX + name, payload)
    ent['gen'] = gen
    ent['raw'] = payload


# ── 分片间表搬运（scratch merge / 回写） ─────────────────────

def _sync_seq(dst, src, tables):
    """单调提升 dst 的 sqlite_sequence（跨分片写需要新 rowid 时避免撞主键）。"""
    for t in tables:
        cur = src.execute('SELECT seq FROM sqlite_sequence WHERE name = ?', (t,))
        row = cur.fetchone()
        cur.close() if hasattr(cur, 'close') else None
        if not row:
            continue
        seq = row[0]
        cur = dst.execute('SELECT seq FROM sqlite_sequence WHERE name = ?', (t,))
        drow = cur.fetchone()
        cur.close() if hasattr(cur, 'close') else None
        if drow is None:
            dst.execute('INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)', (t, seq))
        elif seq > drow[0]:
            dst.execute('UPDATE sqlite_sequence SET seq = ? WHERE name = ?', (seq, t))


def _copy_tables(dst, src, tables):
    for t in tables:
        cur = src.execute('SELECT * FROM %s' % t)
        cols = [d[0] for d in cur.description] if cur.description else []
        rows = cur.fetchall() if cols else []
        cur.close() if hasattr(cur, 'close') else None
        dst.execute('DELETE FROM %s' % t)
        if rows:
            sql = 'INSERT INTO %s (%s) VALUES (%s)' % (
                t, ', '.join(cols), ', '.join('?' for _ in cols))
            dst.executemany(sql, [tuple(r) for r in rows])
    _sync_seq(dst, src, tables)


async def _prepare(sql, tables):
    """装载相关分片并在 base 连接上完成跨分片合并；返回 (base_ent, owners)。"""
    owners = shards_for(tables)
    base = await _load_shard(owners[0])
    if len(owners) > 1:
        for s in owners[1:]:
            ts = _owned_by(s, tables)
            if not ts:
                continue
            ent = await _load_shard(s)
            _copy_tables(base['conn'], ent['conn'], ts)
    return base, owners


def _raise_integrity(exc):
    import db as _db
    if _engine.IntegrityError(exc) is not None:
        raise _db.IntegrityError(str(exc))
    msg = str(exc)
    if 'UNIQUE constraint failed' in msg or 'FOREIGN KEY constraint failed' in msg \
            or 'NOT NULL constraint failed' in msg:
        raise _db.IntegrityError(msg)
    raise exc


# ── 对外三个执行入口（签名与 D1 分支一致） ───────────────────

async def query(sql, args=(), one=False):
    tables = referenced_tables(sql)
    if not tables:
        conn = _engine.connect()
        cur = conn.execute(sql, tuple(args))
        cols = [d[0] for d in cur.description] if cur.description else []
        rows = [dict(zip(cols, r)) for r in cur.fetchall()]
        return (rows[0] if rows else None) if one else rows
    base, _owners = await _prepare(sql, tables)
    cur = base['conn'].execute(sql, tuple(args))
    cols = [d[0] for d in cur.description] if cur.description else []
    rows = [dict(zip(cols, r)) for r in cur.fetchall()]
    if one:
        return rows[0] if rows else None
    return rows


async def execute(sql, args=()):
    if _DDL_RE.match(sql):
        return await _ddl(sql)
    tables = referenced_tables(sql)
    if not tables:
        conn = _engine.connect()
        cur = conn.execute(sql, tuple(args))
        return (cur.lastrowid or 0) if _WRITE_RE.match(sql) else 0
    base, owners = await _prepare(sql, tables)
    try:
        cur = base['conn'].execute(sql, tuple(args))
    except Exception as e:
        _raise_integrity(e)
        raise e
    if not _WRITE_RE.match(sql):
        return 0
    n = cur.rowcount
    if n and n > 0:
        await _persist(owners[0])
        for s in owners[1:]:
            ts = _owned_by(s, tables)
            if not ts:
                continue
            ent = _cache[s]
            _copy_tables(ent['conn'], base['conn'], ts)
            await _persist(s)
    return cur.lastrowid or 0


async def _ddl(sql):
    """DDL：广播到所有分片，schema 变化的分片回写（bootstrap 每 isolate 首请求触发）。"""
    for name in SHARD_ORDER:
        ent = await _load_shard(name)
        before = _engine.dumps(ent['conn'])
        try:
            ent['conn'].execute(sql)
        except Exception as e:
            _raise_integrity(e)
            raise e
        after = _engine.dumps(ent['conn'])
        if after != before:
            await _persist(name)
    return 0


async def execute_script(sql):
    import bootstrap
    for stmt in bootstrap._iter_statements(sql):
        await execute(stmt)
    return None
