# Donchian Live — автоторговый бот на Lighter

Алго-бот по стратегии **Donchian / Turtle breakout**. Профиль **читается из `.env`**. Сейчас default: `30m` · `N=60` · `M=30` · `ATR(40)×1.5` · risk `1%` · cap `$1000` · live **`ls5_cond_brk2.0`**.

Контракт механизма: [`docs/DONCHIAN_LIVE_STARTPACK.md`](docs/DONCHIAN_LIVE_STARTPACK.md) + env-порт [`docs/DONCHIAN_LIVE_STARTPACK_30M_ENV.md`](docs/DONCHIAN_LIVE_STARTPACK_30M_ENV.md). Смена TF/N/M — новый run (новый `bot.db`), не «докрутка» старого журнала. Live vs shadow (три книги, verdict WAIT/STOP): **§17**.

| Слой | Стек |
|------|------|
| Биржа | [Lighter](https://apidocs.lighter.xyz) perps (BTC, ETH), официальный Go-signer |
| Бэкенд | Go, один процесс: торговый цикл + REST API + Telegram |
| БД | SQLite (WAL) |
| Дашборд | React + TypeScript + Tailwind, мобильный |
| Деплой | Docker Compose, опционально Caddy (HTTPS) |

Бэкенд написан на **Go**, не на ASP.NET: подпись транзакций Lighter реализована в Go (`elliottech/lighter-go`), официального C# SDK нет.

---

## 0. Что это за стратегия (чтобы не паниковать)

Простыми словами, с примерами и пайплайном час за часом: [`docs/KAK_RABOTAET_STRATEGIYA.md`](docs/KAK_RABOTAET_STRATEGIYA.md).

- Пробой канала Дончиана на **закрытом баре TF из env** (default 30m, N=60 ≈ те же 30 часов, что 1h N=30). Выход по ATR-стопу `1.5×ATR` (SMA of True Range, **не** Wilder) или по короткому каналу M (default 30).
- Один слот позиции **на символ** (без пирамиды). BTC и ETH торгуются **независимо**.
- Сайзинг: **1% equity** на сделку, cap **$1000** (оба из env).
- После **5 подряд gross-лоссов** — пауза **24 часа** (48 баров на 30m), ранний выход из паузы если пробой ≥ **2×ATR**.
- Ожидаемый профиль: winrate **~25–30%**, max DD часто **−25…−35%**, прибыль от редких runners. Серии мелких −R — норма. Если live показывает WR 55% и крошечный DD — скорее баг порта.

Research считался на Binance. Live идёт на **Lighter**. Свечи и fills не совпадут с backtest — это ожидаемо.

---

## 1. Архитектура

```
Телефон / браузер  →  web (nginx, :8080)  →  /api  →  bot (:8080 внутри сети)
                                           SQLite /data/bot.db
Telegram Bot API  ←  bot
Lighter REST + WS ←  bot  (свечи из DONCHIAN_TIMEFRAME, IOC market, reduce-only stop-loss)
```

Решения принимаются **только на закрытой свече выбранного TF**. Сразу после close ставится market IOC (это `open` следующего бара), затем reduce-only **STOP_LOSS** на бирже. Channel-exit — тоже market reduce-only на следующем open.

С бара 0 считаются **три книги** на одних и тех же closed futures: **Live**, **Shadow-ls5**, **Shadow-baseline**. Канон сверки: Live ↔ Shadow того же live-профиля, то же окно. Не сравнивать месяц live с yearly fee-0 дашбордом.

**Касса shadow = зеркало счёта.** `START_EQUITY` не задаётся. Первое ненулевое equity на бирже — seed всех трёх книг. Дальше пополнения и выводы детектятся как остаток `Δwallet − PnL − комиссии входа` (у Lighter нет нормальной истории депозитов в trading API) и тем же числом пишутся в shadow. PnL и mark не считаются кэш-флоу.

---

## 2. Требования

### Локально (разработка)

- Go 1.23+
- Node.js 22+
- Git

### VPS (Ubuntu 22.04 / 24.04)

- 1–2 vCPU, 2 GB RAM достаточно (бот не HFT)
- 20+ GB диск
- Docker Engine + Docker Compose plugin
- Аккаунт [Lighter](https://app.lighter.xyz) (сначала **testnet**, потом mainnet)
- Telegram-бот от [@BotFather](https://t.me/BotFather)
- (для HTTPS) домен, A-запись на IP VPS

Colocation Lighter рекомендует AWS Tokyo `ap-northeast-1a`. Для 1h это не критично.

---

## 3. Lighter: ключи и account index

1. Создайте кошелёк / зайдите в Lighter UI.
2. **API keys:** индексы **0 и 1 зарезервированы под веб/мобильное приложение**. Для бота создайте ключ с индексом **2–254**.
3. Сохраните **private key** API key (hex). Это секрет, как seed.
4. Узнайте **account index** (целое число аккаунта, не путать с api_key_index). В UI или через API:

```bash
curl "https://mainnet.zklighter.elliot.ai/api/v1/accountsByL1Address?l1_address=0xВАШ_АДРЕС"
```

Для testnet хост: `https://testnet.zklighter.elliot.ai`.

5. Пополните **testnet** (или mainnet) collateral. Без маржи ордера не пройдут.
6. Проверьте, что символы **BTC** и **ETH** — это perp-рынки (бот резолвит `market_id` на старте через `orderBookDetails`).

Endpoints:

| Сеть | REST | WS | Chain ID |
|------|------|----|----------|
| testnet | `https://testnet.zklighter.elliot.ai` | `wss://testnet.zklighter.elliot.ai/stream` | 300 |
| mainnet | `https://mainnet.zklighter.elliot.ai` | `wss://mainnet.zklighter.elliot.ai/stream` | 304 |

---

## 4. Telegram

1. `@BotFather` → `/newbot` → скопируйте token.
2. Напишите боту любое сообщение.
3. Узнайте chat id:

```bash
curl "https://api.telegram.org/bot<TOKEN>/getUpdates"
```

Поле `message.chat.id` (часто отрицательное для групп).  
4. Пропишите `TELEGRAM_BOT_TOKEN` и `TELEGRAM_CHAT_IDS` (можно несколько через запятую).  
5. Чужие чаты бот игнорирует.

Команды: `/health`, `/status`, `/trades` (последние 10 Live), `/report` (суточный снимок + график), `/kill`, `/resume`, `/help`.  
Раз в сутки после закрытия часа **23:00 UTC** бот сам шлёт тот же отчёт с графиком Live vs тень. Heartbeat — если бот молчал `TELEGRAM_HEARTBEAT_MINUTES` минут. Вердикт в отчёте **не** из знака PnL.

---

## 5. Конфигурация (`.env`)

Скопируйте шаблон:

```bash
cp .env.example .env
nano .env
```

| Переменная | Смысл | Пример |
|------------|--------|--------|
| `LIGHTER_NETWORK` | `testnet` или `mainnet` | `testnet` |
| `LIGHTER_API_PRIVATE_KEY` | hex private key API | `0xabc...` |
| `LIGHTER_ACCOUNT_INDEX` | индекс аккаунта | `12345` |
| `LIGHTER_API_KEY_INDEX` | 2–254 | `2` |
| `SYMBOLS` | инструменты | `BTC,ETH` |
| `DONCHIAN_TIMEFRAME` | свеча Lighter: `1h` `30m` `15m` `5m` | `30m` |
| `DONCHIAN_CHANNEL_N` | баров входа | `60` |
| `DONCHIAN_EXIT_M` | баров выхода | `30` |
| `DONCHIAN_ATR_PERIOD` | SMA-TR | `40` |
| `DONCHIAN_ATR_STOP_MULT` | стоп = mult × ATR | `1.5` |
| `DONCHIAN_RISK_PCT` | риск на стоп | `1.0` |
| `DONCHIAN_MAX_RISK_USD` | cap риска сделки | `1000` |
| `DONCHIAN_LIVE_PROFILE` | `ls5_cond_brk2.0` или `baseline` | `ls5_cond_brk2.0` |
| `SHADOW_LS5` | theoretical ls5 с бара 0 | `true` |
| `SHADOW_BASELINE` | theoretical core без паузы | `true` |
| `CASH_FLOW_MIN_USD` | порог автодетекта пополнения/вывода | `5` |
| `MAX_NOTIONAL_USD` | операционный cap номинала | `50000` |
| `DAILY_LOSS_LIMIT_USD` | дневной стоп (не research DD-pause) | `500` |
| `MAX_LEVERAGE` | notional/equity | `10` |
| `MARKET_SLIPPAGE` | запас цены для IOC | `0.02` |
| `KILL_SWITCH` | стартовать в режиме flatten/no-entry | `false` |
| `SQLITE_PATH` | в Docker задаётся compose | `/data/bot.db` |
| `HTTP_ADDR` | listen API | `:8080` |
| `DASHBOARD_PASSWORD` | пароль дашборда **обязателен** | длинный пароль |
| `COOKIE_SECURE` | `true` за HTTPS | `false` на HTTP |
| `TELEGRAM_BOT_TOKEN` | от BotFather | |
| `TELEGRAM_CHAT_IDS` | whitelist | `123456789` |
| `TELEGRAM_HEARTBEAT_MINUTES` | 0 = выкл | `60` |
| `DRY_RUN` | не слать ордера (отладка) | `false` |
| `WEB_PORT` | порт дашборда на хосте | `80` (`http://IP/`) |
| `DOMAIN` | для профиля `tls` | `bot.example.com` |

Параметры стратегии (`DONCHIAN_TIMEFRAME`, N, M, ATR period, stop mult, риск, cap, ls5) читаются из `.env`. Смена TF на уже существующей SQLite запрещена — архивируйте `data/bot.db` и стартуйте новый run.

---

## 6. Развёртывание на Ubuntu (Docker)

Ниже — полный путь «чистый VPS → бот в работе». Команды от пользователя с `sudo`.

### 6.1 Система

```bash
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y ca-certificates curl git ufw fail2ban
```

### 6.2 Docker Engine + Compose

```bash
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker "$USER"
# перелогиньтесь, чтобы группа docker применилась
docker version
docker compose version
```

### 6.3 Firewall

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow OpenSSH
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status
```

Не публикуйте дашборд без пароля. HTTP на 80 нормален для testnet; для телефона лучше HTTPS (ниже).

Откройте на телефоне `http://IP_VPS/` (порт 80), введите `DASHBOARD_PASSWORD`.

### 6.4 Код

```bash
sudo mkdir -p /opt/donchian
sudo chown "$USER:$USER" /opt/donchian
cd /opt/donchian
git clone <URL_ЭТОГО_РЕПО> .
cp .env.example .env
nano .env
mkdir -p data backups
```

Заполните `.env`. Первый запуск: **`LIGHTER_NETWORK=testnet`**.

### 6.5 Сборка и запуск

```bash
cd /opt/donchian
docker compose up -d --build
docker compose ps
docker compose logs -f bot
```

Что должно появиться в логах:

- `market symbol=BTC id=…`
- `bootstrapped symbol=BTC bars=…` (сотни/тысячи 1h свечей)
- `http listen`
- Telegram: `Donchian онлайн` (если токен задан)

Проверка API с VPS:

```bash
curl -s http://127.0.0.1/api/health
```

Откройте на телефоне `http://IP_VPS/` (или домен), введите `DASHBOARD_PASSWORD`. Если порт занят — в `.env` поставьте `WEB_PORT=8080` и ходите на `:8080`.

### 6.6 HTTPS (рекомендуется для телефона)

1. DNS: A-запись `bot.example.com` → IP VPS.
2. В `.env`: `DOMAIN=bot.example.com`, `COOKIE_SECURE=true`, `WEB_PORT=8080` (чтобы 80 остался Caddy).
3. Запуск с Caddy:

```bash
docker compose --profile tls up -d --build
```

Caddy сам получит Let's Encrypt. 80 и 443 тогда у Caddy, дашборд: `https://bot.example.com`.

---

## 7. Первый прогон: testnet, затем mainnet

1. `LIGHTER_NETWORK=testnet`, ключи **testnet**, депозит testnet.
2. Дождитесь закрытия часа — либо смотрите shadow/live в дашборде и Telegram.
3. Убедитесь: после входа на бирже висит **один** reduce-only stop.
4. Проверьте `/kill` в Telegram на testnet (flatten).
5. `/resume`, когда готовы снова входить.

Переключение на mainnet:

```bash
cd /opt/donchian
# 1) бэкап БД (testnet журнал лучше не смешивать с mainnet)
docker compose stop bot
cp -a data "backups/data-testnet-$(date -u +%Y%m%dT%H%M%SZ)"
rm -f data/bot.db data/bot.db-wal data/bot.db-shm
# 2) ключи и сеть mainnet в .env
nano .env   # LIGHTER_NETWORK=mainnet, новые ключи/account_index, COOKIE_SECURE если HTTPS
docker compose up -d
docker compose logs -f bot
```

Не используйте testnet SQLite на mainnet — состояние streak/позиции будет врать.

---

## 8. Обновление проекта

```bash
cd /opt/donchian
docker compose exec bot sqlite3 /data/bot.db ".backup /data/bot-preupdate.db"
cp data/bot-preupdate.db backups/bot-preupdate-$(date -u +%Y%m%dT%H%M%SZ).db

git fetch
git pull
docker compose up -d --build
docker compose ps
docker compose logs --tail=100 bot
```

Если миграции SQLite несовместимы (редко): остановите бота, восстановите бэкап, разберитесь с changelog. Пока схема создаётся `CREATE TABLE IF NOT EXISTS` — обновления обычно безопасны.

Откат:

```bash
git log -5 --oneline
git checkout <предыдущий_коммит>
docker compose up -d --build
```

---

## 9. Бэкапы SQLite

Файл: `data/bot.db` (+ `-wal`/`-shm` если бот запущен).

**Онлайн (предпочтительно):**

```bash
mkdir -p /opt/donchian/backups
docker compose exec -T bot sqlite3 /data/bot.db ".backup /data/bot-snap.db"
cp data/bot-snap.db /opt/donchian/backups/bot-$(date -u +%Y%m%dT%H%M%SZ).db
```

**Офлайн:**

```bash
docker compose stop bot
cp -a data /opt/donchian/backups/data-$(date -u +%Y%m%dT%H%M%SZ)
docker compose start bot
```

Cron раз в день (root crontab):

```cron
15 3 * * * cd /opt/donchian && docker compose exec -T bot sqlite3 /data/bot.db ".backup /data/bot-cron.db" && cp data/bot-cron.db backups/bot-$(date -u +\%Y\%m\%d).db
```

Храните копии **вне VPS** (S3, другой сервер). В БД нет API-ключей, но есть вся история сделок.

---

## 10. Логи и наблюдение

```bash
docker compose logs -f --tail=200 bot
docker compose logs -f web
```

Ротация: Docker log driver по умолчанию может раздуть диск. Рекомендуется `/etc/docker/daemon.json`:

```json
{
  "log-driver": "json-file",
  "log-opts": { "max-size": "50m", "max-file": "5" }
}
```

Затем `sudo systemctl restart docker` и `docker compose up -d`.

Дашборд → вкладка **Health**: WS, kill-switch, last error, лента событий.  
Telegram `/status` — то же с телефона без браузера.

---

## 11. Обслуживание

| Задача | Как |
|--------|-----|
| Рестарт | `docker compose restart bot` — состояние позиции/streak восстанавливается из SQLite + сверка с биржей |
| Пропущен бар (бот лежал) | при старте backfill выбранного TF и REST-poll закрытых баров; вход состоится на **следующем** валидном close, не «догоняется» история ордерами |
| Kill-switch | дашборд / Telegram `/kill` — flatten всех позиций, новые входы запрещены |
| Снять kill | `/resume` или кнопка на дашборде |
| Место на диске | `df -h`, `docker system df`, бэкапы в `backups/` |
| Смена пароля дашборда | поменять `DASHBOARD_PASSWORD` в `.env`, `docker compose up -d bot` |
| Смена API-ключа | новый ключ в `.env`, индекс 2–254, рестарт; nonce начнётся заново |

После рестарта бот:

1. Догружает ≥ ~120 дней свечей выбранного TF.
2. Читает open trade и ls5-состояние из БД.
3. Сверяет позицию с Lighter (reconcile каждые ~15 сек). Если стоп пропал — flatten и алерт.

---

## 12. Локальная разработка без Docker

```bash
cp .env.example .env
# SQLITE_PATH=./data/bot.db
# HTTP_ADDR=:8080
# можно DRY_RUN=true для UI без ордеров (ключ всё равно нужен, если DRY_RUN=false)

mkdir -p data
cd backend
go test ./...
go run ./cmd/bot

# другой терминал
cd frontend
npm install
npm run dev
```

Дашборд: http://127.0.0.1:5173 (Vite проксирует `/api` на `:8080`).

---

## 13. Безопасность

- `.env` не коммитить. Права: `chmod 600 .env`.
- SSH только по ключу, `PermitRootLogin no`.
- UFW: 22, 80, 443. Не светить API бота наружу отдельно от nginx/Caddy.
- `DASHBOARD_PASSWORD` длинный, уникальный. Это не 2FA — при необходимости поставьте VPN (WireGuard) и не публикуйте 80/443.
- API private key Lighter = возможность торговать вашим аккаунтом. Утечка = чужие ордера.
- Не используйте UI-ключи 0/1.

---

## 14. Troubleshooting

| Симптом | Что проверить |
|---------|----------------|
| `LIGHTER_API_PRIVATE_KEY is required` | ключ в `.env`, compose подхватил `env_file` |
| `API_KEY_INDEX must be 2-254` | не 0/1 |
| `market BTC not found` | сеть testnet/mainnet, символ как на Lighter (`BTC` / `ETH`) |
| `sendTx code=… nonce` | другой процесс с тем же api_key_index; один бот на ключ |
| Нет стопа после входа | алерт `watchdog` / `stop`; бот flatten; смотрите active orders в UI |
| Telegram `409 Conflict` | второй polling (ещё один контейнер/процесс с тем же токеном) |
| Дашборд 401 после HTTPS | `COOKIE_SECURE=true`, заходите именно по `https://` |
| `warming up` | мало свечей; смотрите логи backfill, firewall исходящий 443 |
| Бот не видит close бара | WS `candle/{id}/{DONCHIAN_TIMEFRAME}` + REST poll каждые 15с; сверьте время UTC |
| Рассинхрон позиции | Health / Telegram; `/kill` если неясно |
| WR «слишком хороший» | lookahead / не тот ATR; сверьте тесты `go test ./internal/strategy` |

Полезные ручные запросы:

```bash
curl -s "https://mainnet.zklighter.elliot.ai/api/v1/orderBookDetails" | head
curl -s http://127.0.0.1:8080/api/health
docker compose exec bot wget -qO- http://127.0.0.1:8080/api/health
```

---

## 15. Структура репозитория

```
backend/          Go-бот (cmd/bot, internal/strategy|engine|exchange|store|api|telegram|telemetry)
frontend/         React-дашборд (герой: Live vs Shadow-ls5)
deploy/Caddyfile  TLS-прокси
data/             SQLite + equity_snapshots.csv + reports/ (не в git)
docs/DONCHIAN_LIVE_STARTPACK.md
```

Движок сигналов: `backend/internal/strategy` — порт research 1:1, покрыт unit-тестами §12 startpack (`go test ./internal/strategy`).

---

## 16. Операционные гарды vs research

Включено: max notional, daily loss, max leverage, kill-switch, один вход на `signal_time`, watchdog стопа.

**Не включено** (отвергнуто research): equity DD-pause, ADX-фильтры, streak-sizing, invert after loss, pyramiding.

Kill-switch **не** смотрит на знак дня/недели/месяца и не сравнивает с +9% дашборда. `verdict` = матрица §17.13 (A/B/C + расхождение DD). STOP flatten'ит live, shadow продолжает. Перед первым seed лучше flatten чужие позиции: иначе в кассу shadow попадёт текущий equity вместе с чужим uPnL.

---

## 17. Дисклеймер

Это исследовательский порт, не инвестиционная рекомендация. Live PnL зависит от funding, проскальзывания, простоев, отличий Lighter от Binance-свечей. Рискуйте только тем, что готовы потерять.
