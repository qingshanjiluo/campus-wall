"""Native test harness for the Cloudflare Worker code.

Runs the REAL route handlers / models / SQL against a local sqlite3 database
through a D1-compatible shim — no miniflare, no workerd, stable & fast.

Usage:
    python tools/run_native_tests.py            # business-logic matrix
    python tools/run_native_tests.py -v         # print each result line

The suite mirrors the frontend's real API usage (see campus-wall/frontend).
"""
import asyncio
import json
import os
import sqlite3
import sys
import tempfile
import types
from pathlib import Path
from urllib.parse import urlparse

ROOT = Path(__file__).resolve().parents[1]
SRC = ROOT / 'src'
sys.path.insert(0, str(SRC))

# ─────────────────────────────────────────────────────────────
# 1. Mock the Cloudflare/Pyodide runtime modules
# ─────────────────────────────────────────────────────────────
js = types.ModuleType('js')


class _Void:
    def __init__(self, name=None):
        self.constructor = types.SimpleNamespace(name=name or 'Object')

    def __getattr__(self, name):
        return _Void(name)

    def __call__(self, *a, **k):
        return _Void()

    def new(self, *a, **k):
        return _Void()


for _n in ('Response', 'Request', 'FormData', 'Blob', 'File', 'ArrayBuffer',
           'ReadableStream', 'URLSearchParams', 'Object', 'Promise', 'Headers',
           'Error', 'TypeError', 'Map', 'Set', 'Uint8Array'):
    setattr(js, _n, _Void(_n))
jsnull = _Void('null')
sys.modules['js'] = js

_helper = types.ModuleType('_pyodide_entrypoint_helper')
sys.modules['_pyodide_entrypoint_helper'] = _helper
_flags = types.ModuleType('_cloudflare_compat_flags')
sys.modules['_cloudflare_compat_flags'] = _flags

pyodide = types.ModuleType('pyodide')
pyodide.__version__ = '0.26.0'
pyodide_ffi = types.ModuleType('pyodide.ffi')
pyodide_ffi.jsnull = None  # native sqlite3 binds None as NULL natively
pyodide_ffi.JsException = type('JsException', (Exception,), {})
pyodide_ffi.JsProxy = object
pyodide_ffi.__getattr__ = lambda name: _Void(name)
sys.modules['pyodide'] = pyodide
sys.modules['pyodide.ffi'] = pyodide_ffi
pyodide_http = types.ModuleType('pyodide.http')
pyodide_http.__getattr__ = lambda name: _Void(name)
sys.modules['pyodide.http'] = pyodide_http
pyodide.http = pyodide_http


# ── workers.Response mock: captures status/body for assertions ──
class FakeResponse:
    def __init__(self, body=None, status=200, headers=None):
        self.body = body
        self.status = status
        self.headers = dict(headers or {})

    @classmethod
    def from_json(cls, data, status=200, headers=None):
        return cls(data, status=status, headers=headers)

    def json(self):
        return self.body


workers_mod = types.ModuleType('workers')
workers_mod.Response = FakeResponse
workers_mod.WorkerEntrypoint = type('WorkerEntrypoint', (), {'__init__': lambda self, ctx=None: setattr(self, 'ctx', ctx), 'env': None})
sys.modules['workers'] = workers_mod

# ─────────────────────────────────────────────────────────────
# 2. D1 shim on sqlite3
# ─────────────────────────────────────────────────────────────
class RowProxy:
    def __init__(self, sqlite_row):
        self._d = dict(zip(sqlite_row.keys(), tuple(sqlite_row)))

    def to_py(self):
        return self._d


class Stmt:
    def __init__(self, shim, sql):
        self.shim = shim
        self.sql = sql
        self.args = ()

    def bind(self, *args):
        return Stmt(self.shim, self.sql)  # keep chainable semantics
        # (we re-create to mimic D1 immutable builder)

    def _bound_args(self):
        return self.args

    async def all(self):
        conn = self.shim.conn
        cur = conn.execute(self.sql, self.args)
        rows = cur.fetchall()
        cur.close()
        return types.SimpleNamespace(results=[RowProxy(r) for r in rows])

    async def run(self):
        conn = self.shim.conn
        try:
            cur = conn.execute(self.sql, self.args)
            conn.commit()
            last_id = cur.lastrowid or 0
            changes = cur.rowcount
            cur.close()
        except sqlite3.Error as e:
            raise RuntimeError(str(e))
        return {'meta': {'last_row_id': last_id, 'changes': changes}}

    def get(self, key):
        return None


class D1Shim:
    """Mimics env.DB (D1Database). bind() must mutate: implement prepared with closure."""

    def __init__(self, db_path):
        self.conn = sqlite3.connect(db_path)
        self.conn.row_factory = sqlite3.Row
        self.conn.execute('PRAGMA foreign_keys=ON')

    def prepare(self, sql):
        shim = self

        class _Stmt:
            def __init__(self):
                self.args = ()

            def bind(self, *args):
                s = _Stmt()
                s.args = args
                return s

            async def all(self):
                cur = shim.conn.execute(sql, self.args)
                rows = cur.fetchall()
                cur.close()
                return types.SimpleNamespace(results=[RowProxy(r) for r in rows])

            async def run(self):
                try:
                    cur = shim.conn.execute(sql, self.args)
                    shim.conn.commit()
                    res = {'meta': {'last_row_id': cur.lastrowid or 0, 'changes': cur.rowcount}}
                    cur.close()
                    return res
                except sqlite3.Error as e:
                    raise RuntimeError(str(e))

        return _Stmt()

    async def exec(self, sql):
        self.conn.executescript(sql)
        self.conn.commit()
        return None

    def batch(self, stmts):
        raise NotImplementedError


# ─────────────────────────────────────────────────────────────
# 3. Request mock
# ─────────────────────────────────────────────────────────────
class FakeHeaders:
    def __init__(self, d=None):
        self._d = {k.lower(): v for k, v in (d or {}).items()}

    def get(self, k, default=None):
        return self._d.get(k.lower(), default)

    def __getitem__(self, k):
        return self._d[k.lower()]


class FakeFile:
    def __init__(self, name, data, content_type='image/png'):
        self.name = name
        self._data = data
        self.type = content_type

    async def bytes(self):
        return self._data


class FakeRequest:
    def __init__(self, url, method='GET', headers=None, body=None, form=None):
        self.url = url
        self.method = method
        self.headers = FakeHeaders(headers)
        self._body = body
        self._form = form or {}

    async def text(self):
        if self._body is None:
            return ''
        if isinstance(self._body, (dict, list)):
            return json.dumps(self._body)
        return self._body

    async def form_data(self):
        return self._form

    async def array_buffer(self):
        return self._body


# ─────────────────────────────────────────────────────────────
# 4. Harness
# ─────────────────────────────────────────────────────────────
class Env:
    pass


class Harness:
    def __init__(self, db_path):
        self.db_path = db_path
        self.env = Env()
        self.env.DB = D1Shim(db_path)
        self.env.JWT_SECRET = 'dev-only-change-me'
        self.env.JWT_EXPIRATION_DAYS = '30'
        self.env.UPLOADS = None

    async def boot(self):
        import context
        context.env = self.env
        import entry  # noqa: registers routes
        import bootstrap
        await bootstrap.ensure_ready()
        self.entry = entry
        self.bootstrap = bootstrap

    async def call(self, method, path, body=None, token=None, form=None, query=''):
        url = f'https://api.test{path}{query}'
        headers = {}
        if token:
            headers['Authorization'] = f'Bearer {token}'
        req = FakeRequest(url, method=method, headers=headers, body=body, form=form)
        resp = await self.entry.router.dispatch(req)
        return resp


# 1x1 red PNG
PNG_PIXEL = bytes.fromhex(
    '89504e470d0a1a0a0000000d49484452000000010000000108020000009077'
    '53de0000000c4944415408d76360f8cf000000020001e22fbc330000000049'
    '454e44ae426082')


def _png_bytes():
    # valid 1x1 red PNG
    return PNG_PIXEL


PASS = 0
FAIL = 0
FAILED = []
VERBOSE = '-v' in sys.argv


async def check(name, coro, expect=(200,), extract=None):
    global PASS, FAIL
    try:
        resp = await coro
    except Exception as e:
        import traceback
        traceback.print_exc()
        resp = None
        status, payload = 'EXC', str(e)
    if resp is not None:
        status = resp.status
        payload = resp.body if isinstance(resp.body, (dict, list, type(None))) else resp.body
    ok = status in expect
    if ok and extract:
        try:
            extract(payload)
        except Exception as e:
            ok = False
            payload = f'extract failed: {e}'
    if ok:
        PASS += 1
        if VERBOSE:
            print(f'  PASS {name} [{status}]')
    else:
        FAIL += 1
        sample = json.dumps(payload, ensure_ascii=False, default=str)[:200] if payload is not None else 'None'
        FAILED.append((name, status, sample))
        print(f'  FAIL {name} [{status}] {sample}')
    return payload


async def main():
    tmp = tempfile.mkdtemp(prefix='cw-test-')
    db_path = os.path.join(tmp, 'test.sqlite')
    h = Harness(db_path)
    await h.boot()

    print('== public ==')
    await check('stations list', h.call('GET', '/api/stations', query='?limit=3'))
    await check('posts list', h.call('GET', '/api/posts', query='?limit=3'))
    await check('station search', h.call('GET', '/api/stations/search', query='?q=%E8%A1%8C'))
    await check('comprehensive search', h.call('GET', '/api/recommend/search', query='?q=a'))
    await check('announcements', h.call('GET', '/api/announcements'))
    await check('kanban message', h.call('GET', '/api/kanban/message'))
    await check('station categories', h.call('GET', '/api/stations/categories'))

    print('== auth ==')
    import random
    u = f'tester{random.randint(1000, 9999)}'
    reg = await check('register', h.call('POST', '/api/auth/register',
                                         body={'username': u, 'email': f'{u}@test.com', 'password': 'pass123456'}),
                      expect=(201,))
    await check('register duplicate -> 409', h.call('POST', '/api/auth/register',
                                                    body={'username': u, 'email': f'{u}@test.com', 'password': 'pass123456'}),
                expect=(409, 400))
    login = await check('login', h.call('POST', '/api/auth/login',
                                        body={'username': u, 'password': 'pass123456'}))
    token = (login or {}).get('token')
    await check('me', h.call('GET', '/api/auth/me', token=token))
    await check('me anon -> 401', h.call('GET', '/api/auth/me'), expect=(401,))
    adm = await check('admin login', h.call('POST', '/api/auth/login',
                                            body={'username': 'admin', 'password': 'admin123'}))
    atoken = (adm or {}).get('token')

    print('== stations ==')
    st = await check('create station', h.call('POST', '/api/stations',
                                              body={'name': f'test-st-{u}', 'description': 'd', 'tags': ['x'], 'icon': 'book', 'is_public': True},
                                              token=token), expect=(201,))
    sid = (st or {}).get('id') or ((st or {}).get('station') or {}).get('id')

    async def _detail():
        r = await h.call('GET', f'/api/stations/{sid}')
        assert r.status == 200 and r.body['station']['id'] == sid if isinstance(r.body, dict) and 'station' in r.body else r.status == 200
        return r
    await check('station detail', _detail())
    await check('station posts', h.call('GET', f'/api/stations/{sid}/posts'))
    await check('station members', h.call('GET', f'/api/stations/{sid}/members'))
    await check('station stats', h.call('GET', f'/api/stations/{sid}/stats'))
    await check('update station', h.call('PUT', f'/api/stations/{sid}', body={'description': 'new desc'}, token=token))
    await check('update station by non-owner denied or ok', h.call('PUT', f'/api/stations/{sid}', body={'description': 'x'}, token=atoken))
    await check('stations mine', h.call('GET', '/api/stations/mine', token=token))
    await check('stations by/uid public-only', h.call('GET', '/api/stations/by/1'),
                extract=lambda r: None if isinstance(r, list) and all(
                    s.get('is_public') != 0 and 'role' not in s for s in r)
                else (_ for _ in ()).throw(AssertionError('private station or role leak')))
    await check('join station', h.call('POST', f'/api/stations/{sid}/leave', token=atoken))
    await check('rejoin', h.call('POST', f'/api/stations/{sid}/join', token=atoken))

    print('== posts ==')
    po = await check('create post', h.call('POST', '/api/posts',
                                           body={'station_id': sid, 'title': 'p1', 'content': 'body', 'post_type': 'text'}, token=token),
                     expect=(200, 201))
    pid = (po or {}).get('id') or ((po or {}).get('post') or {}).get('id')
    await check('post detail', h.call('GET', f'/api/posts/{pid}'))
    await check('like', h.call('POST', f'/api/posts/{pid}/like', body={}, token=atoken))
    await check('unlike', h.call('POST', f'/api/posts/{pid}/like', body={}, token=atoken))
    cm = await check('comment', h.call('POST', f'/api/posts/{pid}/comments',
                                       body={'content': 'c1'}, token=atoken), expect=(201,))
    cid = (cm or {}).get('id') or ((cm or {}).get('comment') or {}).get('id')
    await check('comments list', h.call('GET', f'/api/posts/{pid}/comments'))
    if cid:
        await check('comment like', h.call('POST', f'/api/social/comment/{cid}/like', body={}, token=token))
        await check('comment delete by admin', h.call('DELETE', f'/api/posts/comments/{cid}', token=atoken))
    await check('edit post', h.call('PUT', f'/api/posts/{pid}', body={'title': 'p1 edited'}, token=token))
    await check('versions', h.call('GET', f'/api/posts/{pid}/versions', token=token))
    await check('favorite toggle', h.call('POST', f'/api/favorites/post/{pid}', body={}, token=token))
    await check('favorite status', h.call('GET', f'/api/favorites/status/post/{pid}', token=token))
    await check('report', h.call('POST', '/api/reports', body={'target_type': 'post', 'target_id': pid, 'reason': 'r'}, token=atoken), expect=(200, 201))
    await check('posts liked', h.call('GET', '/api/posts/liked', token=token))
    await check('upload image', h.call('POST', '/api/posts/upload-image',
                                       form={'file': FakeFile('x.png', _png_bytes())}, token=token))
    await check('delete post (author)', h.call('DELETE', f'/api/posts/{pid}', token=token))

    print('== growth/shop ==')
    await check('checkin status', h.call('GET', '/api/checkin/status', token=token))
    await check('checkin', h.call('POST', '/api/checkin', body={}, token=token))
    await check('checkin twice idempotent', h.call('POST', '/api/checkin', body={}, token=token), expect=(200, 400))
    items = await check('shop items', h.call('GET', '/api/shop/items'))
    await check('shop coins', h.call('GET', '/api/shop/coins', token=token))
    if items:
        item_id = items[0]['id']
        await check('buy (may lack balance)', h.call('POST', '/api/shop/buy',
                                                     body={'item_id': item_id, 'quantity': 1}, token=token),
                    expect=(200, 400))
    await check('orders', h.call('GET', '/api/shop/orders', token=token))
    await check('transactions', h.call('GET', '/api/shop/transactions', token=token))
    await check('identity groups', h.call('GET', '/api/identity/groups'))
    await check('identity my', h.call('GET', '/api/identity/my', token=token))

    print('== trade/romance/gossip/social ==')
    await check('trade list', h.call('GET', '/api/trade'))
    await check('trade create', h.call('POST', '/api/trade',
                                       body={'title': 't', 'content': 'desc', 'price': 9.9, 'category': 'book',
                                             'contact': 'q', 'original_price': 20, 'condition': 'good'}, token=token),
                expect=(200, 201))
    await check('gossip list', h.call('GET', '/api/gossip'))
    await check('gossip create', h.call('POST', '/api/gossip', body={'content': 'tree'}, token=token), expect=(200, 201))
    await check('romance profiles', h.call('GET', '/api/romance/profiles'))
    await check('romance my profile', h.call('GET', '/api/romance/profile', token=token))
    await check('romance create profile', h.call('POST', '/api/romance/profile',
                                                 body={'nickname': 'nn', 'gender': 'f', 'grade': '2024', 'major': 'cs',
                                                       'hobbies': 'x', 'ideal': 'y', 'intro': 'z'}, token=token))
    await check('romance tasks', h.call('GET', '/api/romance/tasks'))
    await check('romance create task', h.call('POST', '/api/romance/task',
                                              body={'title': 'task', 'description': 'd', 'reward': 5}, token=token), expect=(200, 201))
    await check('romance links', h.call('GET', '/api/romance/links', token=token))
    await check('follow', h.call('POST', '/api/social/follow/2', body={}, token=token))
    await check('followers', h.call('GET', '/api/social/followers/2'))
    await check('following', h.call('GET', '/api/social/following/2'))
    await check('is-following', h.call('GET', '/api/social/is-following/2', token=token))
    await check('notifications', h.call('GET', '/api/social/notifications', token=token))
    await check('unread-count', h.call('GET', '/api/social/notifications/unread-count', token=token))
    await check('mark read', h.call('POST', '/api/social/notifications/read', body={}, token=token))
    await check('recommend posts', h.call('GET', '/api/recommend/posts', token=token))
    await check('recommend interests', h.call('GET', '/api/recommend/interests', token=token))
    await check('favorites list', h.call('GET', '/api/favorites', token=token))
    await check('user profile', h.call('GET', '/api/auth/user/2'))

    print('== extended: vote / link / romance update ==')
    vp = await check('create vote post', h.call('POST', '/api/posts',
                                                body={'station_id': sid, 'title': 'vote p', 'content': 'q?',
                                                      'post_type': 'vote', 'vote_options': ['A', 'B', 'C']}, token=token),
                     expect=(200, 201))
    vpid = (vp or {}).get('id') or ((vp or {}).get('post') or {}).get('id')
    if vpid:
        vdetail = await check('vote post detail shaped', h.call('GET', f'/api/posts/{vpid}', token=token))
        vpost = (vdetail or {}).get('post') or vdetail or {}
        await check('vote post has vote_options', h.call('GET', f'/api/posts/{vpid}'),
                    extract=lambda p: (_ for _ in ()).throw(AssertionError('no vote_options'))
                    if not (p.get('vote_options') and 'A' in p['vote_options']) else None)
        await check('vote A', h.call('POST', f'/api/posts/{vpid}/vote', body={'option_index': 0}, token=atoken))
        await check('vote twice -> 400', h.call('POST', f'/api/posts/{vpid}/vote', body={'option_index': 1}, token=atoken), expect=(400,))
        await check('vote bad index -> 400', h.call('POST', f'/api/posts/{vpid}/vote', body={'option_index': 99}, token=token), expect=(400,))
        vd = await check('vote counts reflect', h.call('GET', f'/api/posts/{vpid}', token=atoken),
                         extract=lambda p: None if (p.get('vote_counts', {}) or {}).get('0') == 1 and p.get('user_voted') else (_ for _ in ()).throw(AssertionError(f'counts={p.get("vote_counts")} voted={p.get("user_voted")}')))
    lp = await check('create link post', h.call('POST', '/api/posts',
                                                body={'station_id': sid, 'title': 'link p', 'content': 'see',
                                                      'post_type': 'link', 'link_url': 'https://example.com/x'}, token=token),
                     expect=(200, 201))
    lpid = (lp or {}).get('id') or ((lp or {}).get('post') or {}).get('id')
    if lpid:
        await check('link detail has link_url', h.call('GET', f'/api/posts/{lpid}'),
                    extract=lambda p: None if p.get('link_url') == 'https://example.com/x' else (_ for _ in ()).throw(AssertionError(f'link_url={p.get("link_url")}')))
    xss = await check('link post javascript: url stripped', h.call('POST', '/api/posts',
                                                                   body={'station_id': sid, 'title': 'xss', 'content': 'c',
                                                                         'post_type': 'link', 'link_url': 'javascript:alert(1)'},
                                                                   token=token), expect=(400,))
    # 恋爱档案二次保存（hobbies 数组）——回归 P1 bug
    await check('romance save #1', h.call('POST', '/api/romance/profile',
                                          body={'nickname': 'n1', 'gender': 'f', 'department': 'cs', 'hobbies': ['a', 'b']}, token=token),
                expect=(200, 201))
    await check('romance save #2 (update path, list hobbies)', h.call('POST', '/api/romance/profile',
                                                                      body={'nickname': 'n2', 'gender': 'f', 'department': 'math', 'hobbies': ['c', 'd']}, token=token))
    await check('romance re-read ok', h.call('GET', '/api/romance/profile', token=token))
    # admin today 字段
    await check('admin stats has today fields', h.call('GET', '/api/admin/stats', token=atoken),
                extract=lambda p: None if 'today_posts' in p and 'today_users' in p else (_ for _ in ()).throw(AssertionError(f'missing today_*: keys={list(p.keys())}')))
    # trade 参数校验
    await check('trade bad price -> 400', h.call('POST', '/api/trade',
                                                 body={'title': 't', 'content': 'c', 'price': 'abc<script>'}, token=token), expect=(400,))
    await check('trade negative price -> 400', h.call('POST', '/api/trade',
                                                      body={'title': 't', 'content': 'c', 'price': -5}, token=token), expect=(400,))

    print('== admin ==')
    await check('admin stats', h.call('GET', '/api/admin/stats', token=atoken))
    await check('admin stats series', h.call('GET', '/api/admin/stats/series', token=atoken))
    await check('admin users', h.call('GET', '/api/admin/users', token=atoken))
    await check('admin posts', h.call('GET', '/api/admin/posts', token=atoken))
    await check('admin stations', h.call('GET', '/api/admin/stations', token=atoken))
    await check('admin logs', h.call('GET', '/api/admin/logs', token=atoken))
    await check('admin groups', h.call('GET', '/api/admin/identity-groups', token=atoken))
    await check('reports list', h.call('GET', '/api/reports', token=atoken))
    await check('admin create announcement', h.call('POST', '/api/admin/announcements',
                                                    body={'title': 'ann', 'content': 'a', 'level': 'info'}, token=atoken),
                expect=(200, 201))
    await check('admin update user coins', h.call('PUT', '/api/admin/users/2',
                                                  body={'coins': 500}, token=atoken))
    await check('admin create shop item', h.call('POST', '/api/admin/shop/items',
                                                 body={'name': 'it', 'description': 'd', 'price_coins': 10,
                                                       'price_points': 0, 'item_type': 'badge', 'stock': -1, 'icon': 'medal'},
                                                 token=atoken), expect=(200, 201))
    await check('admin stats as user -> 403', h.call('GET', '/api/admin/stats', token=token), expect=(403,))

    print()
    print(f'================ native suite: total={PASS + FAIL} pass={PASS} fail={FAIL} ================')
    for name, status, sample in FAILED:
        print(f'FAIL {name} [{status}] {sample}')
    sys.exit(1 if FAIL else 0)


if __name__ == '__main__':
    asyncio.run(main())
