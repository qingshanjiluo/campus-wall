/**
 * API 工具层 — 封装 fetch + JWT
 */
const api = {
    _token: localStorage.getItem('token') || null,

    setToken(token) {
        this._token = token;
        if (token) localStorage.setItem('token', token);
        else localStorage.removeItem('token');
    },

    getToken() { return this._token; },

    async request(url, options = {}) {
        const headers = { 'Content-Type': 'application/json', ...(options.headers || {}) };
        if (this._token) headers['Authorization'] = 'Bearer ' + this._token;
        const res = await fetch(url, { ...options, headers });
        const data = await res.json().catch(() => ({}));
        if (!res.ok) throw { status: res.status, message: data.error || '请求失败', data };
        return data;
    },

    get(url) { return this.request(url); },
    post(url, body) { return this.request(url, { method: 'POST', body: JSON.stringify(body || {}) }); },
    put(url, body) { return this.request(url, { method: 'PUT', body: JSON.stringify(body || {}) }); },
    delete(url) { return this.request(url, { method: 'DELETE' }); },
};
