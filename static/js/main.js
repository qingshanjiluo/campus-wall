// ===== 全局状态 =====
let stationsData = [];
let postsData = [];

// ===== Toast 提示 =====
function showToast(message, duration = 2500) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = message;
    toast.classList.add('show');
    clearTimeout(toast._hideTimer);
    toast._hideTimer = setTimeout(() => {
        toast.classList.remove('show');
    }, duration);
}

// ===== 加载子站数据 =====
function loadStations() {
    const grid = document.getElementById('stationGrid');
    grid.innerHTML = '<div class="loading">加载中...</div>';

    fetch('/api/stations')
        .then(res => {
            if (!res.ok) throw new Error('网络请求失败');
            return res.json();
        })
        .then(data => {
            stationsData = data;
            renderStations(data);
        })
        .catch(err => {
            console.error('加载子站失败:', err);
            grid.innerHTML = `<div class="loading" style="color:rgba(255,255,255,0.5);">加载失败，请刷新重试</div>`;
            showToast('加载子站失败，请检查网络');
        });
}

// ===== 渲染子站 =====
function renderStations(stations) {
    const grid = document.getElementById('stationGrid');
    if (!stations || stations.length === 0) {
        grid.innerHTML = '<div class="loading">暂无子站</div>';
        return;
    }

    grid.innerHTML = stations.map(station => `
        <div class="station-card animate__animated animate__fadeInUp" 
             onclick="showToast('进入 ${station.name}')">
            <div class="cover">
                ${station.cover && station.cover !== '/static/images/default-cover.jpg' 
                    ? `<img src="${station.cover}" alt="${station.name}">` 
                    : ''}
            </div>
            <div class="name">${station.name}</div>
            <div class="desc">${station.description || '暂无简介'}</div>
            <div class="tags">
                ${(station.tags || []).map(tag => `<span class="tag">#${tag}</span>`).join('')}
            </div>
            <div class="meta">
                <span> ${station.user_count || 0}</span>
                <span> ${station.post_count || 0}</span>
            </div>
        </div>
    `).join('');
}

// ===== 加载帖子数据 =====
function loadPosts() {
    const list = document.getElementById('postList');
    list.innerHTML = '<div class="loading">加载中...</div>';

    fetch('/api/posts?limit=6')
        .then(res => {
            if (!res.ok) throw new Error('网络请求失败');
            return res.json();
        })
        .then(data => {
            postsData = data;
            renderPosts(data);
        })
        .catch(err => {
            console.error('加载帖子失败:', err);
            list.innerHTML = `<div class="loading" style="color:rgba(255,255,255,0.5);">加载失败，请刷新重试</div>`;
        });
}

// ===== 渲染帖子 =====
function renderPosts(posts) {
    const list = document.getElementById('postList');
    if (!posts || posts.length === 0) {
        list.innerHTML = '<div class="loading">暂无动态</div>';
        return;
    }

    list.innerHTML = posts.map(post => `
        <div class="post-item animate__animated animate__fadeInUp">
            <div class="title">${post.title}</div>
            <div class="content">${post.content || '暂无内容'}</div>
            <div class="meta">
                <span class="station-name"> ${post.station_name}</span>
                <span> ${post.author || '匿名'}</span>
                <span>️ ${post.views || 0}</span>
                <span>❤️ ${post.likes || 0}</span>
                <span> ${post.comments || 0}</span>
                <span> ${formatTime(post.created_at)}</span>
            </div>
        </div>
    `).join('');
}

// ===== 格式化时间 =====
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
    } catch (e) {
        return '刚刚';
    }
}

// ===== 搜索功能 =====
function initSearch() {
    const input = document.getElementById('searchInput');
    const btn = document.getElementById('searchBtn');

    const doSearch = () => {
        const keyword = input.value.trim();
        if (!keyword) {
            renderStations(stationsData);
            return;
        }

        fetch(`/api/search?q=${encodeURIComponent(keyword)}`)
            .then(res => {
                if (!res.ok) throw new Error('搜索请求失败');
                return res.json();
            })
            .then(data => {
                renderStations(data);
                if (data.length === 0) {
                    showToast('未找到相关子站');
                }
            })
            .catch(err => {
                console.error('搜索失败:', err);
                showToast('搜索失败，请重试');
            });
    };

    btn.addEventListener('click', doSearch);
    input.addEventListener('keypress', (e) => {
        if (e.key === 'Enter') doSearch();
    });
}

// ===== 页面加载动画 =====
document.addEventListener('DOMContentLoaded', function() {
    // 使用 Anime.js 为卡片添加入场动画
    setTimeout(() => {
        const cards = document.querySelectorAll('.station-card, .post-item');
        if (cards.length && typeof anime !== 'undefined') {
            anime({
                targets: cards,
                opacity: [0, 1],
                translateY: [30, 0],
                duration: 600,
                delay: (el, i) => i * 80,
                easing: 'easeOutCubic'
            });
        }
    }, 300);
});
