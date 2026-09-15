"""Extract inline <script> blocks from frontend pages and syntax-check with node --check."""
import re, subprocess, sys, pathlib, tempfile, os

root = pathlib.Path('campus-wall/frontend')
fail = 0; checked = 0
pat = re.compile(r'<script(?![^>]*\bsrc=)[^>]*>(.*?)</script>', re.S | re.I)
for p in sorted(root.rglob('*.html')):
    for i, m in enumerate(pat.finditer(p.read_text(encoding='utf-8', errors='ignore'))):
        code = m.group(1)
        if not code.strip():
            continue
        checked += 1
        with tempfile.NamedTemporaryFile('w', suffix='.js', delete=False, encoding='utf-8') as tf:
            tf.write(code); tmp = tf.name
        try:
            r = subprocess.run(['node', '--check', tmp], capture_output=True, text=True, timeout=60)
            if r.returncode:
                fail += 1
                print(f'FAIL {p} block#{i}')
                print(r.stderr[:800])
        finally:
            os.unlink(tmp)
for p in sorted((root / 'static' / 'js').glob('*.js')):
    checked += 1
    r = subprocess.run(['node', '--check', str(p)], capture_output=True, text=True, timeout=60)
    if r.returncode:
        fail += 1
        print(f'FAIL {p}'); print(r.stderr[:800])
# _worker.js is an ES module -> check as .mjs
w = root / '_worker.js'
if w.exists():
    checked += 1
    r = subprocess.run(['node', '--check', str(w)], capture_output=True, text=True, timeout=60,
                       env={**os.environ, 'NODE_OPTIONS': ''})
    if r.returncode and 'export' in (r.stderr or ''):
        # retry via .mjs copy
        tmp = str(w) + '.mjs'
        pathlib.Path(tmp).write_bytes(w.read_bytes())
        r = subprocess.run(['node', '--check', tmp], capture_output=True, text=True, timeout=60)
        os.unlink(tmp)
    if r.returncode:
        fail += 1
        print(f'FAIL {w}'); print(r.stderr[:800])
print(f'== js syntax gate: checked={checked} fail={fail} ==')
sys.exit(1 if fail else 0)
