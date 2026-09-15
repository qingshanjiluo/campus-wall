"""生成 src/bootstrap.py：把 migrations/*.sql 内嵌为 Python 模块。

Python Workers 无法在运行时读磁盘文件，schema 必须打进 bundle。
改完 migrations/0001_init.sql 后运行：

    python tools/gen_bootstrap.py
"""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SQL_FILES = ['migrations/0001_init.sql']
OUT = ROOT / 'src' / 'bootstrap.py'

TEMPLATE = '''"""自举模块（由 tools/gen_bootstrap.py 生成，请勿手改 SQL 部分）。

职责：
1. SCHEMA_SQL —— 内嵌 D1 建表语句（与 migrations/ 保持同步）
2. ensure_ready() —— 每个 isolate 首次请求时：建表（幂等）+ 空库自动播种
"""
import seed as seedmod

_READY = False

SCHEMA_SQL = \'\'\'
{schema}
\'\'\'


def _iter_statements(sql):
    """按分号拆分并剔除行注释（D1 exec 对纯注释段报 no statement）。"""
    for chunk in sql.split(';'):
        lines = [ln for ln in chunk.splitlines() if not ln.strip().startswith("--")]
        body = "\\n".join(lines).strip()
        if body:
            yield body


async def ensure_ready():
    """幂等自举：建表 + 种子。失败抛出异常由入口统一转 500。"""
    global _READY
    if _READY:
        return
    import db as _db
    for stmt in _iter_statements(SCHEMA_SQL):
        await _db.execute(stmt)
    rows = await _db.query('SELECT COUNT(*) AS c FROM users')
    c = rows[0]['c'] if rows else 0
    if not c:
        await seedmod.seed()
    _READY = True
'''


def main():
    parts = []
    for rel in SQL_FILES:
        p = ROOT / rel
        sql = p.read_text(encoding='utf-8')
        parts.append(f'-- ── {rel} ──\n{sql}')
    schema = '\n'.join(parts)
    if "'''" in schema:
        raise SystemExit('SQL 含三重引号，请改用其他转义方式')
    OUT.write_text(TEMPLATE.format(schema=schema), encoding='utf-8')
    print(f'written {OUT} ({len(schema)} chars of SQL)')


if __name__ == '__main__':
    main()
