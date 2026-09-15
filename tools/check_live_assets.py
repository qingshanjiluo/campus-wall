"""Crawl live Pages routes; verify every referenced local asset (/static/.., /pages/..) returns 200."""
import re, sys, urllib.request, collections

BASE = 'https://campus-wall-673.pages.dev'
ROUTES = ['/', '/waterfall', '/trade', '/romance', '/gossip', '/shop', '/search', '/checkin',
          '/favorites', '/notifications', '/create', '/create-station', '/admin', '/reset-password',
          '/about', '/terms', '/privacy', '/post/1', '/station/1', '/profile/1', '/zzz-none']
EXPECT_404 = {'/zzz-none'}   # 404 兜底路由探针
pat = re.compile(r'''(?:src|href)=["'](/[^"']+)["']''')

def head(url):
    req = urllib.request.Request(url, method='GET', headers={'Range': 'bytes=0-0', 'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)'})
    try:
        with urllib.request.urlopen(req, timeout=25) as r:
            return r.status
    except Exception as e:
        return getattr(e, 'code', str(e))

refs = collections.defaultdict(set)
html_fail = 0
for rt in ROUTES:
    try:
        with urllib.request.urlopen(urllib.request.Request(BASE + rt, headers={'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64)'}), timeout=25) as r:
            body = r.read().decode('utf-8', 'ignore')
            for m in pat.finditer(body):
                u = m.group(1).split('?')[0]
                if u.startswith('/static/') or u.startswith('/assets/'):
                    refs[u].add(rt)
    except Exception as e:
        print(f'ROUTE FAIL {rt}: {e}'); html_fail += 1

bad = 0
checked = 0
for u in sorted(refs):
    checked += 1
    code = head(BASE + u)
    if code not in (200, 206):
        bad += 1
        print(f'ASSET FAIL {code} {u}  <- from {sorted(refs[u])[:3]}')
print(f'== asset crawl: routes={len(ROUTES)} html_fail={html_fail} refs={checked} bad={bad} ==')
sys.exit(1 if (bad or html_fail) else 0)
