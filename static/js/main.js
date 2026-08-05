/* ============================================================
   CampusWall 前端逻辑
   · KunUI 风格轮播（自动播放 + 拖拽 + 悬停暂停 + 指示器）
   · 动态流无限滚动（IntersectionObserver 哨兵 + cursor 分页）
   · 分类标签切换 · 深浅主题 · 子站市场 · 搜索
   ============================================================ */

let stationsData = [];

/* ---------- Toast ---------- */
function showToast(message, duration = 2500) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toast._hideTimer);
    toast._hideTimer = setTimeout(() => toast.classList.remove('show'), duration);
}

/* ---------- 主题切换 ---------- */
function initThemeToggle() {
    const toggle = document.getElementById('themeToggle');
    if (!toggle) return;
    const root = document.documentElement;

    const render = () => {
        const isDark = root.getAttribute('data-theme') === 'dark';
        toggle.textContent = isDark ? '☀️' : '🌙';
    };

    toggle.addEventListener('click', () => {
        const next = root.getAttribute('data-theme') === 'dark' ? 'light' : 'dark';
        root.setAttribute('data-theme', next);
        try { localStorage.setItem('cw-theme', next); } catch (e) { /* ignore */ }
        render();
    });

    render();
}

/* ---------- 轮播（Carousel） ---------- */
function initCarousel() {
    const track = document.getElementById('carouselTrack');
    const dotsWrap = document.getElementById('carouselDots');
    const carousel = document.getElementById('carousel');
    if (!track) return;

    const AUTOPLAY_DELAY = 3500;
    const DRAG_THRESHOLD = 40;

    let slides = [];
    let current = 0;
    let timer = null;
    let isDragging = false;
    let dragStart = 0;
    let dragOffset = 0;

    function next() {
        if (!slides.length) return;
        current = (current + 1) % slides.length;
        render();
    }
    function prev() {
        if (!slides.length) return;
        current = (current - 1 + slides.length) % slides.length;
        render();
    }
    function render() {
        if (!slides.length) return;
        track.style.transform = `translateX(${-current * 100 + dragOffset}%)`;
        [...dotsWrap.children].forEach((d, i) => d.classList.toggle('active', i === current));
    }

    function startAutoplay() {
        stopAutoplay();
        timer = setInterval(next, AUTOPLAY_DELAY);
    }
    function stopAutoplay() {
        if (timer) { clearInterval(timer); timer = null; }
    }

    // 鼠标/触摸拖拽
    function onStart(e) {
        if (!slides.length) return;
        isDragging = true;
        dragStart = (e.clientX !== undefined ? e.clientX : e.touches[0].clientX);
        stopAutoplay();
    }
    function onMove(e) {
        if (!isDragging) return;
        const x = (e.clientX !== undefined ? e.clientX : e.touches[0].clientX);
        const w = carousel.offsetWidth || 1;
        dragOffset = ((x - dragStart) / w) * 100;
        render();
    }
    function onEnd() {
        if (!isDragging) return;
        if (Math.abs(dragOffset) > DRAG_THRESHOLD) {
            dragOffset > 0 ? prev() : next();
        }
        isDragging = false;
        dragOffset = 0;
        render();
        startAutoplay();
    }

    // 悬停暂停 / 离开恢复
    carousel.addEventListener('mouseenter', stopAutoplay);
    carousel.addEventListener('mouseleave', startAutoplay);

    carousel.addEventListener('mousedown', onStart);
    carousel.addEventListener('mousemove', onMove);
    carousel.addEventListener('mouseup', onEnd);
    carousel.addEventListener('mouseleave', onEnd);
    carousel.addEventListener('touchstart', onStart, { passive: true });
    carousel.addEventListener('touchmove', onMove, { passive: true });
    carousel.addEventListener('touchend', onEnd);

    document.getElementById('carouselPrev').addEventListener('click', () => { prev(); startAutoplay(); });
    document.getElementById('carouselNext').addEventListener('click', () => { next(); startAutoplay(); });

    // 数据：热门子站做轮播
    fetch('/api/stations?order=hot&limit=8')
        .then(res => { if (!res.ok) throw new Error(); return res.json(); })
        .then(data => {
            if (!data.length) { carousel.style.display = 'none'; return; }
            slides = data;
            track.innerHTML = data.map((s, i) => `
                <div class="carousel-slide" onclick="showToast('进入 ${s.name}')">
                    ${s.cover && s.cover !== '/static/images/default-cover.jpg'
                        ? `<img src="${s.cover}" alt="${s.name}" ${i === 0 ? '' : 'loading="lazy"'}>`
                        : `<img src="/static/images/default-cover.jpg" alt="${s.name}">`}
                    <div class="carousel-caption">
                        <h3>${s.name}</h3>
                        <p>${s.description || '欢迎来到这个子站'}</p>
                    </div>
                </div>
            `).join('');
            dotsWrap.innerHTML = data.map((_, i) =>
                `<button class="carousel-dot ${i === 0 ? 'active' : ''}" data-index="${i}" aria-label="第 ${i + 1} 张"></button>`
            ).join('');
            [...dotsWrap.children].forEach(dot => {
                dot.addEventListener('click', () => { current = +dot.dataset.index; render(); startAutoplay(); });
            });
            render();
            startAutoplay();
        })
        .catch(() => { carousel.style.display = 'none'; });
}

/* ---------- 动态流（无限滚动） ---------- */
let feedCursor = 0;
let feedHasMore = true;
let feedLoading = false;
let activeTab = '';

function initFeed() {
    loadFeed(true);
}

function loadFeed(reset) {
    const list = document.getElementById('feedList');
    const sentinel = document.getElementById('feedSentinel');
    const dimmer = document.getElementById('feedDimmer');
    if (!list || feedLoading) return;

    if (reset) {
        feedCursor = 0;
        feedHasMore = true;
        list.innerHTML = skeleton(3);
    }

    feedLoading = true;
    sentinel.innerHTML = '<span class="feed-loading-spinner"></span>';
    dimmer.classList.add('loading-dimmer');

    const params = new URLSearchParams({ limit: 20 });
    if (feedCursor) params.set('cursor', feedCursor);
    if (activeTab) params.set('tab', activeTab);

    fetch(`/api/posts?${params}`)
        .then(res => { if (!res.ok) throw new Error('网络请求失败'); return res.json(); })
        .then(data => {
            feedHasMore = data.has_more;
            feedCursor = data.next_cursor;

            if (reset) {
                list.innerHTML = data.items.length
                    ? data.items.map(renderActivityCard).join('')
                    : '<div class="kun-null">暂无动态，快去创建第一个子站吧</div>';
            } else {
                list.insertAdjacentHTML('beforeend', data.items.map(renderActivityCard).join(''));
                if (!data.items.length) feedHasMore = false;
            }

            sentinel.innerHTML = feedHasMore
                ? '<button class="load-more-btn" id="loadMoreBtn">加载更多</button>'
                : '<span class="feed-end">没有更多动态了</span>';

            const moreBtn = document.getElementById('loadMoreBtn');
            if (moreBtn) moreBtn.addEventListener('click', () => loadFeed(false));

            dimmer.classList.remove('loading-dimmer');
            feedLoading = false;
        })
        .catch(() => {
            if (reset) list.innerHTML = '<div class="kun-null">加载失败，请刷新重试</div>';
            sentinel.innerHTML = '';
            dimmer.classList.remove('loading-dimmer');
            feedLoading = false;
        });
}

// IntersectionObserver 哨兵：接近底部自动加载
document.addEventListener('DOMContentLoaded', () => {
    const observeSentinel = () => {
        const sentinel = document.getElementById('feedSentinel');
        if (!sentinel || !('IntersectionObserver' in window)) return;
        new IntersectionObserver((entries) => {
            if (entries[0].isIntersecting && feedHasMore && !feedLoading) {
                loadFeed(false);
            }
        }, { rootMargin: '300px' }).observe(sentinel);
    };
    // 等首次渲染完成后观察
    setTimeout(observeSentinel, 300);
});

function renderActivityCard(p) {
    const tags = (p.station_tags || []).slice(0, 3)
        .map(t => `<span class="card-tag">#${t}</span>`).join('');
    return `
        <article class="activity-card reveal">
            <div class="card-head">
                <div class="card-avatar">${avatarEmoji(p.author)}</div>
                <div>
                    <div class="uname">${escapeHtml(p.author || '匿名')}</div>
                </div>
                <span class="station-chip">🏫 ${escapeHtml(p.station_name)}</span>
                <span class="time">${formatTime(p.created_at)}</span>
            </div>
            <h3 class="card-title">${escapeHtml(p.title)}</h3>
            <p class="card-content">${escapeHtml(p.content || '暂无内容')}</p>
            ${tags ? `<div class="card-tags">${tags}</div>` : ''}
            <div class="card-foot">
                <span class="stat"><span>👁</span><em>${p.views || 0}</em></span>
                <span class="stat"><span>❤</span><em>${p.likes || 0}</em></span>
                <span class="stat"><span>💬</span><em>${p.comments || 0}</em></span>
                <a href="#" class="view-more" onclick="showToast('帖子详情开发中');return false;">查看详情 ›</a>
            </div>
        </article>
    `;
}

function avatarEmoji(name) {
    const seed = (name || '匿名').charCodeAt(0) % 4;
    return ['🌸', '☀️', '🍀', '🌙'][seed];
}

function skeleton(n) {
    let html = '';
    for (let i = 0; i < n; i++) {
        html += `
            <div class="skeleton-card">
                <div class="skeleton-line" style="width:35%"></div>
                <div class="skeleton-line" style="width:85%"></div>
                <div class="skeleton-line" style="width:60%"></div>
                <div class="skeleton-line" style="width:45%;margin-bottom:0"></div>
            </div>`;
    }
    return html;
}

/* ---------- 标签切换 ---------- */
function initTabs() {
    const tabs = document.querySelectorAll('.home-tab');
    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            tabs.forEach(t => t.classList.remove('active'));
            tab.classList.add('active');
            activeTab = tab.dataset.tab;
            loadFeed(true);
        });
    });
}

/* ---------- 子站市场 ---------- */
function loadStations() {
    const grid = document.getElementById('stationGrid');
    grid.innerHTML = '<div class="loading" style="grid-column:1/-1;">加载中...</div>';
    fetch('/api/stations')
        .then(res => { if (!res.ok) throw new Error('网络请求失败'); return res.json(); })
        .then(data => { stationsData = data; renderStations(data); loadMiniStations(); })
        .catch(() => { grid.innerHTML = '<div class="loading" style="grid-column:1/-1;">加载失败，请刷新重试</div>'; showToast('加载子站失败'); });
}

function renderStations(stations) {
    const grid = document.getElementById('stationGrid');
    if (!stations || stations.length === 0) {
        grid.innerHTML = '<div class="loading" style="grid-column:1/-1;">暂无子站</div>';
        return;
    }
    grid.innerHTML = stations.map(s => `
        <div class="station-card reveal" onclick="showToast('进入 ${s.name}')">
            <div class="cover">${s.cover && s.cover !== '/static/images/default-cover.jpg' ? '<img src="' + s.cover + '" alt="' + s.name + '" loading="lazy">' : ''}</div>
            <div class="name">${escapeHtml(s.name)}</div>
            <div class="desc">${escapeHtml(s.description || '暂无简介')}</div>
            <div class="tags">${(s.tags || []).map(t => '<span class="tag">#' + escapeHtml(t) + '</span>').join('')}</div>
            <div class="meta"><span>👥 ${s.user_count || 0}</span><span>📝 ${s.post_count || 0}</span></div>
        </div>
    `).join('');
}

function loadMiniStations() {
    const list = document.getElementById('miniStations');
    if (!list) return;
    if (!stationsData.length) {
        list.innerHTML = '<div class="kun-null">暂无子站</div>';
        return;
    }
    const top = [...stationsData].sort((a, b) => (b.user_count || 0) - (a.user_count || 0)).slice(0, 5);
    list.innerHTML = top.map(s => `
        <a href="#" class="mini-station" onclick="showToast('进入 ${s.name}');return false;">
            <div class="icon">${s.cover && s.cover !== '/static/images/default-cover.jpg' ? `<img src="${s.cover}" alt="" style="width:100%;height:100%;object-fit:cover;border-radius:10px;">` : '🏫'}</div>
            <div class="info">
                <div class="name">${escapeHtml(s.name)}</div>
                <div class="meta">${escapeHtml((s.tags || []).slice(0, 2).join(' · ') || '校园子站')}</div>
            </div>
            <div class="count">${s.user_count || 0} 人</div>
        </a>
    `).join('');
}

/* ---------- 搜索 ---------- */
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

/* ---------- 工具 ---------- */
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

function escapeHtml(str) {
    return String(str == null ? '' : str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;');
}
