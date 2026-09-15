/**
 * Cloudflare Pages Function — API 反向代理
 * 将 /api/* 请求转发到后端（Cloudflare Worker）。
 *
 * 环境变量：
 *   API_BASE   后端基础地址，如 https://campus-wall-api.sifangzhiji.workers.dev
 *              未设置时返回 501 明确报错（不做同源回环，避免自我循环）
 */
export async function onRequest(context) {
  const { request, env, params } = context;
  let target = '';
  try {
    const base = (env.API_BASE && env.API_BASE.replace(/\/+$/, '')) || '';

    const url = new URL(request.url);
    const rest = (params.path || []).join('/');
    const query = url.search;

    if (!base) {
      // 明确失败优于自我回环（origin 再进本函数会无限循环）
      return new Response(JSON.stringify({
        error: 'API_NOT_CONFIGURED',
        message: 'Pages 环境变量 API_BASE 未设置（Settings → Environment variables）'
      }), { status: 501, headers: { 'Content-Type': 'application/json' } });
    }
    target = `${base}/api/${rest}${query}`;

    // 复制请求头（去掉 hop-by-hop 头）
    const headers = new Headers(request.headers);
    headers.delete('host');
    headers.delete('connection');

    const init = {
      method: request.method,
      headers,
      redirect: 'manual'
    };

    // GET/HEAD 不带 body，其余带
    if (!['GET', 'HEAD'].includes(request.method)) {
      init.body = request.body;
    }

    const upstream = await fetch(target, init);

    const resHeaders = new Headers(upstream.headers);
    resHeaders.set('Access-Control-Allow-Origin', '*');
    resHeaders.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
    resHeaders.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
    resHeaders.set('X-Debug-Target', target);
    resHeaders.set('X-Debug-Upstream-Status', String(upstream.status));

    // OPTIONS 预检
    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: resHeaders });
    }

    return new Response(upstream.body, {
      status: upstream.status,
      statusText: upstream.statusText,
      headers: resHeaders
    });
  } catch (err) {
    return new Response(JSON.stringify({
      error: 'PROXY_ERROR',
      message: String(err && err.stack || err),
      target,
      apiBase: env && env.API_BASE ? 'set' : 'unset'
    }), { status: 500, headers: { 'Content-Type': 'application/json' } });
  }
}
