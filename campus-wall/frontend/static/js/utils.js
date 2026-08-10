/**
 * CampusWall 工具函数模块
 */

// HTML 转义
function escHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
}

// 显示 Toast
function showToast(msg, type = 'info', duration = 3000) {
    const toast = document.getElementById('toast');
    if (!toast) return;
    const colors = {
        success: 'var(--success)',
        error: 'var(--danger)',
        warning: 'var(--warning)',
        info: 'var(--info)'
    };
    toast.textContent = msg;
    toast.style.cssText = `
        position: fixed; bottom: 24px; left: 50%; transform: translateX(-50%);
        padding: 12px 24px; border-radius: 12px; font-size: 0.9rem; font-weight: 500;
        background: ${colors[type] || colors.info}; color: white;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15); z-index: 10000;
        animation: toast-in 0.3s ease;
    `;
    clearTimeout(toast._timer);
    toast._timer = setTimeout(() => {
        toast.style.animation = 'toast-out 0.3s ease forwards';
    }, duration);
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
    showToast,
    debounce,
    throttle,
    formatTime,
    formatNumber,
    generateId,
    copyToClipboard,
    truncate
};
