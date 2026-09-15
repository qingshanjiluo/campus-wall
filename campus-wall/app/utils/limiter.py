"""限流器（flask-limiter）。默认关闭，生产经 RATELIMIT_ENABLED=1 显式开启。

单机 memory storage 即可起步（gunicorn 多 worker 各自计数，作为粗略闸够用）；
扩容多机时把 RATELIMIT_STORAGE_URI 指向 redis 即可无缝升级。
"""
import os

from flask_limiter import Limiter
from flask_limiter.util import get_remote_address

enabled = os.environ.get('RATELIMIT_ENABLED', '0') == '1'
_storage = os.environ.get('RATELIMIT_STORAGE_URI') or 'memory://'

limiter = Limiter(
    key_func=get_remote_address,
    default_limits=[],
    enabled=enabled,
    storage_uri=_storage,
    strategy='fixed-window',
)


def init_app(app):
    limiter.init_app(app)
