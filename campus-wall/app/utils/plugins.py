# -*- coding: utf-8 -*-
"""插件钩子雏形（R13）。

极简事件钩子注册表：业务关键路径埋点（post_created / comment_created /
user_registered / chat_message_sent），钩子异常绝不影响主流程（全部吞掉+记日志）。
启用状态持久化在 site_config 的 disabled_plugins（JSON 数组）。

插件形态：一个插件 = 一个普通函数（接收 ctx dict）+ register_hook 调用。
未来扩展为独立包/热加载时，只需保持该注册表协议不变。
"""
import json

from app.models import query_db

# {hook_name: [(plugin_name, callable), ...]}
_REGISTRY = {}
_DISABLED = set()

HOOK_NAMES = ('post_created', 'comment_created', 'user_registered', 'chat_message_sent')


def register_hook(hook_name, plugin_name, fn):
    if hook_name not in HOOK_NAMES:
        raise ValueError(f'未知钩子: {hook_name}')
    _REGISTRY.setdefault(hook_name, []).append((plugin_name, fn))


def _load_disabled():
    try:
        from app.models_ext import get_site_config
        raw = get_site_config().get('disabled_plugins', '[]')
        data = json.loads(raw) if isinstance(raw, str) else []
        _DISABLED.clear()
        _DISABLED.update(data)
    except Exception:
        pass


def run_hook(hook_name, **ctx):
    """触发钩子：任何插件异常都被吞掉（不影响主流程）。"""
    if hook_name not in _REGISTRY:
        return
    if not _DISABLED:
        _load_disabled()
    for plugin_name, fn in _REGISTRY.get(hook_name, []):
        if plugin_name in _DISABLED:
            continue
        try:
            fn(dict(ctx))
        except Exception:
            pass


def list_plugins():
    """管理端插件清单：名字 / 挂载点 / 启用状态。"""
    if not _DISABLED:
        _load_disabled()
    out = []
    for hook_name, fns in _REGISTRY.items():
        for plugin_name, _fn in fns:
            out.append({'plugin': plugin_name, 'hook': hook_name,
                        'enabled': plugin_name not in _DISABLED})
    return out


def set_plugin_enabled(plugin_name, enabled):
    if not _DISABLED:
        _load_disabled()
    if enabled:
        _DISABLED.discard(plugin_name)
    else:
        _DISABLED.add(plugin_name)
    try:
        from app.models_ext import set_site_config
        set_site_config('disabled_plugins', json.dumps(sorted(_DISABLED)))
    except Exception:
        pass
    return True


# ── 内置示例插件：欢迎插件（新注册用户收到一条欢迎通知）──
def _welcome_plugin(ctx):
    try:
        from app.models import create_notification
        create_notification(ctx['user_id'], None, 'system',
                            '欢迎加入校园墙！先去任务中心领取新手任务吧', '/tasks')
    except Exception:
        pass


def register_builtin_plugins():
    register_hook('user_registered', 'welcome_plugin', _welcome_plugin)