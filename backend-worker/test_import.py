"""开发用：mock js/workers/pyodide 以在本地验证路由/模型可导入。

用法：uv run python test_import.py
"""
import sys, types
from pathlib import Path

SRC = Path(__file__).parent / 'src'
sys.path.insert(0, str(SRC))

# ── mock 全局 js 与 pyodide.ffi（workers 包依赖）──
js = types.ModuleType('js')
sys.modules['js'] = js

# 系统化 mock js 全局（workers 包会访问构造函数名与静态属性）
def _js_attr(name):
    return _Void(name)

class _Void:
    def __init__(self, name=None):
        self.constructor = types.SimpleNamespace(name=name or 'Object')
    def __getattr__(self, name): return _Void(name)
    def __call__(self, *a, **k): return _Void()
    def new(self, *a, **k): return _Void()

# 常用 js 全局构造器
for _n in ('Response', 'Request', 'FormData', 'Blob', 'File', 'ArrayBuffer',
           'ReadableStream', 'URLSearchParams', 'Object', 'Promise', 'Headers',
           'Error', 'TypeError', 'Map', 'Set', 'Uint8Array'):
    setattr(js, _n, _Void(_n))
jsnull = _Void('null')

_helper = types.ModuleType('_pyodide_entrypoint_helper')
sys.modules['_pyodide_entrypoint_helper'] = _helper
_flags = types.ModuleType('_cloudflare_compat_flags')
sys.modules['_cloudflare_compat_flags'] = _flags


pyodide = types.ModuleType('pyodide')
pyodide.__version__ = '0.26.0'
pyodide_ffi = types.ModuleType('pyodide.ffi')

def _ffi_attr(name):
    return _Void(name)

pyodide_ffi.create_proxy = _ffi_attr
pyodide_ffi.to_js = _ffi_attr
pyodide_ffi.destroy_proxies = _ffi_attr
pyodide_ffi.jsnull = jsnull
pyodide_ffi.JsException = type('JsException', (Exception,), {})
pyodide_ffi.JsProxy = object
pyodide_ffi.create_once_callable = _ffi_attr
pyodide_ffi.create_py_proxy = _ffi_attr
pyodide_ffi.wraps = lambda *a, **k: (lambda x: x)
pyodide_ffi.__getattr__ = _ffi_attr
sys.modules['pyodide'] = pyodide
sys.modules['pyodide.ffi'] = pyodide_ffi

# pyodide.http
pyodide_http = types.ModuleType('pyodide.http')
pyodide_http.FetchResponse = type('FetchResponse', (), {})
pyodide_http.pyfetch = _Void('pyfetch')
pyodide_http.open_url = _Void('open_url')
pyodide_http.__getattr__ = _ffi_attr
sys.modules['pyodide.http'] = pyodide_http
pyodide.http = pyodide_http

import entry

count = len(entry.router.routes)
methods = {}
for r in entry.router.routes:
    methods[str(r.method)] = methods.get(str(r.method), 0) + 1

print(f'OK: {count} routes registered')
for m, c in sorted(methods.items()):
    print(f'  {m}: {c}')

if len(sys.argv) > 1 and sys.argv[1] == '--dump':
    dump_data = '\n'.join(f'{r.method} {r.pattern.pattern}' for r in entry.router.routes)
    if len(sys.argv) > 2:
        with open(sys.argv[2], 'w', encoding='utf-8') as _f:
            _f.write(dump_data + '\n')
    else:
        print(dump_data)

for r in entry.router.routes:
    assert r.pattern, r.pattern.pattern

print('Import cascade OK')
