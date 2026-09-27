/**
 * 动画层 — 涂鸦背景 + 加载动画 + 滚动淡入
 */

// ── 页边手写涂鸦（纸上校园：像有人在本子边角随手画了几笔）──
function generateDoodles() {
    const layer = document.getElementById('doodleLayer');
    if (!layer) return;
    // 手绘笔触：星号 / 螺旋 / 圈 / 波浪 / 三角，统一墨色低透明度
    const strokes = [
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M12 4v16M5 8l14 8M19 8L5 16"/></svg>',
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M13 11a4 4 0 1 0-3 6.9c3 0 5.5-2 5.5-5S13 8 10 8 5 10 5 13"/></svg>',
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><circle cx="12" cy="12" r="7"/></svg>',
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M3 14c3-5 6 5 9 0s6 5 9 0"/></svg>',
        '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"><path d="M12 5l7 13H5z"/></svg>',
    ];
    // 只在左右页边排布（避开内容区），纵向错落、轻微旋转，静态不闪烁
    const slots = [
        { side: 'left', top: 14 }, { side: 'right', top: 20 },
        { side: 'left', top: 38 }, { side: 'right', top: 46 },
        { side: 'left', top: 62 }, { side: 'right', top: 70 },
        { side: 'left', top: 84 }, { side: 'right', top: 88 },
    ];
    slots.forEach((slot, i) => {
        const d = document.createElement('div');
        d.className = 'margin-doodle';
        d.innerHTML = strokes[i % strokes.length];
        d.style[slot.side] = (Math.random() * 3 + 1.2) + 'vw';
        d.style.top = slot.top + '%';
        d.style.width = d.style.height = (Math.random() * 10 + 14) + 'px';
        d.style.transform = 'rotate(' + (Math.random() * 40 - 20) + 'deg)';
        d.style.opacity = (Math.random() * 0.07 + 0.05).toFixed(3);
        layer.appendChild(d);
    });
}

// ── 加载动画（打字机效果）──
function initLoadingAnimation() {
    const screen = document.getElementById('loadingScreen');
    const logo = document.getElementById('loadingLogo');
    const textEl = document.getElementById('loadingText');
    if (!screen || !logo || !textEl) return Promise.resolve();

    const texts = [
        "正在打开社团活动室的门...",
        "阳光透过百叶窗洒了进来...",
        "欢迎来到校园墙"
    ];

    return new Promise(resolve => {
        let resolved = false;
        function done() {
            if (resolved) return;
            resolved = true;
            screen.classList.add('hidden');
            resolve();
        }

        setTimeout(() => logo.classList.add('show'), 200);

        let textIdx = 0, charIdx = 0;
        function typeWriter() {
            if (textIdx < texts.length) {
                if (charIdx < texts[textIdx].length) {
                    textEl.innerHTML = texts[textIdx].substring(0, charIdx + 1) + '<span class="loading-cursor"></span>';
                    textEl.style.opacity = '1';
                    charIdx++;
                    setTimeout(typeWriter, 50 + Math.random() * 30);
                } else {
                    setTimeout(() => {
                        textIdx++;
                        charIdx = 0;
                        if (textIdx < texts.length) typeWriter();
                        else setTimeout(done, 500);
                    }, 400);
                }
            }
        }

        setTimeout(typeWriter, 600);

        // 安全超时：如果动画卡住，2秒后强制隐藏
        setTimeout(done, 2500);
    });
}

// ── 导航栏显示 ──
function showNavbar() {
    const nav = document.getElementById('navbar');
    if (nav) nav.classList.add('show');
}

// ── 滚动淡入（复用同一 observer）──
let revealObserver = null;

function initReveal() {
    if (!revealObserver) {
        revealObserver = new IntersectionObserver((entries) => {
            entries.forEach(entry => {
                if (entry.isIntersecting) {
                    entry.target.classList.add('visible');
                    revealObserver.unobserve(entry.target);
                }
            });
        }, { threshold: 0.1, rootMargin: '0px 0px -30px 0px' });
    }
    document.querySelectorAll('.reveal:not(.visible)').forEach(el => revealObserver.observe(el));
}

// ── 启动所有动画 ──
async function initAnimations() {
    generateDoodles();

    const isHome = window.location.pathname === '/';
    const played = sessionStorage.getItem('loadingPlayed');
    const screen = document.getElementById('loadingScreen');

    if (!isHome || played) {
        if (screen) screen.classList.add('hidden');
    } else {
        await initLoadingAnimation();
        sessionStorage.setItem('loadingPlayed', '1');
    }

    showNavbar();
    initReveal();
}

// ── 标签页不可见时暂停背景动画 ──
document.addEventListener('visibilitychange', () => {
    const paused = document.hidden ? 'paused' : 'running';
    const layer = document.getElementById('doodleLayer');
    if (layer) {
        layer.style.animationPlayState = paused;
        layer.querySelectorAll('*').forEach(el => {
            el.style.animationPlayState = paused;
        });
    }
    document.querySelectorAll('.bg-glow').forEach(el => {
        el.style.animationPlayState = paused;
    });
});
