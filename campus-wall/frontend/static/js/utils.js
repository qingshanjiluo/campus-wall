/**
 * CampusWall 工具函数模块
 */

// HTML 转义（含引号，可安全用于双引号包裹的属性插值）
function escHtml(str) {
    if (str === 0) return '0';
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

// URL 属性守卫：仅允许站内路径与 http(s)
function safeUrl(url, fallback = '') {
    const s = String(url || '');
    if (/^(\/|https?:\/\/)/i.test(s) && !/^https?:\/\/javascript/i.test(s)) return escHtml(s);
    return fallback;
}

// 显示 Toast（复用 style.css 的 .toast / .toast.show / .toast.success|error 体系）
function showToast(msg, type = 'info', duration = 3000) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    toast.textContent = msg;
    toast.className = 'toast show' + (type && type !== 'info' ? ' ' + type : '');
    clearTimeout(toast._timer);
    toast._timer = setTimeout(() => toast.classList.remove('show'), duration);
}

// 防抖
function debounce(fn, delay = 300) {
    let timer;
    return function (...args) {
        clearTimeout(timer);
        timer = setTimeout(() => fn.apply(this, args), delay);
    };
}

// 节流
function throttle(fn, delay = 100) {
    let last = 0;
    return function (...args) {
        const now = Date.now();
        if (now - last >= delay) {
            last = now;
            return fn.apply(this, args);
        }
    };
}

// 格式化时间
function formatTime(dateStr) {
    if (!dateStr) return '';
    const date = new Date(dateStr);
    const now = new Date();
    const diff = (now - date) / 1000;
    if (diff < 60) return '刚刚';
    if (diff < 3600) return Math.floor(diff / 60) + '分钟前';
    if (diff < 86400) return Math.floor(diff / 3600) + '小时前';
    if (diff < 2592000) return Math.floor(diff / 86400) + '天前';
    return date.toLocaleDateString('zh-CN');
}

// 格式化数字
function formatNumber(num) {
    if (!num) return '0';
    if (num >= 10000) return (num / 10000).toFixed(1) + '万';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'k';
    return num.toString();
}

// 生成随机 ID
function generateId() {
    return Math.random().toString(36).substr(2, 9);
}

// 复制到剪贴板
async function copyToClipboard(text) {
    try {
        await navigator.clipboard.writeText(text);
        showToast('已复制到剪贴板', 'success');
        return true;
    } catch (e) {
        showToast('复制失败', 'error');
        return false;
    }
}

// 截断文本
function truncate(str, maxLen = 100) {
    if (!str || str.length <= maxLen) return str;
    return str.substring(0, maxLen) + '...';
}

window.CampusUtils = {
    escHtml,
    safeUrl,
    showToast,
    debounce,
    throttle,
    formatTime,
    formatNumber,
    generateId,
    copyToClipboard,
    truncate
};
