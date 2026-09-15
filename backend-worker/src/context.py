"""请求级上下文：保存当前 env（D1/R2 绑定）与认证用户。"""
env = None
current_user = None


def auth_header(request):
    """从 Authorization 头提取 Bearer token。"""
    raw = request.headers.get("Authorization") or ""
    if raw.startswith("Bearer "):
        return raw[len("Bearer "):]
    return raw