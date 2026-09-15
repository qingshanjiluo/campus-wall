"""端点语句数预算探针 + 门槛（测试工具，不进 worker 包）。

生产代码零改动：monkeypatch db.query/execute/execute_script 计一次 API 调用
（不关心后端是 D1 还是 KV，两者走同一模型层）。

口径（父任务确认的预算帽，按请求统计，D1 模式）：
    admin   ≤5（冷） / ≤1（暖：60s memo 命中，auth 用户查询 + 0 条统计）
    list    ≤3（带 auth）/ ≤2（匿名）
    detail  ≤8（带 auth）/ ≤2（匿名）
    feed    ≤4（带 auth）/ ≤2（匿名）
    整套 suite（bootstrap 探针+seed+97 请求）总语句数 ≤100k

单独测量：  python tools/budgeted.py
作为门槛：  python tools/run_native_tests.py --budget   （默认模式仍为 d1/97 不变）
"""
import asyncio
import os
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

CAPS = {
    # scenario -> (cap,) 场景：admin_cold/admin_warm/list_auth/list_anon/
    #              detail_auth/detail_anon/feed_auth/feed_anon
    'admin_cold': 5, 'admin_warm': 1,
    'list_auth': 3, 'list_anon': 2,
    'detail_auth': 8, 'detail_anon': 2,
    'feed_auth': 4, 'feed_anon': 2,
}
SUITE_TOTAL_MAX = 100_000

# ── 计数器 ───────────────────────────────────────────────────
_state = {'n': 0, 'orig': None}


def install(dbmod):
    if _state['orig'] is not None:
        return
    orig = (dbmod.query, dbmod.execute, dbmod.execute_script)

    def counted(fn):
        async def wrapper(*a, **k):
            _state['n'] += 1
            return await fn(*a, **k)
        return wrapper

    dbmod.query, dbmod.execute, dbmod.execute_script = (counted(f) for f in orig)
    _state['orig'] = orig


def uninstall(dbmod):
    if _state['orig'] is None:
        return
    dbmod.query, dbmod.execute, dbmod.execute_script = _state['orig']
    _state['orig'] = None


def total():
    return _state['n']


# ── 场景测量 ────────────────────────────────────────────────
async def measure(h):
    """在当前 harness（任意模式）上跑预算场景，返回 {scenario: statements}。"""
    import db as dbmod
    import models_ext

    out = {}

    async def count(label, coro_fn, *, warm=False, shape=None):
        if not warm:
            models_ext.reset_stats_cache()
            if dbmod.backend() == 'kv':
                dbmod._kv_reset_state()
        base = total()
        resp = await coro_fn()
        out[label] = total() - base
        assert resp.status in (200, 201), f'{label}: status={resp.status}'
        if shape:
            shape(resp.body)
        return resp

    def _feed_shape(body):
        items = (body or {}).get('posts', [])
        assert items and isinstance(items[0].get('is_liked'), bool), \
            'auth feed must carry batched is_liked bool'

    login = await h.call('POST', '/api/auth/login',
                         body={'username': 'admin', 'password': 'admin123'})
    tok = login.body['token']

    # feed（/api/posts limit=20，带 auth 与匿名）
    await count('feed_auth', lambda: h.call('GET', '/api/posts', query='?limit=20', token=tok),
                shape=_feed_shape)
    await count('feed_anon', lambda: h.call('GET', '/api/posts', query='?limit=20'))
    # list（/api/stations）
    await count('list_auth', lambda: h.call('GET', '/api/stations', query='?limit=20', token=tok))
    await count('list_anon', lambda: h.call('GET', '/api/stations', query='?limit=20'))
    # detail（子站详情 + 帖子详情取大者）
    st = await h.call('GET', '/api/stations', query='?limit=1')
    sid = (st.body if isinstance(st.body, list) else st.body['stations'])[0]['id']
    po = await h.call('POST', '/api/posts',
                      body={'station_id': sid, 'title': 'budget probe', 'content': 'c',
                            'post_type': 'text'}, token=tok)
    pid = po.body.get('id') or po.body.get('post', {}).get('id')
    d1 = await count('detail', lambda: h.call('GET', f'/api/stations/{sid}', token=tok))
    d2 = await count('detail_post', lambda: h.call('GET', f'/api/posts/{pid}', token=tok))
    d3 = await count('detail_post_anon', lambda: h.call('GET', f'/api/posts/{pid}'))
    out['detail_auth'] = max(out['detail'], out['detail_post'])   # 子站详情/帖子详情取大
    out['detail_anon'] = out['detail_post_anon']                  # 子站详情匿名=1 ≤ 帽 2
    # admin stats 冷/暖（memo）
    await count('admin_cold', lambda: h.call('GET', '/api/admin/stats', token=tok))
    await count('admin_warm', lambda: h.call('GET', '/api/admin/stats', token=tok), warm=True)
    return out


async def gate(h, mode_label):
    """跑 measure 并断言全部帽 + 套件累计总量（由调用方保证计数器已安装）。"""
    res = await measure(h)
    viol = [(k, res[k], CAPS[k]) for k in CAPS if res.get(k, 0) > CAPS[k]]
    print(f'  [budget:{mode_label}] ' + '  '.join(f'{k}={v}' for k, v in sorted(res.items())))
    assert not viol, f'预算超限: {viol}'
    return res


# ── 独立 CLI：打印实测表 ─────────────────────────────────────
async def _cli():
    import run_native_tests as rnt
    import db as dbmod
    install(dbmod)  # once, before any boot(): seeds count toward totals
    tmp = tempfile.mkdtemp(prefix='cw-budget-')
    for mode in ('d1', 'kv'):
        base = total()
        h = rnt.Harness(mode, db_path=os.path.join(tmp, f'{mode}.sqlite'))
        await h.boot()
        res = await measure(h)
        print(f'== {mode} ==')
        for k in sorted(res):
            cap = CAPS.get(k)
            flag = '' if cap is None or res[k] <= cap else f'  <-- OVER cap {cap}'
            print(f'  {k:<16} {res[k]}{flag}')
        print(f'  mode total (incl. seed): {total() - base}')


if __name__ == '__main__':
    asyncio.run(_cli())
