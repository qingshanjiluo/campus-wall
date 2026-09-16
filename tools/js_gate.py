# -*- coding: utf-8 -*-
"""前端 JS 语法门：外链脚本 + 全部页面内联脚本 node --check。CI 与本地通用。"""
import pathlib
import re
import subprocess
import sys
import tempfile

ROOT = pathlib.Path(__file__).resolve().parent.parent
FRONT = ROOT / 'campus-wall' / 'frontend'
fails = 0

for f in sorted((FRONT / 'static' / 'js').glob('*.js')):
    if subprocess.run(['node', '--check', str(f)], capture_output=True).returncode:
        fails += 1
        print('EXTERNAL FAIL:', f)

tmpdir = tempfile.TemporaryDirectory()
tmp = pathlib.Path(tmpdir.name)
for page in sorted((FRONT / 'pages').glob('*.html')):
    html = page.read_text(encoding='utf-8')
    for i, m in enumerate(re.finditer(r'<script(?![^>]*src)[^>]*>([\s\S]*?)</script>', html)):
        body = m.group(1)
        if not body.strip() or 'src=' in body[:40] and '<' not in body:
            continue
        p = tmp / f'{page.stem}_inline_{i}.js'
        p.write_text(body, encoding='utf-8-sig')
        r = subprocess.run(['node', '--check', str(p)], capture_output=True, text=True)
        if r.returncode:
            fails += 1
            print('INLINE FAIL:', page, f'#{i}')
            print(r.stderr[:400])

tmpdir.cleanup()

print(f'js_gate: {"PASS (0 fails)" if fails == 0 else f"FAIL ({fails})"}')
sys.exit(1 if fails else 0)
