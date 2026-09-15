/**
 * CampusWall 图标系统
 * 基于 Lucide Icons 的 SVG 图标管理
 */

// 图标映射表：emoji → Lucide 图标名
const ICON_MAP = {
  // 导航/品牌
  '🏫': 'school',
  '🌸': 'flower-2',
  '❤': 'heart',
  '❤️': 'heart',
  '🤍': 'heart',
  '✨': 'sparkles',
  
  // 功能操作
  '✍️': 'pencil',
  '🔔': 'bell',
  '🔍': 'search',
  '📝': 'file-text',
  '💬': 'message-circle',
  '👁️': 'eye',
  '🔖': 'bookmark',
  '📌': 'pin',
  '✏️': 'pencil',
  '🗑️': 'trash-2',
  '➕': 'plus',
  '🚪': 'log-out',
  '🔑': 'key',
  '↩️': 'reply',
  '📎': 'paperclip',
  '🎭': 'venetian-mask',
  '🏗️': 'building-2',
  '⚙️': 'settings',
  '🔒': 'lock',
  '🚩': 'flag',
  '⚠️': 'alert-triangle',
  
  // 数据/统计
  '👥': 'users',
  '👤': 'user',
  '💰': 'coins',
  '⭐': 'star',
  '📊': 'bar-chart-2',
  '📋': 'clipboard-list',
  '🏷️': 'tag',
  '🎁': 'gift',
  '🎖️': 'medal',
  '🔥': 'flame',
  '📦': 'package',
  
  // 状态/情感
  '😢': 'sad',
  '🎉': 'party-popper',
  '✅': 'check-circle',
  '💕': 'heart-handshake',
  '💝': 'heart',
  '💎': 'gem',
  
  // 分类标签
  '💌': 'mail',
  '📚': 'book-open',
  '🍜': 'utensils',
  '📸': 'camera',
  '🏃': 'running',
  '🎵': 'music',
  '🌳': 'tree-pine',
  '🛒': 'shopping-cart',
  '📱': 'smartphone',
  '🏠': 'home',
  '👗': 'shirt',
  '🌊': 'waves',
  '🎯': 'target',
  '🔮': 'crystal-ball',
  '⏰': 'clock',
  '📅': 'calendar',
  '👩': 'user',
  '👨': 'user',
  '➕': 'plus',
  '➖': 'minus',
  '🔄': 'refresh-cw',
  '📤': 'upload',
  '📥': 'download',
  '🔗': 'link',
  '📧': 'mail',
  '💬': 'message-circle',
  '📍': 'map-pin',
  '🌐': 'globe',
  '📱': 'smartphone',
  '💻': 'laptop',
  '🎮': 'gamepad-2',
  '🎬': 'film',
  '📸': 'camera',
  '🎨': 'palette',
  '📚': 'book-open',
  '✈️': 'plane',
  '🚗': 'car',
  '🏠': 'home',
  '🏢': 'building',
  '🏥': 'hospital',
  '🎓': 'graduation-cap',
  '🏫': 'school',
  '🏬': 'store',
  '🏪': 'store',
  '🍜': 'utensils',
  '🍕': 'pizza',
  '🍔': 'burger',
  '☕': 'coffee',
  '🍺': 'beer',
  '🥤': 'cup-soda',
  '🍰': 'cake',
  '🎁': 'gift',
  '🎄': 'tree-pine',
  '🎃': 'pumpkin',
  '🎄': 'tree-pine',
  '🎉': 'party-popper',
  '🎊': 'confetti-ball',
  '🎈': 'balloon',
  '🎀': 'ribbon',
  '💍': 'ring',
  '💎': 'gem',
  '🔑': 'key',
  '🔒': 'lock',
  '🔓': 'unlock',
  '🔐': 'lock',
  '📍': 'map-pin',
  '🗺️': 'map',
  '🌍': 'globe',
  '🌎': 'globe',
  '🌏': 'globe',
  '🌅': 'sunrise',
  '🌇': 'sunset',
  '🌃': 'night',
  '🌌': 'stars',
  '🌈': 'rainbow',
  '🔥': 'flame',
  '💧': 'droplet',
  '⚡': 'zap',
  '❄️': 'snowflake',
  '☀️': 'sun',
  '🌙': 'moon',
  '⭐': 'star',
  '🌟': 'star',
  '💫': 'sparkles',
  '✨': 'sparkles',
  '💥': 'zap',
  '💢': 'anger',
  '💤': 'moon',
  '💨': 'wind',
  '🕳️': 'circle',
  '💬': 'message-circle',
  '👁️': 'eye',
  '👀': 'eye',
  '👁️‍🗨️': 'eye',
  '🗨️': 'message-circle',
  '🗯️': 'message-square',
  '💭': 'message-circle',
  '🗯️': 'message-square',
  '💰': 'coins',
  '💵': 'banknote',
  '💴': 'banknote',
  '💶': 'banknote',
  '💷': 'banknote',
  '💸': 'banknote',
  '💳': 'credit-card',
  '💹': 'trending-up',
  '📉': 'trending-down',
  '📊': 'bar-chart-2',
  '📋': 'clipboard-list',
  '📁': 'folder',
  '📂': 'folder-open',
  '📅': 'calendar',
  '📇': 'hash',
  '📈': 'trending-up',
  '📉': 'trending-down',
  '📊': 'bar-chart-2',
  '📋': 'clipboard-list',
  '📌': 'pin',
  '📍': 'map-pin',
  '📎': 'paperclip',
  '📏': 'ruler',
  '📐': 'ruler',
  '✂️': 'scissors',
  '🖊️': 'pen',
  '🖋️': 'pen-tool',
  '✒️': 'pen',
  '🖌️': 'brush',
  '🖍️': 'pen-tool',
  '📝': 'file-text',
  '✏️': 'pencil',
  '🔍': 'search',
  '🔎': 'search',
  '🔏': 'lock',
  '🔐': 'lock',
  '🔒': 'lock',
  '🔓': 'unlock',
  '🔑': 'key',
  '🗝️': 'key',
  '🔨': 'hammer',
  '🪓': 'axe',
  '⛏️': 'pickaxe',
  '🔧': 'wrench',
  '🔩': 'nut',
  '⚙️': 'settings',
  '🗜️': 'paperclip',
  '⚖️': 'scale',
  '🔗': 'link',
  '⛓️': 'link',
  '🧰': 'wrench',
  '🧲': 'magnet',
  '🔫': 'gun',
};

// Lucide CDN 基础路径
const LUCIDE_CDN = 'https://unpkg.com/lucide@1.46.0/dist/umd/lucide.min.js';

/**
 * 加载 Lucide Icons
 */
function loadLucideIcons() {
  if (typeof lucide !== 'undefined') {
    lucide.createIcons();
    return;
  }
  
  const script = document.createElement('script');
  script.src = LUCIDE_CDN;
  script.onload = () => {
    lucide.createIcons();
  };
  document.head.appendChild(script);
}

/**
 * 获取图标 HTML
 * @param {string} iconName - 图标名称
 * @param {string} size - 图标大小 (sm, md, lg)
 * @param {string} className - 额外的 CSS 类
 * @returns {string} 图标 HTML
 */
function getIcon(iconName, size = 'md', className = '') {
  const sizeClass = {
    'sm': 'icon-sm',
    'md': 'icon-md',
    'lg': 'icon-lg'
  }[size] || 'icon-md';
  
  return `<i data-lucide="${iconName}" class="icon ${sizeClass} ${className}"></i>`;
}

/**
 * 将 emoji 转换为图标 HTML
 * @param {string} emoji - emoji 字符
 * @param {string} size - 图标大小
 * @returns {string} 图标 HTML
 */
function emojiToIcon(emoji, size = 'md') {
  const iconName = ICON_MAP[emoji] || 'help-circle';
  return getIcon(iconName, size);
}

/**
 * 批量替换页面中的 emoji 为图标
 */
function replaceEmojisInPage() {
  const elements = document.querySelectorAll('[data-emoji]');
  elements.forEach(el => {
    const emoji = el.getAttribute('data-emoji');
    const size = el.getAttribute('data-icon-size') || 'md';
    el.innerHTML = emojiToIcon(emoji, size);
  });
}

// 初始化
document.addEventListener('DOMContentLoaded', () => {
  loadLucideIcons();
  replaceEmojisInPage();
});

// 将子站图标映射到 Lucide 图标名
function getStationIcon(icon) {
  if (!icon) return 'school';
  if (icon.startsWith('lucide:')) return icon.slice(7);
  if (ICON_MAP[icon]) return ICON_MAP[icon];
  // 已是合法 lucide 名（后端 seed/建站的 icon 多为 'book-open'、'gamepad' 这类 kebab 名）
  if (/^[a-z][a-z0-9]*(-[a-z0-9]+)*$/.test(icon) && icon.length <= 30) return icon;
  return 'school';
}

// 导出
window.CampusIcons = {
  getIcon,
  emojiToIcon,
  replaceEmojisInPage,
  loadLucideIcons,
  getStationIcon,
  ICON_MAP
};

// 全局函数供直接调用
window.getStationIcon = getStationIcon;
