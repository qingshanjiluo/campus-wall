"""入口：路由注册与请求分发。"""
import re
from urllib.parse import urlparse

from workers import WorkerEntrypoint
from http import HTTPMethod

import web as httpmod
import context
import bootstrap
import routes_auth
import routes_stations
import routes_posts
import routes_social
import routes_extended
import uploads


class Route:
    """单个路由：(方法, 正则, handler)。handler 签名 async fn(request, params)"""

    def __init__(self, method, pattern, handler):
        self.method = method.upper()
        self.pattern = re.compile(pattern)
        self.handler = handler


class Router:
    def __init__(self):
        self.routes = []

    def add(self, method, pattern, handler):
        self.routes.append(Route(method, pattern, handler))

    def register_module(self, module):
        for (method, pattern, handler) in getattr(module, 'ROUTES', []):
            self.routes.append(Route(method, pattern, handler))

    async def dispatch(self, request):
        path = urlparse(request.url).path
        method = httpmod.method_value(request)
        for r in self.routes:
            if r.method != '*' and r.method != method:
                continue
            m = r.pattern.match(path)
            if not m:
                continue
            return await r.handler(request, m.groupdict())
        return httpmod.error('接口不存在', 404)


router = Router()

for mod in (routes_auth, routes_stations, routes_posts, routes_social, routes_extended):
    router.register_module(mod)

# 上传文件回传：D1 base64 / R2
router.add(HTTPMethod.GET, r'^/api/uploads/(?P<path>[A-Za-z0-9_./-]+)$', uploads.serve_upload)
# 前端 /static/uploads/* 由 Pages Function 反代，这里同样支持直接命中
router.add(HTTPMethod.GET, r'^/static/uploads/(?P<path>[A-Za-z0-9_./-]+)$', uploads.serve_upload)


class Default(WorkerEntrypoint):
    async def fetch(self, request):
        context.env = self.env
        context.current_user = None
        try:
            await bootstrap.ensure_ready()
            return await router.dispatch(request)
        except Exception as e:
            # D1/权限/参数异常统一转 500
            import traceback
            traceback.print_exc()
            return httpmod.error(f'服务器内部错误: {e}', 500)