"""In-memory Workers KV binding shim for the native test harness.

Implements the async subset db.py uses:
    await kv.get(key)                      -> str | bytes | None
    await kv.put(key, value)               -> None   (str/bytes only, ≤5MiB)
    await kv.delete(key)                   -> None
    await kv.list({'prefix','limit','cursor'}) -> dict matching the JS KV metadata
        shape: {'keys': [{'name','expiration','metadata'}...], 'list_keys': [...],
                'cursors': {'before','after'}, 'completed': bool}

Keys are stored in a plain dict sorted lexicographically (like real KV).
Cursor semantics: the page resumes AFTER the cursor key (exclusive), like CF.
"""

MAX_VALUE_BYTES = 5 * 1024 * 1024
DEFAULT_LIST_LIMIT = 1000


class KVShim:
    def __init__(self):
        self._data = {}
        self.get_calls = 0
        self.put_calls = 0
        self.delete_calls = 0
        self.list_calls = 0

    # ── core ops ──
    async def get(self, key):
        self.get_calls += 1
        if not isinstance(key, str):
            raise TypeError('KV key must be a string')
        _check_key(key)
        return self._data.get(key)

    async def put(self, key, value):
        self.put_calls += 1
        if not isinstance(key, str):
            raise TypeError('KV key must be a string')
        _check_key(key)
        if isinstance(value, str):
            size = len(value.encode('utf-8'))
        elif isinstance(value, (bytes, bytearray)):
            value = bytes(value)
            size = len(value)
        else:
            raise TypeError('KV value must be text or bytes')
        if size > MAX_VALUE_BYTES:
            raise ValueError('value exceeds 5MiB KV limit (%d bytes)' % size)
        self._data[key] = value

    async def delete(self, key):
        self.delete_calls += 1
        self._data.pop(key, None)

    async def list(self, options=None):
        self.list_calls += 1
        opts = dict(options or {})
        prefix = opts.get('prefix') or ''
        limit = int(opts.get('limit') or DEFAULT_LIST_LIMIT)
        cursor = opts.get('cursor') or ''
        names = sorted(k for k in self._data if k.startswith(prefix))
        if cursor:
            names = [k for k in names if k > cursor]
        page = names[:limit]
        has_more = len(names) > len(page)
        keys = [{'name': k, 'expiration': None, 'metadata': None} for k in page]
        cursors = {
            'before': page[0] if page else (cursor or ''),
            'after': page[-1] if page else '',
        }
        return {
            'keys': keys,
            'list_keys': page[:],
            'listKeys': page[:],          # JS spelling too, just in case
            'cursors': cursors,
            'completed': not has_more,
            'cacheStatus': 'hit',
        }

    # ── test helpers ──
    def snapshot(self):
        """Plain dict copy of all stored key→value (for the export tool)."""
        return dict(self._data)

    def keys(self):
        return sorted(self._data)


def _check_key(key):
    if not key or len(key.encode('utf-8')) > 512:
        raise ValueError('invalid key length: %r' % key)
