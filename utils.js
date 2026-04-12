// ============================================================
//  utils.js — общий модуль для всех страниц проекта
//  Подключать ПЕРВЫМ тегом <script> на каждой странице
// ============================================================
 
// ------ 1. БАЗОВЫЙ URL БЭКЕНДА ----------------------------
const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? 'http://localhost:8080'
    : 'https://kenesary-server.onrender.com';
 
// ------ 2. ЗАГОЛОВКИ С ТОКЕНОМ ----------------------------
function authHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token') || ''}`
    };
}
 
// ------ 3. ОБЁРТКА НАД fetch ------------------------------
async function apiFetch(endpoint, options = {}) {
    const url = API_BASE + endpoint;
    const config = {
        ...options,
        headers: { ...authHeaders(), ...(options.headers || {}) }
    };
    const response = await fetch(url, config);
    if (response.status === 401) {
        localStorage.removeItem('token');
        localStorage.removeItem('username');
        window.location.href = 'login.html';
        return;
    }
    if (!response.ok) {
        let errMsg = `HTTP ${response.status}`;
        try {
            const errData = await response.json();
            errMsg = errData.error || errData.message || errMsg;
        } catch (_) {}
        throw new Error(errMsg);
    }
    return response;
}
 
// ------ 4. AUTH GUARD -------------------------------------
function requireAuth() {
    if (!localStorage.getItem('token')) {
        window.location.href = 'login.html';
    }
}
 
// ------ 5. ВЫХОД (LOGOUT) ---------------------------------
function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    localStorage.removeItem('cachedAvatarUrl');
    localStorage.removeItem('cachedAvatarId');
    window.location.href = 'login.html';
}
 
// ------ 6. АНИМАЦИЯ ЧИСЛА ---------------------------------
function animateValue(id, start, end, duration) {
    const obj = document.getElementById(id);
    if (!obj) return;
    if (start === end) { obj.innerHTML = end.toLocaleString('ru-RU'); return; }
    const range = end - start;
    const increment = end > start ? Math.ceil(range / (duration / 30)) : Math.floor(range / (duration / 30));
    let current = start;
    const timer = setInterval(() => {
        current += increment;
        if ((increment > 0 && current >= end) || (increment < 0 && current <= end)) {
            current = end; clearInterval(timer);
        }
        obj.innerHTML = current.toLocaleString('ru-RU');
    }, 30);
}
 
// ------ 7. ПЛАВНЫЙ ПЕРЕХОД МЕЖДУ СТРАНИЦАМИ --------------
function initPageTransitions() {
    document.querySelectorAll('a').forEach(link => {
        link.addEventListener('click', function (e) {
            const href = this.getAttribute('href');
            if (href && href !== '#' && this.hostname === window.location.hostname && this.target !== '_blank') {
                e.preventDefault();
                document.body.style.transition = 'opacity 0.4s';
                document.body.style.opacity = '0';
                setTimeout(() => { window.location.href = this.href; }, 400);
            }
        });
    });
}
document.addEventListener('DOMContentLoaded', initPageTransitions);

// ------ 8. КАРТА URL АВАТАРОК ----------------------------
//  ID-лер сервердегі allAvatars массивімен ДӘЛМЕ-ДӘЛ сәйкес болуы керек:
//  common_1, common_2, common_3
//  rare_1, rare_2, rare_3
//  epic_1, epic_2
//  legendary_1, legendary_2

const DEFAULT_AVATAR_URL = 'https://cdn-icons-png.flaticon.com/512/149/149071.png';

const AVATAR_URLS = {
    'common_1':    'ава1.png',
    'common_2':    '3a18b683-5363-4267-a890-59cc2f5f6d87.jpeg',
    'common_3':    'ава3.jpeg',
    'rare_1':      'a7d505b9-5b86-4057-8b87-9348b5983ddb.jpeg',
    'rare_2':      'ава5.jpeg',
    'rare_3':      'ава6.jpeg',
    'epic_1':      'ава 7.jpeg',
    'epic_2':      'ава8.jpeg',
    'legendary_1': 'ава9.jpeg',
    'legendary_2': 'ава10.jpeg',   // ← было legendary_3, исправлено на legendary_2
};

function resolveAvatarUrl(avatarId) {
    return AVATAR_URLS[avatarId] || DEFAULT_AVATAR_URL;
}

// ------ 9. АВАТАРКА В САЙДБАРЕ ----------------------------

// Редкость по ID аватарки
function getAvatarRarity(avatarId) {
    if (!avatarId) return 'common';
    if (avatarId.startsWith('legendary')) return 'legendary';
    if (avatarId.startsWith('epic'))      return 'epic';
    if (avatarId.startsWith('rare'))      return 'rare';
    return 'common';
}

// Цвет подсветки по редкости
function getRarityGlowColor(rarity) {
    const map = {
        common:    '#94a3b8',
        rare:      '#3b82f6',   // синий
        epic:      '#a855f7',   // фиолетовый
        legendary: '#D4AF37',   // золотой
    };
    return map[rarity] || map.common;
}

// Применить rarity-стиль к img-элементу аватарки
function applyRarityGlow(imgEl, avatarId) {
    if (!imgEl) return;
    const rarity = getAvatarRarity(avatarId);
    const color  = getRarityGlowColor(rarity);
    imgEl.style.borderColor = color;
    imgEl.style.boxShadow   = `0 0 12px ${color}CC, 0 0 24px ${color}66`;
}

async function loadSidebarAvatar() {
    const imgs = [
        document.getElementById('sidebarAvatar'),
        document.getElementById('sidebarAvatarDisplay')
    ].filter(Boolean);
    if (imgs.length === 0) return;

    const cachedUrl = localStorage.getItem('cachedAvatarUrl');
    const cachedId  = localStorage.getItem('cachedAvatarId');
    if (cachedUrl) {
        imgs.forEach(img => {
            img.src = cachedUrl;
            applyRarityGlow(img, cachedId);
        });
    }

    try {
        const res = await apiFetch('/api/v1/profile');
        const data = await res.json();

        let url = data.avatar_url || null;
        if (!url && data.avatar) {
            url = resolveAvatarUrl(data.avatar);
        }
        if (!url) url = DEFAULT_AVATAR_URL;

        imgs.forEach(img => {
            img.src = url;
            applyRarityGlow(img, data.avatar);
        });
        localStorage.setItem('cachedAvatarUrl', url);
        if (data.avatar) localStorage.setItem('cachedAvatarId', data.avatar);
    } catch (_) { /* используем кеш */ }
}

// ------ 10. ИМЕНА АВАТАРОК --------------------------------
//  ID-лер сервердегі allAvatars-пен сәйкес
const AVATAR_NAMES = {
    'common_1':    'Дала Жауынгері',
    'common_2':    'Күзетші',
    'common_3':    'Садақшы',
    'rare_1':      'Сарбаз Басшы',
    'rare_2':      'Ат Жауынгер',
    'rare_3':      'Найзагер',
    'epic_1':      'Хан Нөкері',
    'epic_2':      'Темір Қалқан',
    'legendary_1': 'Кенесары Хан',
    'legendary_2': 'Аруана Ханым',
};

function getAvatarName(avatarId) {
    return AVATAR_NAMES[avatarId] || 'Белгісіз Жауынгер';
}

// ------ 11. АНИМАЦИЯ МОНЕТ --------------------------------
function spawnCoinsAnimation(anchorEl, coinsCount) {
    const rect = anchorEl
        ? anchorEl.getBoundingClientRect()
        : { left: window.innerWidth / 2, top: window.innerHeight / 2, width: 0, height: 0 };
    const originX = rect.left + rect.width / 2;
    const originY = rect.top + rect.height / 2;

    const count = Math.min(28, Math.max(8, Math.floor(coinsCount / 15)));

    let container = document.getElementById('_coinAnimContainer');
    if (!container) {
        container = document.createElement('div');
        container.id = '_coinAnimContainer';
        container.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;pointer-events:none;z-index:99999;overflow:hidden;';
        document.body.appendChild(container);
    }

    for (let i = 0; i < count; i++) {
        const coin = document.createElement('div');
        const angle = (Math.random() * 360) * (Math.PI / 180);
        const speed = 90 + Math.random() * 170;
        const dx = Math.cos(angle) * speed;
        const dy = -Math.abs(Math.sin(angle)) * speed - 80;
        const size = 16 + Math.random() * 16;
        const dur = 0.65 + Math.random() * 0.55;
        const delay = Math.random() * 0.3;
        const uid = Date.now() + '_' + i;

        coin.style.cssText = `
            position:absolute;left:${originX}px;top:${originY}px;
            width:${size}px;height:${size}px;border-radius:50%;
            background:radial-gradient(circle at 35% 35%,#ffe566,#D4AF37 60%,#a07800);
            box-shadow:0 0 8px #D4AF3799,inset 0 -2px 4px rgba(0,0,0,0.3);
            transform:translate(-50%,-50%);
            animation:_cf${uid} ${dur}s ease-out ${delay}s forwards;
            display:flex;align-items:center;justify-content:center;
            font-size:9px;color:#7a5a00;font-weight:900;line-height:1;
        `;
        coin.textContent = '₸';

        const style = document.createElement('style');
        style.textContent = `@keyframes _cf${uid}{0%{transform:translate(-50%,-50%) scale(0.4);opacity:1}60%{opacity:1}100%{transform:translate(calc(-50% + ${dx}px),calc(-50% + ${dy}px)) scale(1.1);opacity:0}}`;
        document.head.appendChild(style);
        container.appendChild(coin);

        const totalMs = (dur + delay + 0.15) * 1000;
        setTimeout(() => { coin.remove(); style.remove(); }, totalMs);
    }

    const uid2 = Date.now();
    const popup = document.createElement('div');
    popup.textContent = `+${coinsCount.toLocaleString('ru-RU')} ТГ`;
    popup.style.cssText = `
        position:fixed;left:${originX}px;top:${originY - 20}px;
        transform:translateX(-50%);
        font-family:'Montserrat',sans-serif;font-size:24px;font-weight:800;
        color:#D4AF37;text-shadow:0 0 16px rgba(212,175,55,0.9),0 2px 5px rgba(0,0,0,0.9);
        pointer-events:none;z-index:100000;
        animation:_pp${uid2} 1.3s ease-out forwards;
    `;
    const ps = document.createElement('style');
    ps.textContent = `@keyframes _pp${uid2}{0%{opacity:0;transform:translateX(-50%) translateY(0) scale(0.6)}20%{opacity:1;transform:translateX(-50%) translateY(-15px) scale(1.15)}70%{opacity:1;transform:translateX(-50%) translateY(-50px) scale(1)}100%{opacity:0;transform:translateX(-50%) translateY(-80px) scale(0.9)}}`;
    document.head.appendChild(ps);
    document.body.appendChild(popup);
    setTimeout(() => { popup.remove(); ps.remove(); }, 1500);
}