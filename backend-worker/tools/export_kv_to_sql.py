"""KV 分片 → SQL 数据导出：为「KV 上线 → D1 升级」路径准备数据迁移文件。

用法：
    python tools/export_kv_to_sql.py                      # 打印到 stdout
    python tools/export_kv_to_sql.py -o migrations/9001_data_from_kv.sql

工作原理：把每个 db-shard:<name> 的 base64 转储装载回 SQLite，按"拥有表"取权威行，
合并进一个全 schema 库，然后输出 27 张表的 DELETE+INSERT 脚本 + sqlite_sequence 对齐，
可直接 `wrangler d1 execute ... --file` 进 D1。

数据快照来源：
- 测试：KVShim.snapshot()（harness 直接 import 本模块调用）。
- 生产：`wrangler kv key get db-shard:users --namespace-id <id> --remote` 逐个取回后
  组装成 {key: value-json-string} dict 传给 export_script()。
"""
import argparse
import base64
import json
import os
import sqlite3
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
sys.path.insert(0, os.path.join(ROOT, 'src'))

import _kv            # noqa: E402  (pure imports: context/_engine — safe standalone)
import bootstrap      # noqa: E402  (SCHEMA_SQL)


def _lit(v):
    if v is None:
        return 'NULL'
    if isinstance(v, bool):
        return '1' if v else '0'
    if isinstance(v, int):
        return str(v)
    if isinstance(v, float):
        return repr(v)
    if isinstance(v, (bytes, bytearray)):
        return "X'%s'" % bytes(v).hex()
    return "'" + str(v).replace("'", "''") + "'"


def _load_store(path):
    """--store-json：{"db-shard:users": "<payload json string>", ...} 的快照文件。"""
    with open(path, 'r', encoding='utf-8') as f:
        data = json.load(f)
    return {str(k): (v if isinstance(v, str) else json.dumps(v)) for k, v in data.items()}


def export_script(snapshot):
    """snapshot: {key: value} （仅含 str value）→ 完整可执行的 INSERT 迁移脚本。"""
    prefix = _kv.KEY_PREFIX
    names = sorted(k[len(prefix):] for k in snapshot if k.startswith(prefix))
    if not names:
        raise SystemExit('快照中没有任何 %s* 分片键' % prefix)

    master = sqlite3.connect(':memory:')
    master.executescript(bootstrap.SCHEMA_SQL)
    for name in names:
        payload = json.loads(snapshot[prefix + name])
        script = base64.b64decode(payload['db'].encode('ascii')).decode('utf-8')
        conn = sqlite3.connect(':memory:')
        conn.executescript(script)
        conn.row_factory = sqlite3.Row
        owned = [t for t, s in _kv.TABLE_SHARDS.items() if s == name]
        for t in sorted(owned):
            rows = conn.execute('SELECT * FROM "%s"' % t).fetchall()
            master.execute('DELETE FROM "%s"' % t)
            if rows:
                cols = list(rows[0].keys())
                collist = ', '.join('"%s"' % c for c in cols)
                ph = ', '.join('?' * len(cols))
                master.executemany('INSERT INTO "%s" (%s) VALUES (%s)' % (t, collist, ph),
                                   [tuple(r) for r in rows])
        conn.close()

    out = [
        '-- 9001_data_from_kv.sql —— 由 tools/export_kv_to_sql.py 生成',
        '-- 来源：Workers KV 分片快照（%s）' % ', '.join(names),
        '-- 用法：wrangler d1 execute campus-wall-db --remote --file=migrations/9001_data_from_kv.sql',
        '-- 前置：目标库已应用 0001_init.sql。脚本可重复执行（DELETE+INSERT）。',
        '',
    ]
    tables = sorted(_kv.TABLE_SHARDS)
    imported = []
    for t in tables:
        rows = master.execute('SELECT * FROM "%s"' % t).fetchall()
        cols = master.execute('PRAGMA table_info("%s")' % t)
        colnames = [r[1] for r in cols]
        out.append('-- %s: %d 行' % (t, len(rows)))
        out.append('DELETE FROM "%s";' % t)
        collist = ', '.join('"%s"' % c for c in colnames)
        for r in rows:
            out.append('INSERT INTO "%s" (%s) VALUES (%s);' % (
                t, collist, ', '.join(_lit(v) for v in r)))
        out.append('')
        if rows:
            imported.append(t)
    # 仅为确实导入过行的表对齐序列：显式 id 的 INSERT 会创建/推进 sqlite_sequence，
    # 全空时 sqlite_sequence 表可能尚不存在（发出语句会报 no such table）。
    if imported:
        out.append('-- AUTOINCREMENT 序列对齐到已导入的最大 id（先 UPDATE 再补 INSERT）')
        for t in imported:
            out.append("UPDATE sqlite_sequence SET seq = (SELECT COALESCE(MAX(id), 0) FROM \"%s\") "
                       "WHERE name = '%s';" % (t, t))
            out.append("INSERT OR IGNORE INTO sqlite_sequence (name, seq) "
                       "SELECT '%s', COALESCE(MAX(id), 0) FROM \"%s\" "
                       "WHERE (SELECT COUNT(*) FROM sqlite_sequence WHERE name = '%s') = 0 "
                       "AND EXISTS (SELECT 1 FROM sqlite_master "
                       "WHERE type = 'table' AND name = '%s' AND sql LIKE '%%AUTOINCREMENT%%');"
                       % (t, t, t, t))
    master.close()
    return '\n'.join(out) + '\n'


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('--store-json', required=True,
                    help='{"db-shard:<name>": "<payload>"} 快照 JSON 文件')
    ap.add_argument('-o', '--out', default=None)
    args = ap.parse_args()
    sql = export_script(_load_store(args.store_json))
    if args.out:
        path = args.out if os.path.isabs(args.out) else os.path.join(ROOT, args.out)
        with open(path, 'w', encoding='utf-8') as f:
            f.write(sql)
        print('wrote %s (%d bytes)' % (path, len(sql)))
    else:
        sys.stdout.write(sql)


if __name__ == '__main__':
    main()
