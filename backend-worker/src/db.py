"""D1 异步数据库封装：兼容原 sqlite3 模型的调用方式。

原 models.py 中 query_db / execute_db 是同步调用，这里改为 async，
函数签名保持一致，路由层 await 即可。
"""
import context

try:
    from pyodide.ffi import jsnull
except ImportError:
    jsnull = None


def _get_db():
    return context.env.DB if context.env is not None else None


class IntegrityError(Exception):
    """唯一约束冲突（对应 sqlite3.IntegrityError）。"""


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
    await _get_db().exec(sql)


# 兼容原名，便于未来直接替换
query_db = query
execute_db = execute