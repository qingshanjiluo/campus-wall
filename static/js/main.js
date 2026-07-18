let stationsData = [];

function showToast(message, duration = 2500) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toast._hideTimer);
    toast._hideTimer = setTimeout(() => toast.classList.remove('show'), duration);
}

function loadStations() {
    const grid = document.getElementById('stationGrid');
    grid.innerHTML = '<div class="loading">加载中...</div>';
    fetch('/api/stations')
        .then(res => { if (!res.ok) throw new Error('网络请求失败'); return res.json(); })
        .then(data => { stationsData = data; renderStations(data); })
        .catch(() => { grid.innerHTML = '<div class="loading">加载失败，请刷新重试</div>'; showToast('加载子站失败'); });
}

function renderStations(stations) {
    const grid = document.getElementById('stationGrid');
    if (!stations || stations.length === 0) { grid.innerHTML = '<div class="loading">暂无子站</div>'; return; }
    grid.innerHTML = stations.map(s => `
        <div class="station-card reveal" onclick="showToast('进入 ${s.name}')">
            <div class="cover">${s.cover && s.cover !== '/static/images/default-cover.jpg' ? '<img src="' + s.cover + '" alt="' + s.name + '">' : ''}</div>
            <div class="name">${s.name}</div>
            <div class="desc">${s.description || '暂无简介'}</div>
            <div class="tags">${(s.tags || []).map(t => '<span class="tag">#' + t + '</span>').join('')}</div>
            <div class="meta"><span>👥 ${s.user_count || 0}</span><span>📝 ${s.post_count || 0}</span></div>
        </div>
    `).join('');
}

function loadPosts() {
    const list = document.getElementById('postList');
    list.innerHTML = '<div class="loading">加载中...</div>';
    fetch('/api/posts?limit=6')
        .then(res => { if (!res.ok) throw new Error('网络请求失败'); return res.json(); })
        .then(data => renderPosts(data))
        .catch(() => { list.innerHTML = '<div class="loading">加载失败，请刷新重试</div>'; });
}

function renderPosts(posts) {
    const list = document.getElementById('postList');
    if (!posts || posts.length === 0) { list.innerHTML = '<div class="loading">暂无动态</div>'; return; }
    list.innerHTML = posts.map(p => `
        <article class="post-item reveal">
            <div class="title">${p.title}</div>
            <div class="content">${p.content || '暂无内容'}</div>
            <div class="meta">
                <span class="station-name">🏫 ${p.station_name}</span>
                <span>✍️ ${p.author || '匿名'}</span>
                <span>👁️ ${p.views || 0}</span>
                <span>❤️ ${p.likes || 0}</span>
                <span>💬 ${p.comments || 0}</span>
                <span>🕐 ${formatTime(p.created_at)}</span>
            </div>
        </article>
    `).join('');
}

function formatTime(timestamp) {
    if (!timestamp) return '刚刚';
    try {
        const date = new Date(timestamp);
        if (isNaN(date.getTime())) return '刚刚';
        const now = new Date();
        const diff = Math.floor((now - date) / 1000);
        if (diff < 60) return '刚刚';
        if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
        if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
        if (diff < 2592000) return Math.floor(diff / 86400) + '天前';
        return date.toLocaleDateString('zh-CN');
    } catch { return '刚刚'; }
}

function initSearch() {
    const input = document.getElementById('searchInput');
    if (!input) return;
    const doSearch = () => {
        const keyword = input.value.trim();
        if (!keyword) { renderStations(stationsData); return; }
        fetch('/api/search?q=' + encodeURIComponent(keyword))
            .then(res => res.json())
            .then(data => { renderStations(data); if (data.length === 0) showToast('未找到相关子站'); })
            .catch(() => showToast('搜索失败，请重试'));
    };
    input.addEventListener('keypress', e => { if (e.key === 'Enter') doSearch(); });
}
