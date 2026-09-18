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

# ── PowerShell 脚本必须纯 ASCII ──
# PS 5.1 默认以 GBK 读取无 BOM 的 UTF-8 文件：CJK 注释会把下一行吞进注释，
# 造成"变量莫名未定义/步骤莫名失败"的诡异 bug（已实际踩过）。CJK 文本一律走 [regex]::Unescape('\uXXXX')。
for ps in sorted(ROOT.glob('tools/*.ps1')):
    text = ps.read_text(encoding='utf-8')
    bad_lines = [i for i, line in enumerate(text.split('\n'), 1) if not line.isascii()]
    if bad_lines:
        fails += 1
        print(f'ASCII FAIL: {ps.name} 非 ASCII 行 {bad_lines[:5]}（PS5.1 解析风险）')

# ── 页面壳层结构守卫（锁死 R7-A 修复，防回归）──
# 1) 每个页面必须恰有一个 #app-root 与一个 #page-main（重复会让 JS 定位错乱）
for page in sorted((FRONT / 'pages').glob('*.html')):
    html = page.read_text(encoding='utf-8')
    if page.name == '404.html':
        continue
    n_root = html.count('id="app-root"')
    n_main = html.count('id="page-main"')
    if n_root != 1:
        fails += 1
        print(f'SHELL FAIL: {page.name} #app-root 出现 {n_root} 次（应为 1）')
    if n_main != 1:
        fails += 1
        print(f'SHELL FAIL: {page.name} #page-main 出现 {n_main} 次（应为 1）')
# 2) common.js 注入模板不得再包含空 <main>（会与页面自带 main 重复），
#    footer 必须随 TAIL_HTML 注入到页面内容之后（beforeend），否则 footer 渲染在正文上方
common = (FRONT / 'static' / 'js' / 'common.js').read_text(encoding='utf-8')
if '<main id="page-main">' in common:
    fails += 1
    print('SHELL FAIL: common.js 注入模板仍含 main 注入（重复 main 回归）')
if not re.search(r"insertAdjacentHTML\(\s*['\"]beforeend['\"]\s*,\s*TAIL_HTML\s*\)", common):
    fails += 1
    print("SHELL FAIL: injectCommon 未按 beforeend 注入 TAIL_HTML（footer 位置回归）")

print(f'js_gate: {"PASS (0 fails)" if fails == 0 else f"FAIL ({fails})"}')
sys.exit(1 if fails else 0)
