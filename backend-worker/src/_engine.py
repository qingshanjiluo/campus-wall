"""KV 模式的 SQL 引擎选择层（被 db.py 使用）。

优先级（任务规定）：stdlib sqlite3 → vendored puresql。

- CPython（native tests / tools）：sqlite3 恒可用。
- Cloudflare Python Workers：官方 stdlib 文档（developers.cloudflare.com/workers/
  languages/python/stdlib/）明确 Python Workers 携带完整标准库，排除清单中
  不含 sqlite3（排除项为 curses/dbm/…/tkinter 等）。因此生产 KV 模式同样使用
  真正的 SQLite 语义（Pyodide 内 WASM 版 libsqlite3）。
- vendor/puresql：任务推荐的 `puresql` 包在 PyPI 上并不存在（2026-08 验证：
  pip download / pypi.org/simple 均 404；pysqlite3/fakesql/minisql 等替代要么
  是 C 扩展要么无法安装）。这里保留装载缝隙：只要把任何 sqlite3 兼容的纯 Python
  引擎放到 backend-worker/vendor/puresql/（模块级暴露 connect/IntegrityError），
  即可在 sqlite3 缺失的环境里自动接管，测试模式 'prod' 也会随之启用。
"""
import os
import sys

_VENDOR = None
_mod = None
_name = None

FORCE_ENGINE = (os.environ.get('CW_SQL_ENGINE') or 'auto').strip().lower()


def _vendor_dir():
    here = os.path.dirname(os.path.abspath(__file__))
    return os.path.join(os.path.dirname(here), 'vendor')


def _load():
    global _mod, _name
    if _mod is not None:
        return _mod
    if FORCE_ENGINE != 'puresql':
        try:
            import sqlite3
            _mod, _name = sqlite3, 'sqlite3'
            return _mod
        except ImportError:
            pass
    vd = _vendor_dir()
    if vd not in sys.path:
        sys.path.append(vd)
    try:
        import puresql
        _mod, _name = puresql, 'puresql'
        return _mod
    except ImportError:
        pass
    raise ImportError(
        'no SQL engine available: stdlib sqlite3 import failed and no vendored '
        'puresql found in %s (see README storage section)' % vd)


def engine_name():
    _load()
    return _name


def available():
    """True if some engine can be loaded (bootstrap/KV gate)."""
    try:
        _load()
        return True
    except ImportError:
        return False


def connect():
    """New empty in-memory database connection."""
    eng = _load()
    try:
        return eng.connect(':memory:')
    except TypeError:  # pragma: no cover - non-sqlite engines
        return eng.connect()


def IntegrityError(exc):
    """Engine-specific IntegrityError class (matches by name for fallbacks)."""
    eng = _load()
    cls = getattr(eng, 'IntegrityError', None)
    if cls is None:
        return None
    return cls if isinstance(exc, cls) else None


def dumps(conn):
    """Serialize a connection's DB to a portable SQL script (sqlite iterdump format)."""
    try:
        return '\n'.join(conn.iterdump())
    except AttributeError:  # pragma: no cover - alt engines
        return conn.dump()


def loads(b64_text):
    """Rebuild a connection from dumps() output (base64 of UTF-8 SQL script)."""
    import base64
    script = base64.b64decode(b64_text.encode('ascii')).decode('utf-8')
    conn = connect()
    conn.executescript(script)
    return conn
