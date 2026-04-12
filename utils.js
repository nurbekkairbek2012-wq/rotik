// ============================================================
//  utils.js — общий модуль для всех страниц проекта
//  Подключать ПЕРВЫМ тегом <script> на каждой странице:
//  <script src="utils.js"></script>
// ============================================================
 
// ------ 1. БАЗОВЫЙ URL БЭКЕНДА ----------------------------
//  На локальной машине будет localhost:8080
//  На продакшн-сервере подставится реальный домен
const API_BASE = window.location.hostname === 'localhost' || window.location.hostname === '127.0.0.1'
    ? 'http://localhost:8080'
    : 'https://kenesary-server.onrender.com'; // <- поменяй на свой домен при деплое
 
 
// ------ 2. ЗАГОЛОВКИ С ТОКЕНОМ ----------------------------
//  Использовать во всех fetch-запросах к защищённым эндпоинтам
function authHeaders() {
    return {
        'Content-Type': 'application/json',
        'Authorization': `Bearer ${localStorage.getItem('token') || ''}`
    };
}
 
 
// ------ 3. ОБЁРТКА НАД fetch ------------------------------
//  - Автоматически добавляет Authorization заголовок
//  - При 401 (токен истёк/недействителен) — чистит хранилище и редиректит на логин
//  - Бросает ошибку если ответ не ok, чтобы catch в вызывающем коде её поймал
async function apiFetch(endpoint, options = {}) {
    const url = API_BASE + endpoint;
 
    const config = {
        ...options,
        headers: {
            ...authHeaders(),
            ...(options.headers || {})
        }
    };
 
    const response = await fetch(url, config);
 
    if (response.status === 401) {
        localStorage.removeItem('token');
        localStorage.removeItem('username');
        window.location.href = 'login.html';
        return; // прерываем выполнение
    }
 
    if (!response.ok) {
        // Пробуем прочитать тело ошибки от сервера
        let errMsg = `HTTP ${response.status}`;
        try {
            const errData = await response.json();
            errMsg = errData.error || errData.message || errMsg;
        } catch (_) { /* тело не JSON — оставляем HTTP-статус */ }
        throw new Error(errMsg);
    }
 
    return response;
}
 
 
// ------ 4. AUTH GUARD -------------------------------------
//  Вызывать на каждой защищённой странице (все кроме login.html и register.html)
//  Если токена нет — сразу редирект, страница не отрисуется
function requireAuth() {
    if (!localStorage.getItem('token')) {
        window.location.href = 'login.html';
    }
}
 
 
// ------ 5. ВЫХОД (LOGOUT) ---------------------------------
function logout() {
    localStorage.removeItem('token');
    localStorage.removeItem('username');
    window.location.href = 'login.html';
}
 
 
// ------ 6. АНИМАЦИЯ ЧИСЛА (используется везде) -----------
//  animateValue('elementId', 0, 15400, 1000)
function animateValue(id, start, end, duration) {
    const obj = document.getElementById(id);
    if (!obj) return;
    if (start === end) {
        obj.innerHTML = end.toLocaleString('ru-RU');
        return;
    }
    const range = end - start;
    const increment = end > start
        ? Math.ceil(range / (duration / 30))
        : Math.floor(range / (duration / 30));
    let current = start;
    const timer = setInterval(() => {
        current += increment;
        if ((increment > 0 && current >= end) || (increment < 0 && current <= end)) {
            current = end;
            clearInterval(timer);
        }
        obj.innerHTML = current.toLocaleString('ru-RU');
    }, 30);
}
 
 
// ------ 7. ПЛАВНЫЙ ПЕРЕХОД МЕЖДУ СТРАНИЦАМИ --------------
//  Автоматически вешается на все <a> при загрузке DOM
function initPageTransitions() {
    document.querySelectorAll('a').forEach(link => {
        link.addEventListener('click', function (e) {
            const href = this.getAttribute('href');
            if (
                href && href !== '#' &&
                this.hostname === window.location.hostname &&
                this.target !== '_blank'
            ) {
                e.preventDefault();
                document.body.style.transition = 'opacity 0.4s';
                document.body.style.opacity = '0';
                setTimeout(() => { window.location.href = this.href; }, 400);
            }
        });
    });
}
 
document.addEventListener('DOMContentLoaded', initPageTransitions);


// ------ 8. АВАТАРКА В САЙДБАРЕ (используется на всех страницах) ----
//  Вызывать после DOMContentLoaded на каждой защищённой странице.
//  Находит первый элемент с id="sidebarAvatar" и проставляет src из API.
async function loadSidebarAvatar() {
    const img = document.getElementById('sidebarAvatar');
    if (!img) return;
    try {
        const res = await apiFetch('/api/v1/profile');
        const data = await res.json();
        if (data.avatar_url) {
            img.src = data.avatar_url;
        }
    } catch (_) { /* молчим — дефолтная картинка уже стоит */ }
}