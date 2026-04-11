// ============================================================
//  auth.js — вход и регистрация
//  Требует подключённого utils.js (API_BASE, apiFetch)
// ============================================================

// ------ ФОРМА ВХОДА ---------------------------------------
const loginForm = document.getElementById('loginForm');
if (loginForm) {
    loginForm.addEventListener('submit', async function (event) {
        event.preventDefault();

        const username = document.getElementById('username').value.trim();
        const password = document.getElementById('password').value;
        const btn = loginForm.querySelector('button[type="submit"]');

        btn.disabled = true;
        btn.textContent = 'Жүктелуде...';

        try {
            const response = await fetch(`${API_BASE}/api/v1/auth/login`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || data.message || 'Кіру қатесі');
            }

            // Сохраняем токен и имя пользователя
            localStorage.setItem('token', data.token);
            localStorage.setItem('username', data.username || username);

            // Плавный переход в главное меню
            document.body.style.transition = 'opacity 0.4s';
            document.body.style.opacity = '0';
            setTimeout(() => { window.location.href = 'main.html'; }, 400);

        } catch (error) {
            alert(error.message || 'Белгісіз қате. Қайталап көріңіз.');
            btn.disabled = false;
            btn.textContent = 'Жүйеге кіру';
        }
    });
}


// ------ ФОРМА РЕГИСТРАЦИИ ---------------------------------
const registerForm = document.getElementById('registerForm');
if (registerForm) {
    registerForm.addEventListener('submit', async function (event) {
        event.preventDefault();

        const username = document.getElementById('reg-username').value.trim();
        const password = document.getElementById('reg-password').value;
        const confirm  = document.getElementById('reg-confirm').value;
        const btn = registerForm.querySelector('button[type="submit"]');

        if (password !== confirm) {
            alert('Құпия сөздер сәйкес келмейді!');
            return;
        }

        if (username.length < 3) {
            alert('Пайдаланушы аты кемінде 3 таңбадан тұруы керек.');
            return;
        }

        if (password.length < 6) {
            alert('Құпия сөз кемінде 6 таңбадан тұруы керек.');
            return;
        }

        btn.disabled = true;
        btn.textContent = 'Жүктелуде...';

        try {
            const response = await fetch(`${API_BASE}/api/v1/auth/register`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ username, password })
            });

            const data = await response.json();

            if (!response.ok) {
                throw new Error(data.error || data.message || 'Тіркелу қатесі');
            }

            // После успешной регистрации — сразу логиним
            localStorage.setItem('token', data.token);
            localStorage.setItem('username', data.username || username);

            document.body.style.transition = 'opacity 0.4s';
            document.body.style.opacity = '0';
            setTimeout(() => { window.location.href = 'main.html'; }, 400);

        } catch (error) {
            alert(error.message || 'Белгісіз қате. Қайталап көріңіз.');
            btn.disabled = false;
            btn.textContent = 'Аккаунты құру';
        }
    });
}