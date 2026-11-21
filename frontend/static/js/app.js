// Tour Tracker — SPA JavaScript
const API = '/api';

// Router
const routes = {
    '/': 'home',
    '/tours': 'tours',
    '/about': 'about',
};

function navigate(path) {
    history.pushState(null, '', path);
    render();
}

function render() {
    const path = window.location.pathname;
    const page = routes[path] || 'home';

    // Update nav
    document.querySelectorAll('.nav-link').forEach(link => {
        link.classList.toggle('active', link.dataset.page === page);
    });

    const app = document.getElementById('app');

    switch (page) {
        case 'home':
            app.innerHTML = renderHome();
            loadHotTours();
            break;
        case 'tours':
            app.innerHTML = renderToursPage();
            loadAllTours();
            break;
        case 'about':
            app.innerHTML = renderAbout();
            break;
        default:
            app.innerHTML = renderHome();
            loadHotTours();
    }

    // Setup form
    const form = document.getElementById('subscribeForm');
    if (form) {
        form.addEventListener('submit', handleSubscribe);
    }

    // Setup filters
    const filterBtn = document.getElementById('filterBtn');
    if (filterBtn) {
        filterBtn.addEventListener('click', filterTours);
    }
}

// ===== Home Page =====
function renderHome() {
    return `
        <section class="hero">
            <div class="hero-content">
                <p class="hero-tag">TOUR TRACKER</p>
                <h1>TOUR TRACKER</h1>
                <p class="hero-subtitle">Мы просто ищем лучшие возможности вашей мечты!</p>
                    <p class="hero-desc">Для Вас бесплатно!</p>
            </div>
            
            <div class="search-form">
                <form id="heroForm">
                    <div class="search-field">
                        <label>Назначение</label>
                        <input type="text" id="heroDest" placeholder="Страна">
                    </div>
                    <div class="search-field">
                        <label>Выберите свой тур</label>
                        <select id="heroType">
                            <option value="">Все туры</option>
                            <option value="hot">🔥 Горящие</option>
                            <option value="early">📅 Раннее бронирование</option>
                            <option value="last">⚡ Last Minute</option>
                        </select>
                    </div>
                    <div class="search-field">
                        <label>Дата</label>
                        <input type="date" id="heroDate">
                    </div>
                    <button type="button" onclick="handleHeroSearch()" class="btn btn-primary">Найти 🔍</button>
                </form>
            </div>
        </section>

        <section class="popular-destinations">
            <div class="section-header">
                <h2>ПОПУЛЯРНЫЕ НАПРАВЛЕНИЯ</h2>
                <p>Откройте для себя лучшие места для отдыха</p>
            </div>
            <div class="destinations-grid">
                <div class="destination-card">
                    <img src="/static/images/destinations/Turkey.jpg" alt="Турция">
                    <div class="destination-overlay">
                        <h3>🇹🇷 Турция</h3>
                    </div>
                </div>
                <div class="destination-card">
                    <img src="/static/images/destinations/OAE.jpg" alt="ОАЭ">
                    <div class="destination-overlay">
                        <h3>🇦🇪 ОАЭ</h3>
                    </div>
                </div>
                <div class="destination-card">
                    <img src="/static/images/destinations/Tayland.jpg" alt="Таиланд">
                    <div class="destination-overlay">
                        <h3>🇹🇭 Таиланд</h3>
                    </div>
                </div>
                <div class="destination-card">
                    <img src="/static/images/destinations/Egypt.jpg" alt="Египет">
                    <div class="destination-overlay">
                        <h3>🇪🇬 Египет</h3>
                    </div>
                </div>
            </div>
        </section>

        <section class="subscribe-section" id="subscribe">
            <div class="subscribe-card">
                <h2>✨ Добавить подписку</h2>
                <form id="subscribeForm">
                    <div class="form-group">
                        <label>Город вылета</label>
                        <input type="text" name="origin_city" placeholder="Москва" required maxlength="100">
                    </div>
                    <div class="form-group">
                        <label>Направление</label>
                        <input type="text" name="destination" placeholder="Анталья, Турция" required maxlength="100">
                    </div>
                    <div class="form-row">
                        <div class="form-group">
                            <label>Дата вылета</label>
                            <input type="date" name="date_from">
                        </div>
                        <div class="form-group">
                            <label>Кол-во ночей</label>
                            <input type="number" name="nights" value="7" min="1" max="30">
                        </div>
                    </div>
                    <div class="form-group">
                        <label>Макс. бюджет на человека (₽)</label>
                        <input type="number" name="max_price" placeholder="Не ограничено">
                    </div>
                    <div class="form-row">
                        <div class="form-group">
                            <label>Мессенджер</label>
                            <select name="messenger" required>
                                <option value="telegram">Telegram</option>
                                <option value="max">MAX</option>
                            </select>
                        </div>
                        <div class="form-group">
                            <label>Username или ID</label>
                            <input type="text" name="user_identifier" placeholder="@username или chat_id" required>
                        </div>
                    </div>
                    <button type="submit" class="btn btn-primary" style="width:100%;">Добавить подписку</button>
                    <div class="form-message" id="formMessage"></div>
                </form>
            </div>
        </section>

        <section class="hot-tours">
            <div class="section-header">
                <h2>ГОРЯЩИЕ ТУРЫ</h2>
                <p>Лучшие предложения, которые нужно бронировать прямо сейчас</p>
            </div>
            <div class="tours-grid" id="hotToursGrid">
                <div class="tour-loading">Загрузка...</div>
            </div>
        </section>

        <section class="how-it-works">
            <div class="section-header">
                <h2>КАК ЭТО РАБОТАЕТ</h2>
            </div>
            <div class="steps-grid">
                <div class="step">
                    <div class="step-icon">📍</div>
                    <h3>Укажите направление</h3>
                    <p>Куда, когда и за какой бюджет</p>
                </div>
                <div class="step">
                    <div class="step-icon">📉</div>
                    <h3>Мы мониторим цены</h3>
                    <p>Постоянно отслеживаем предложения туроператоров</p>
                </div>
                <div class="step">
                    <div class="step-icon">🔔</div>
                    <h3>Получаете уведомление</h3>
                    <p>Когда цена упадёт — пишем в Telegram</p>
                </div>
            </div>
        </section>
    `;
}

// ===== Tours Page =====
function renderToursPage() {
    return `
        <section class="tours-page">
            <div class="section-header">
                <h1>ГОРЯЩИЕ ТУРЫ</h1>
                <p>Лучшие предложения, которые нужно бронировать прямо сейчас</p>
            </div>
            <div class="tour-filters">
                <input type="text" id="filterCity" placeholder="🔍 Поиск по направлению...">
                <select id="filterNights">
                    <option value="">Любое кол-во ночей</option>
                    <option value="5">5-7 ночей</option>
                    <option value="7">7-10 ночей</option>
                    <option value="10">10-14 ночей</option>
                    <option value="14">14+ ночей</option>
                </select>
                <button class="btn btn-primary" id="filterBtn">Фильтровать</button>
            </div>
            <div class="tours-grid" id="allToursGrid">
                <div class="tour-loading">Загрузка...</div>
            </div>
        </section>
    `;
}

// ===== About Page =====
function renderAbout() {
    return `
        <section class="about-page">
            <h1>О сервисе</h1>

            <div class="about-section">
                <h2>Как это работает?</h2>
                <p>Tour Tracker — это бесплатный сервис, который помогает находить лучшие цены на пакетные туры. Вот как это работает:</p>
                <ul>
                    <li>Вы добавляете направление и параметры поиска (город, даты, бюджет)</li>
                    <li>Наш сервис постоянно мониторит цены от ведущих туроператоров</li>
                    <li>Когда цена падает — вы получаете уведомление в Telegram</li>
                    <li>Бронируете по ссылке — мы получаем комиссию, вы экономите</li>
                </ul>
            </div>

            <div class="about-section">
                <h2>Почему бесплатно?</h2>
                <p>Мы зарабатываем на партнёрских комиссиях с бронирований. Это значит, что для вас сервис полностью бесплатен — никаких подписок и скрытых платежей.</p>
            </div>

            <div class="about-section">
                <h2>Кого мониторим?</h2>
                <p>Мы отслеживаем цены от ведущих туроператоров и агрегаторов:</p>
                <ul>
                    <li>Travelata — один из крупнейших агрегаторов туров в РФ</li>
                    <li>Level.Travel — поиск туров от множества туроператоров</li>
                    <li>И другие партнёры (в процессе подключения)</li>
                </ul>
            </div>

            <div class="about-section">
                <h2>Направления</h2>
                <p>Сейчас мы мониторим самые популярные направления из России:</p>
                <ul>
                    <li>🇹🇷 Турция — Анталья, Бодрум, Стамбул</li>
                    <li>🇪🇬 Египет — Хургада, Шарм-эль-Шейх</li>
                    <li>🇦🇪 ОАЭ — Дубай, Абу-Даби</li>
                    <li>🇹🇭 Таиланд — Пхукет, Паттайя, Бангкок</li>
                    <li>🇷🇺 Россия — Сочи, Абхазия</li>
                </ul>
            </div>

            <div class="about-section">
                <h2>Контакты</h2>
                <p>По вопросам: <a href="mailto:support@tourtracker.example.com" style="color:var(--primary);">support@tourtracker.example.com</a></p>
                <p style="margin-top:12px;">Telegram-бот: <a href="https://t.me/your_bot_username" style="color:var(--primary);">@your_bot_username</a></p>
            </div>
        </section>
    `;
}

// ===== Load Hot Tours =====
async function loadHotTours() {
    const grid = document.getElementById('hotToursGrid');
    if (!grid) return;

    try {
        const res = await fetch(`${API}/tours/hot`);
        const data = await res.json();

        if (!data.tours || data.tours.length === 0) {
            grid.innerHTML = '<div class="tour-loading">Пока нет предложений. Добавьте подписку и мы уведомим вас первыми!</div>';
            return;
        }

        grid.innerHTML = data.tours.map(tour => renderTourCard(tour)).join('');
    } catch (err) {
        grid.innerHTML = '<div class="tour-loading">Ошибка загрузки. Попробуйте позже.</div>';
    }
}

async function loadAllTours() {
    const grid = document.getElementById('allToursGrid');
    if (!grid) return;

    try {
        const res = await fetch(`${API}/tours/hot`);
        const data = await res.json();

        if (!data.tours || data.tours.length === 0) {
            grid.innerHTML = '<div class="tour-loading">Пока нет предложений</div>';
            return;
        }

        grid.dataset.allTours = JSON.stringify(data.tours);
        grid.innerHTML = data.tours.map(tour => renderTourCard(tour)).join('');
    } catch (err) {
        grid.innerHTML = '<div class="tour-loading">Ошибка загрузки. Попробуйте позже.</div>';
    }
}

// ===== Render Tour Card =====
function renderTourCard(tour) {
    const priceFormatted = tour.price.toLocaleString('ru-RU') + ' ₽';
    
    // Generate a gradient based on destination
    const gradients = [
        'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
        'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
        'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)',
        'linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)',
        'linear-gradient(135deg, #fa709a 0%, #fee140 100%)',
    ];
    const gradient = gradients[Math.abs(tour.id || 0) % gradients.length];

    return `
        <div class="tour-card">
            <div class="tour-card-image" style="background: ${gradient};">
                <div class="tour-route">${tour.origin_city} → ${tour.destination}</div>
            </div>
            <div class="tour-card-body">
                <div class="tour-info">
                    <div class="tour-info-item">
                        <span class="label">Отель</span>
                        <span class="value">${tour.hotel_name}</span>
                    </div>
                    <div class="tour-info-item">
                        <span class="label">Звёзды</span>
                        <span class="value">${'⭐'.repeat(tour.hotel_stars)}</span>
                    </div>
                    <div class="tour-info-item">
                        <span class="label">Ночей</span>
                        <span class="value">${tour.nights}</span>
                    </div>
                </div>
                <div class="tour-price">${priceFormatted}</div>
                <a href="${tour.affiliate_url || tour.tour_url}" target="_blank" class="btn btn-primary">Забронировать</a>
            </div>
        </div>
    `;
}

// ===== Hero Search =====
function handleHeroSearch() {
    const dest = document.getElementById('heroDest')?.value;
    const type = document.getElementById('heroType')?.value;
    
    if (dest) {
        document.getElementById('filterCity').value = dest;
    }
    
    navigate('/tours');
}

// ===== Handle Subscribe =====
async function handleSubscribe(e) {
    e.preventDefault();
    const form = e.target;
    const msgEl = document.getElementById('formMessage');
    const submitBtn = form.querySelector('button[type="submit"]');

    submitBtn.disabled = true;
    submitBtn.textContent = 'Отправка...';

    const formData = new FormData(form);
    const data = Object.fromEntries(formData.entries());

    // Convert types for API
    const payload = {
        origin_city: data.origin_city,
        destination: data.destination,
        date_from: data.date_from || '',
        messenger: data.messenger || 'telegram',
        user_identifier: data.user_identifier || '',
    };
    if (data.nights && parseInt(data.nights) >= 1 && parseInt(data.nights) <= 30) {
        payload.nights = parseInt(data.nights);
    } else {
        payload.nights = 7;
    }
    payload.max_price = data.max_price ? parseInt(data.max_price) || 0 : 0;

    try {
        const res = await fetch(`${API}/subscriptions`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });

        let result;
        const text = await res.text();
        try {
            result = JSON.parse(text);
        } catch {
            showFormMessage(msgEl, text || 'Ошибка сервера', 'error');
            submitBtn.disabled = false;
            submitBtn.textContent = 'Добавить подписку';
            return;
        }

        if (res.ok) {
            showFormMessage(msgEl, '✅ Подписка создана! Мы уведомим вас в Telegram.', 'success');
            form.reset();
        } else {
            showFormMessage(msgEl, result.error || 'Ошибка при создании подписки', 'error');
        }
    } catch (err) {
        showFormMessage(msgEl, 'Ошибка сети. Попробуйте позже.', 'error');
    }

    submitBtn.disabled = false;
    submitBtn.textContent = 'Добавить подписку';
}

function showFormMessage(el, text, type) {
    el.textContent = text;
    el.className = 'form-message ' + type;
}

// ===== Filter Tours =====
function filterTours() {
    const cityFilter = document.getElementById('filterCity').value.toLowerCase();
    const nightsFilter = document.getElementById('filterNights').value;
    const grid = document.getElementById('allToursGrid');

    if (!grid.dataset.allTours) return;

    const allTours = JSON.parse(grid.dataset.allTours);
    let filtered = allTours;

    if (cityFilter) {
        filtered = filtered.filter(t =>
            t.destination.toLowerCase().includes(cityFilter) ||
            t.origin_city.toLowerCase().includes(cityFilter) ||
            t.hotel_name.toLowerCase().includes(cityFilter)
        );
    }

    if (nightsFilter) {
        const nights = parseInt(nightsFilter);
        filtered = filtered.filter(t => t.nights >= nights && t.nights <= nights + 3);
    }

    if (filtered.length === 0) {
        grid.innerHTML = '<div class="tour-loading">Ничего не найдено. Попробуйте изменить фильтры.</div>';
    } else {
        grid.innerHTML = filtered.map(tour => renderTourCard(tour)).join('');
    }
}

// ===== Mobile Menu =====
document.addEventListener('DOMContentLoaded', () => {
    const btn = document.getElementById('mobileMenuBtn');
    const nav = document.querySelector('.nav');

    if (btn && nav) {
        btn.addEventListener('click', () => {
            nav.style.display = nav.style.display === 'flex' ? 'none' : 'flex';
            nav.style.position = 'absolute';
            nav.style.top = '60px';
            nav.style.left = '0';
            nav.style.right = '0';
            nav.style.background = 'white';
            nav.style.flexDirection = 'column';
            nav.style.padding = '16px';
            nav.style.boxShadow = '0 4px 6px rgba(0,0,0,0.1)';
        });
    }

    // Setup navigation clicks
    document.querySelectorAll('.nav-link').forEach(link => {
        link.addEventListener('click', (e) => {
            e.preventDefault();
            const href = link.getAttribute('href');
            navigate(href);
            // Close mobile menu
            if (nav && window.innerWidth <= 768) {
                nav.style.display = 'none';
            }
        });
    });

    // Handle browser back/forward
    window.addEventListener('popstate', render);

    // Initial render
    render();
});
