/**
 * CampusWall Pages 高级控制（_worker.js）
 * 背景：campus-wall Pages 项目为 legacy "advanced control" 模式，Direct Upload
 * 会忽略 functions/ 目录，仅识别仓库内 _worker.js。本文件同时承担：
 *   1. /api/*  → 反代到 env.API_BASE（Cloudflare Python Worker + KV）
 *   2. /static/uploads/* → 同样反代（图片由 Worker 存 KV）
 *   3. 干净路由：/ → /pages/index.html；/post/:id → /pages/post.html 等（内部 rewrite）
 *   4. 其余：先走静态资源，未命中回退 /pages/404.html (404)
 */

const ROUTES = {
  '/': '/pages/index.html',
  '/waterfall': '/pages/waterfall.html',
  '/trade': '/pages/trade.html',
  '/romance': '/pages/romance.html',
  '/gossip': '/pages/gossip.html',
  '/shop': '/pages/shop.html',
  '/search': '/pages/search.html',
  '/checkin': '/pages/checkin.html',
  '/favorites': '/pages/favorites.html',
  '/notifications': '/pages/notifications.html',
  '/create': '/pages/create.html',
  '/create-station': '/pages/create_station.html',
  '/admin': '/pages/admin.html',
  '/reset-password': '/pages/reset_password.html',
  '/about': '/pages/about.html',
  '/terms': '/pages/terms.html',
  '/privacy': '/pages/privacy.html',
};

const PREFIXES = [
  ['/post/', '/pages/post.html'],
  ['/station/', '/pages/station.html'],
  ['/profile/', '/pages/profile.html'],
];

async function proxyToWorker(request, env, targetPath) {
  const base = env && env.API_BASE ? String(env.API_BASE).replace(/\/+$/, '') : '';
  if (!base) {
    return new Response(JSON.stringify({
      error: 'API_NOT_CONFIGURED',
      message: 'Pages 环境变量 API_BASE 未设置（Settings → Environment variables）',
    }), { status: 501, headers: { 'Content-Type': 'application/json', 'Access-Control-Allow-Origin': '*' } });
  }
  const url = new URL(request.url);
  const target = base + targetPath + url.search;
  const headers = new Headers(request.headers);
  headers.delete('host');
  headers.delete('connection');
  const init = { method: request.method, headers, redirect: 'manual' };
  if (!['GET', 'HEAD'].includes(request.method)) init.body = request.body;
  try {
    const upstream = await fetch(target, init);
    const resHeaders = new Headers(upstream.headers);
    resHeaders.set('Access-Control-Allow-Origin', '*');
    resHeaders.set('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE, OPTIONS');
    resHeaders.set('Access-Control-Allow-Headers', 'Content-Type, Authorization');
    return new Response(upstream.body, {
      status: upstream.status, statusText: upstream.statusText, headers: resHeaders,
    });
  } catch (err) {
    return new Response(JSON.stringify({ error: 'PROXY_ERROR', message: String((err && err.message) || err), target }),
      { status: 502, headers: { 'Content-Type': 'application/json' } });
  }
}

export default {
  async fetch(request, env, ctx) {
    const url = new URL(request.url);
    const path = url.pathname;

    // 1) API 反代
    if (path === '/api' || path.startsWith('/api/')) {
      if (request.method === 'OPTIONS') {
        return new Response(null, { status: 204, headers: {
          'Access-Control-Allow-Origin': '*',
          'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, OPTIONS',
          'Access-Control-Allow-Headers': 'Content-Type, Authorization',
        } });
      }
      return proxyToWorker(request, env, path);
    }
    // 2) 上传文件反代（图片二进制在 Worker KV）
    if (path.startsWith('/static/uploads/')) {
      return proxyToWorker(request, env, path);
    }

    // 3) 干净路由 → 直接改写（优先于磁盘查找，避免 .html 308 干扰）
    if (ROUTES[path]) {
      const req = new Request(url.origin + ROUTES[path], request);
      return env.ASSETS.fetch(req);
    }
    for (const [prefix, page] of PREFIXES) {
      if (path.startsWith(prefix)) {
        const req = new Request(url.origin + page, request);
        return env.ASSETS.fetch(req);
      }
    }

    // 4) 静态资源
    const assetResp = await env.ASSETS.fetch(request);
    if (assetResp.status !== 404) return assetResp;

    // 5) 无扩展名兜底：/foo -> foo.html（若存在）
    if (!path.includes('.', path.lastIndexOf('/'))) {
      const withHtml = new Request(url.origin + path + '.html', request);
      const htmlResp = await env.ASSETS.fetch(withHtml);
      if (htmlResp.status === 200) return htmlResp;
    }

    // 6) 404
    const notFound = new Request(url.origin + '/pages/404.html', request);
    return new Response((await env.ASSETS.fetch(notFound)).body, {
      status: 404, headers: { 'Content-Type': 'text/html; charset=utf-8' },
    });
  },
};
