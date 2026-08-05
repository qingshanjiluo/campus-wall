/**
 * Cloudflare Pages Function — API 反向代理
 * 将 /api/* 请求转发到后端（PythonAnywhere / 其他 Flask 服务器）。
 *
 * 环境变量：
 *   API_BASE   后端基础地址，如 https://yourname.pythonanywhere.com
 *              不设置时回退到相对路径 /api（适用于同源部署）
 */
const API_BASE = (typeof API_BASE !== 'undefined' && API_BASE)
  ? API_BASE.replace(/\/+$/, '')
  : '';

export async function onRequest(context) {
  const { request, env, params } = context;

  // 优先用 Pages 环境变量，其次构建期变量
  const base = (env.API_BASE && env.API_BASE.replace(/\/+$/, '')) || API_BASE;

  const url = new URL(request.url);
  const rest = (params.path || []).join('/');
  const query = url.search;

  const target = base
    ? `${base}/api/${rest}${query}`
    : `${url.origin}/api/${rest}${query}`;

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

  // OPTIONS 预检
  if (request.method === 'OPTIONS') {
    return new Response(null, { status: 204, headers: resHeaders });
  }

  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: resHeaders
  });
}
