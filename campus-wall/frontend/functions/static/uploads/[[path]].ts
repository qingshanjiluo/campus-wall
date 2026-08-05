/**
 * Cloudflare Pages Function — 上传图片代理
 * 将 /static/uploads/* 请求转发到后端（图片上传后存储在 Flask 端）。
 */
const API_BASE = (typeof API_BASE !== 'undefined' && API_BASE)
  ? API_BASE.replace(/\/+$/, '')
  : '';

export async function onRequest(context) {
  const { request, env, params } = context;
  const base = (env.API_BASE && env.API_BASE.replace(/\/+$/, '')) || API_BASE;

  const url = new URL(request.url);
  const rest = (params.path || []).join('/');
  const query = url.search;

  const target = base
    ? `${base}/static/uploads/${rest}${query}`
    : `${url.origin}/static/uploads/${rest}${query}`;

  const headers = new Headers(request.headers);
  headers.delete('host');

  const upstream = await fetch(target, {
    method: request.method,
    headers,
    redirect: 'manual'
  });

  const resHeaders = new Headers(upstream.headers);
  resHeaders.set('Access-Control-Allow-Origin', '*');
  return new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: resHeaders
  });
}
