"""异步数据库封装：双后端（Cloudflare D1 / Workers KV），兼容原 sqlite3 模型的调用方式。

原 models.py 中 query_db / execute_db 是同步调用，这里改为 async，
函数签名保持一致，路由层 await 即可。

后端选择（每个请求按 env 动态判定）：
- env.DB 存在 → D1 模式（代码路径与旧版完全一致）；
- 否则 env.KV 存在 → KV 模式（分片快照 + 嵌入式 SQLite，见 _kv.py / README）；
- 都没有 → 走 D1 分支（与旧行为一致，缺绑定时报错交由 entry 转 500）。
"""
import context

try:
    from pyodide.ffi import jsnull
except ImportError:
    jsnull = None


class IntegrityError(Exception):
    """唯一约束冲突（对应 sqlite3.IntegrityError）。"""


def _get_db():
    return context.env.DB if context.env is not None else None


def _env_binding(name):
    env = context.env
    return getattr(env, name, None) if env is not None else None


def backend():
    """'d1' | 'kv' | None。D1 优先：只要 DB 绑定存在就用 D1。"""
    if _env_binding('DB') is not None:
        return 'd1'
    if _env_binding('KV') is not None:
        return 'kv'
    return None


def _is_kv():
    return backend() == 'kv'


def begin_request():
    """每个请求入口调用（bootstrap.ensure_ready 驱动）。KV 模式轮换分片缓存 epoch，
    使本请求首次使用的分片从 KV 重新拉取；D1 模式为空操作。"""
    if _is_kv():
        import _kv
        _kv.begin_request()


def kv_shard_gens():
    """KV 模式下的 {shard: gen}（测试/排障用）。"""
    import _kv
    return _kv.shard_gens()


def _kv_reset_state():
    """测试用：丢弃 KV 分片缓存，模拟全新 isolate。"""
    import _kv
    _kv._reset_state()


def _check_integrity(exc):
    msg = str(exc)
    if "UNIQUE constraint failed" in msg or "FOREIGN KEY constraint failed" in msg:
        raise IntegrityError(msg)
    raise exc


def _row_to_dict(row):
    if row is None:
        return None
    to_py = getattr(row, "to_py", None)
    if to_py is not None:
        return to_py()
    return dict(row)


def _bind_args(stmt, args):
    """将 Python None 映射为 SQL NULL（jsnull），避免 D1 报 undefined。"""
    if jsnull is None:
        return stmt.bind(*args)
    return stmt.bind(*[jsnull if a is None else a for a in args])


async def query(sql, args=(), one=False):
    """执行查询，one=True 返回单行 dict/None，否则返回 list[dict]。"""
    if _is_kv():
        import _kv
        return await _kv.query(sql, args, one)
    stmt = _get_db().prepare(sql)
    if args:
        stmt = _bind_args(stmt, args)
    res = await stmt.all()
    rows = res.results if res is not None else []
    if one:
        return _row_to_dict(rows[0]) if rows else None
    return [_row_to_dict(r) for r in rows]


async def execute(sql, args=()):
    """执行写操作，返回 last_row_id（或 0）。"""
    if _is_kv():
        import _kv
        return await _kv.execute(sql, args)
    stmt = _get_db().prepare(sql)
    if args:
        stmt = _bind_args(stmt, args)
    try:
        res = await stmt.run()
    except Exception as e:
        _check_integrity(e)
        raise e
    if res is None:
        return 0
    meta = res.get("meta") if hasattr(res, "get") else getattr(res, "meta", None)
    if not meta:
        return 0
    if hasattr(meta, "get"):
        return meta.get("last_row_id") or 0
    return getattr(meta, "last_row_id", 0) or 0


async def execute_script(sql):
    """执行多条 SQL 语句（用于初始化/迁移）。必须 await，否则 D1 不保证完成。"""
    if _is_kv():
        import _kv
        return await _kv.execute_script(sql)
    await _get_db().exec(sql)


# 兼容原名，便于未来直接替换
query_db = query
execute_db = execute
