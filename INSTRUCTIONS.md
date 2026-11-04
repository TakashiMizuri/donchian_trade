# Операции на проде

Каталог на VPS: `/opt/donchian`.  
`bot_a` = **5m R-026** (brk0 + vol off + tstop 20h + late trail 18h/2R).  
`bot_b` = **1m R-025 satellite** (brk0 + vol off + tstop 14h + late trail 12h/6R).  
Файлы `.env.a` и `.env.b` **не удалять** (там ключи Lighter / Telegram / пароль дашборда).

Контракты:

- 5m: [`docs/DONCHIAN_LIVE_STARTPACK_5M_BRK0_VOLOFF_TSTOP20_LATE_TRAIL2.md`](docs/DONCHIAN_LIVE_STARTPACK_5M_BRK0_VOLOFF_TSTOP20_LATE_TRAIL2.md)
- 1m: [`docs/DONCHIAN_LIVE_STARTPACK_1M_BRK0_VOLOFF_TSTOP14_LATE_TRAIL6.md`](docs/DONCHIAN_LIVE_STARTPACK_1M_BRK0_VOLOFF_TSTOP14_LATE_TRAIL6.md)

Это **новый run_id**. Старый `bot.db` с brk+vol без trail (или с другим trail R) процесс не примет — fingerprint другой. Нужен полный сброс журнала (§1.0 / §1.1).

Перед полным сбросом на **обоих** аккаунтах Lighter не должно быть открытых позиций и висящих стопов. После пустой базы бот считает себя flat и чужую позицию на бирже закроет рынком.

Менял `.env` → всегда `docker compose up -d --force-recreate …`, не просто `restart`.  
`KILL_SWITCH=false` в файле ≠ в процессе, пока не было recreate.

---

## 0. Переход на BRK0 + late trail (с текущего 5m/1m brk+vol)

С Windows код уже в репо. На VPS — `git pull`, правка `.env.a`/`.env.b`, wipe sqlite, recreate.

### 0.1 Что меняется в стратегии

| | Было (live) | Стало (live **и** тень `shadow_ls5`) |
|--|--|--|
| `bot_a` 5m | brk 0.5 + vol 0.64, tstop 20h, без trail | **фильтры off**, tstop 20h, **trail 216 / 2R / MFE 6** |
| `bot_b` 1m | brk 0.5 + vol 0.64, tstop 14h, без trail | **фильтры off**, tstop 14h, **trail 720 / 6R / MFE 15** |

Тень — **та же стратегия**, бумажные филлы (без слипа). Расхождение Live↔Тень = исполнение, не другой набор knobs. Baseline по-прежнему голый канал без фильтров/tstop/trail.

`MAX_VOL_RANK=0` = фильтр **выключен**, не «пропускать rank 0».  
Trail: peak **прошлого** бара; orig ATR-стоп важнее на том же баре; trail-выход = `outcome=time`, цена = `trail_stop` (не next open).  
Когда trail вооружён, бот **переставляет** биржевой reduce-only SL на lagged trail.

Не копировать 2R на 1m и 6R на 5m. Не возвращать brk/vol «потому что DD». Не ретюнить 216/2/6 и 720/6/15 по живой неделе.

### 0.2 Правка `.env.a` / `.env.b` на VPS

**Не** `cp .env.example .env.a` — сотрёте ключи. Меняйте только `DONCHIAN_*` / `SHADOW_*`. `LIGHTER_*`, `TELEGRAM_*`, `DASHBOARD_PASSWORD`, `SYMBOLS` оставить.

На VPS:

```bash
cd /opt/donchian
cp -a .env.a .env.a.bak-$(date -u +%Y%m%dT%H%M%SZ)
cp -a .env.b .env.b.bak-$(date -u +%Y%m%dT%H%M%SZ)
```

`.env.a` (5m) — добейтесь такого блока (остальное не трогать):

```dotenv
SHADOW_BASELINE=true
DONCHIAN_SHADOW_BRK_VOL=true

DONCHIAN_TAG=5m_N360_M180_ATR240_R1.0_brk0_voloff_tstop20h_late18h_2R
DONCHIAN_TIMEFRAME=5m
DONCHIAN_BAR_SECONDS=300
DONCHIAN_CHANNEL_N=360
DONCHIAN_EXIT_M=180
DONCHIAN_ATR_PERIOD=240
DONCHIAN_ATR_METHOD=sma_tr
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=625
DONCHIAN_MAX_RISK_ENABLED=true
DONCHIAN_FEE_RATE_PER_SIDE=0

DONCHIAN_LIVE_PROFILE=core
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_VOL_RANK_BARS=240
DONCHIAN_VOL_RANK_LOOKBACK=12000

DONCHIAN_MAX_BARS_IN_TRADE=240
DONCHIAN_SHADOW_MAX_BARS_IN_TRADE=240

DONCHIAN_LATE_TRAIL_ON_BARS=216
DONCHIAN_LATE_TRAIL_R=2
DONCHIAN_LATE_TRAIL_MIN_MFE=6
```

`.env.b` (1m satellite):

```dotenv
SHADOW_BASELINE=true
DONCHIAN_SHADOW_BRK_VOL=true

DONCHIAN_TAG=1m_N1800_M900_ATR1200_R1.0_brk0_voloff_tstop14h_late12h_6R_satellite
DONCHIAN_TIMEFRAME=1m
DONCHIAN_BAR_SECONDS=60
DONCHIAN_CHANNEL_N=1800
DONCHIAN_EXIT_M=900
DONCHIAN_ATR_PERIOD=1200
DONCHIAN_ATR_METHOD=sma_tr
DONCHIAN_ATR_STOP_MULT=1.5
DONCHIAN_RISK_PCT=1.0
DONCHIAN_MAX_RISK_USD=1000
DONCHIAN_MAX_RISK_ENABLED=true
DONCHIAN_FEE_RATE_PER_SIDE=0

DONCHIAN_LIVE_PROFILE=core
DONCHIAN_MIN_BREAKOUT_ATR=0
DONCHIAN_MAX_VOL_RANK=0
DONCHIAN_VOL_RANK_BARS=1200
DONCHIAN_VOL_RANK_LOOKBACK=60000

DONCHIAN_MAX_BARS_IN_TRADE=840
DONCHIAN_SHADOW_MAX_BARS_IN_TRADE=840

DONCHIAN_LATE_TRAIL_ON_BARS=720
DONCHIAN_LATE_TRAIL_R=6
DONCHIAN_LATE_TRAIL_MIN_MFE=15
```

Проверка, что секреты на месте и knobs правильные:

```bash
grep -E '^(LIGHTER_API_PRIVATE_KEY|LIGHTER_ACCOUNT_INDEX|DASHBOARD_PASSWORD|TELEGRAM_BOT_TOKEN)=' .env.a .env.b
# ключи не пустые

grep -E '^(DONCHIAN_TAG|DONCHIAN_LIVE_PROFILE|DONCHIAN_MIN_BREAKOUT_ATR|DONCHIAN_MAX_VOL_RANK|DONCHIAN_MAX_BARS_IN_TRADE|DONCHIAN_LATE_TRAIL_|DONCHIAN_SHADOW_LATE_TRAIL_|KILL_SWITCH)=' .env.a .env.b
```

Ожидание:

- `.env.a`: `LIVE_PROFILE=core`, brk=0, vol=0, tstop=240, trail **216/2/6**, `KILL_SWITCH=false`
- `.env.b`: `LIVE_PROFILE=core`, brk=0, vol=0, tstop=840, trail **720/6/15**, tag содержит `satellite`, `KILL_SWITCH=false`

Если `KILL_SWITCH=true` — выключить в **файле** до recreate.

### 0.3 Порядок на бирже, потом git + wipe

1. В Lighter UI оба аккаунта: позиции FLAT, нет reduce-only стопов.  
2. Потом §1.1 (с бэкапом) или §1.0.  
3. `bot_b` (1m) греет **долго** (канал N=1800 + запас). `start_period` у `bot_b` = 3600s — контейнер не убивать.

---


## 1.0 Остановка, полный сброс, pull, перезапуск — без бэкапа

Для перехода на BRK0 + late trail сначала §0 (правка `.env.a`/`.env.b`).  
Журналы и sqlite **пропадут**. Откатить нечем.

```bash
cd /opt/donchian

git fetch origin
git checkout main
git pull --ff-only
git log -1 --oneline
# должно совпасть с origin/main

grep '^KILL_SWITCH=' .env.a .env.b
# оба: KILL_SWITCH=false
# если нет:
#   sed -i 's/^KILL_SWITCH=.*/KILL_SWITCH=false/' .env.a .env.b
#   grep '^KILL_SWITCH=' .env.a .env.b

/usr/local/sbin/sync-clock-http
# offset_s около 0 (±2s)

docker compose stop bot_a bot_b web

rm -f data/a/bot.db data/a/bot.db-wal data/a/bot.db-shm data/a/equity_snapshots.csv data/a/bot-preupdate.db data/a/bot-snap.db
rm -f data/b/bot.db data/b/bot.db-wal data/b/bot.db-shm data/b/equity_snapshots.csv data/b/bot-preupdate.db data/b/bot-snap.db
rm -rf data/a/logs data/a/reports data/b/logs data/b/reports

docker compose up -d --build --force-recreate bot_a bot_b web
```

Проверки — §1.2.

---

## 1.1 То же, но с бэкапом

Нужно свободно **≥ 3G** на `/` (`df -h /`). Если меньше — сначала место (часто Docker Build Cache):

```bash
docker builder prune -a -f
df -h /
```

Потом сброс с архивом `data/`:

```bash
cd /opt/donchian

git fetch origin
git checkout main
git pull --ff-only
git log -1 --oneline

grep '^KILL_SWITCH=' .env.a .env.b
/usr/local/sbin/sync-clock-http

mkdir -p backups
tar czf "backups/data-$(date -u +%Y%m%dT%H%M%SZ).tgz" data
ls -lh backups/data-*.tgz | tail

docker compose stop bot_a bot_b web

rm -f data/a/bot.db data/a/bot.db-wal data/a/bot.db-shm data/a/equity_snapshots.csv data/a/bot-preupdate.db data/a/bot-snap.db
rm -f data/b/bot.db data/b/bot.db-wal data/b/bot.db-shm data/b/equity_snapshots.csv data/b/bot-preupdate.db data/b/bot-snap.db
rm -rf data/a/logs data/a/reports data/b/logs data/b/reports

docker compose up -d --build --force-recreate bot_a bot_b web
```

Проверки — §1.2.

---

## 1.2 Проверки после старта (для 1.0 и 1.1)

```bash
docker compose ps
docker compose exec bot_a printenv KILL_SWITCH DONCHIAN_LIVE_PROFILE DONCHIAN_MIN_BREAKOUT_ATR DONCHIAN_MAX_VOL_RANK DONCHIAN_LATE_TRAIL_ON_BARS DONCHIAN_LATE_TRAIL_R DONCHIAN_LATE_TRAIL_MIN_MFE DONCHIAN_TAG
docker compose exec bot_b printenv KILL_SWITCH DONCHIAN_LIVE_PROFILE DONCHIAN_MIN_BREAKOUT_ATR DONCHIAN_MAX_VOL_RANK DONCHIAN_LATE_TRAIL_ON_BARS DONCHIAN_LATE_TRAIL_R DONCHIAN_LATE_TRAIL_MIN_MFE DONCHIAN_TAG
# a: KILL_SWITCH=false  PROFILE=core  brk=0  vol=0  trail=216/2/6  tag=..._brk0_voloff_..._late18h_2R
# b: KILL_SWITCH=false  PROFILE=core  brk=0  vol=0  trail=720/6/15 tag=..._satellite

docker compose exec bot_a wget -qO- http://127.0.0.1:8080/api/health

docker compose logs --tail=80 bot_a
docker compose logs -f bot_b
```

В логах старта (`strategy profile`): `live_profile=core`, `min_breakout_atr=0`, `max_vol_rank=0`, `late_trail_on` / `late_trail_r` как выше; `shadow_trail_r` **тот же**, что live.  
Не должно быть `DONCHIAN_LIVE_PROFILE "core"` как ошибки (старый бинарь). Если есть — образ не пересобрался (`--build`).

`bootstrapped` / `boot catchup ok`, `http listen`.  
У `bot_b` (1m) прогрев **долгий** — контейнер не убивать.

Дашборд: аварийный стоп выкл, Live/тень ≈ депозит, позиции FLAT, лаг свечей сотни мс (не минуты).  
Live без brk/vol будет входить **чаще**, чем старый профиль. Тень должна входить в те же бары, что live (расхождение = skip/slip/лаг, не фильтры).  
Первый бар может не дать сделку — нормально. Trail на 5m живёт с 18-го часа в сделке (216×5m), на 1m — с 12-го (720×1m); до этого на бирже обычный ATR-стоп.

Если скрипта часов нет: `/usr/local/sbin/sync-clock-http` — на этом хосте NTP (UDP/123) обычно мёртв, сверка по HTTP `Date` биржи раз в 10 минут.

---

## 2. Полная выгрузка архива для анализа (5m + 1m)

Ботов **не останавливать**. Ключи в архив не попадают (env режется).

На VPS вставь целиком:

```bash
cd /opt/donchian
TS=$(date -u +%Y%m%dT%H%M%SZ)
OUT=/tmp/prod-$TS
mkdir -p "$OUT"/data_a "$OUT"/data_b

date -u > "$OUT/date.txt"
uptime >> "$OUT/date.txt"
hostname >> "$OUT/date.txt"
uname -a >> "$OUT/date.txt"

git rev-parse HEAD > "$OUT/git.txt" 2>&1
git log -8 --oneline >> "$OUT/git.txt" 2>&1
git status --short >> "$OUT/git.txt" 2>&1
git describe --always --dirty >> "$OUT/git.txt" 2>&1

docker compose ps -a > "$OUT/compose-ps.txt" 2>&1
docker compose images > "$OUT/compose-images.txt" 2>&1
for c in donchian-bot_a-1 donchian-bot_b-1 donchian-web-1; do
  docker inspect "$c" --format \
    '{{.Name}} image={{.Config.Image}} id={{.Id}} started={{.State.StartedAt}} finished={{.State.FinishedAt}} restarts={{.RestartCount}} oom={{.State.OOMKilled}} exit={{.State.ExitCode}} status={{.State.Status}}' \
    >> "$OUT/inspect.txt" 2>&1
done
docker compose exec -T bot_a printenv KILL_SWITCH INSTANCE_ID INSTANCE_NAME DONCHIAN_TIMEFRAME DONCHIAN_TAG DONCHIAN_LIVE_PROFILE DONCHIAN_MIN_BREAKOUT_ATR DONCHIAN_MAX_VOL_RANK DONCHIAN_MAX_BARS_IN_TRADE DONCHIAN_LATE_TRAIL_ON_BARS DONCHIAN_LATE_TRAIL_R DONCHIAN_LATE_TRAIL_MIN_MFE 2>/dev/null > "$OUT/env_runtime_a.txt"
docker compose exec -T bot_b printenv KILL_SWITCH INSTANCE_ID INSTANCE_NAME DONCHIAN_TIMEFRAME DONCHIAN_TAG DONCHIAN_LIVE_PROFILE DONCHIAN_MIN_BREAKOUT_ATR DONCHIAN_MAX_VOL_RANK DONCHIAN_MAX_BARS_IN_TRADE DONCHIAN_LATE_TRAIL_ON_BARS DONCHIAN_LATE_TRAIL_R DONCHIAN_LATE_TRAIL_MIN_MFE 2>/dev/null > "$OUT/env_runtime_b.txt"

/usr/local/sbin/sync-clock-http > "$OUT/clock.txt" 2>&1
date -u >> "$OUT/clock.txt"
timedatectl status >> "$OUT/clock.txt" 2>&1
systemctl list-timers sync-clock-http.timer --no-pager >> "$OUT/clock.txt" 2>&1
journalctl -u sync-clock-http --since "7 days ago" --no-pager >> "$OUT/clock.txt" 2>&1
journalctl -k --since "7 days ago" --no-pager 2>/dev/null | grep -iE 'oom|killed process|Out of memory' > "$OUT/host_oom.txt"
free -m > "$OUT/host.txt"
df -h / /opt >> "$OUT/host.txt"
docker stats --no-stream >> "$OUT/host.txt" 2>&1

for x in a b; do
  sed -E 's/^([A-Za-z0-9_]*(KEY|SECRET|TOKEN|PASS|PRIVATE|ADDRESS|CHAT)[A-Za-z0-9_]*=).*/\1<redacted>/' ".env.$x" \
    > "$OUT/env_$x.redacted.txt"
done

docker compose logs --no-color --timestamps --since 168h bot_a > "$OUT/bot_a.logs.txt" 2>&1
docker compose logs --no-color --timestamps --since 168h bot_b > "$OUT/bot_b.logs.txt" 2>&1

for x in a b; do
  docker compose exec -T bot_$x sqlite3 /data/bot.db ".backup /data/bot-snap.db" \
    && mv "data/$x/bot-snap.db" "$OUT/data_$x/bot.db"
  cp -a "data/$x/logs" "data/$x/reports" "$OUT/data_$x/" 2>/dev/null
  cp -a "data/$x/equity_snapshots.csv" "$OUT/data_$x/" 2>/dev/null
done

for x in a b; do
  {
    echo "=== instance bot_$x ==="
    echo "=== global ==="
    docker compose exec -T bot_$x sqlite3 /data/bot.db \
      "SELECT value FROM bot_state WHERE key='global';" 2>/dev/null || true
    echo
    echo "=== live trades (all) ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT id,symbol,direction,signal_time,entry_time,entry_price,stop,exit_time,exit_price,outcome,net,funding,entry_px_shadow,entry_px_live,exit_px_shadow,exit_px_live FROM trades WHERE profile='live' ORDER BY id;" 2>/dev/null || true
    echo
    echo "=== shadow_ls5 (last 80) ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT id,symbol,direction,signal_time,entry_time,exit_time,outcome,net FROM trades WHERE profile='shadow_ls5' ORDER BY id DESC LIMIT 80;" 2>/dev/null || true
    echo
    echo "=== shadow_baseline (last 80) ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT id,symbol,direction,signal_time,entry_time,exit_time,outcome,net FROM trades WHERE profile='shadow_baseline' ORDER BY id DESC LIMIT 80;" 2>/dev/null || true
    echo
    echo "=== events (last 300) ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT id,ts,level,kind,substr(message,1,240) FROM events ORDER BY id DESC LIMIT 300;" 2>/dev/null || true
    echo
    echo "=== mismatches ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT * FROM mismatches ORDER BY id DESC LIMIT 100;" 2>/dev/null || true
    echo
    echo "=== skip reasons ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT skip_reason, count(*) FROM bar_logs GROUP BY skip_reason ORDER BY 2 DESC;" 2>/dev/null || true
    echo
    echo "=== daily reports ==="
    docker compose exec -T bot_$x sqlite3 -header -csv /data/bot.db \
      "SELECT * FROM daily_reports ORDER BY date_utc;" 2>/dev/null || true
  } > "$OUT/data_$x/db_peek.txt"
done

for x in a b; do
  {
    echo "=== focus jsonl bot_$x ==="
    grep -hE '"kind":"(entry_intent|entry_sent|entry_fill|exit_fill|exit_intent|bar_closed|boundary_poll|shadow_sync|reconcile|boot|ws)"' \
      "$OUT/data_$x/logs"/bot-*.jsonl 2>/dev/null | tail -n 4000
  } > "$OUT/data_$x/jsonl_focus.txt"
done

mkdir -p backups
tar czf "backups/prod-$TS.tgz" -C /tmp "prod-$TS"
rm -rf "$OUT"
ls -lh "backups/prod-$TS.tgz"
echo "DONE backups/prod-$TS.tgz"
```

С Windows:

```powershell
cd C:\All\Develop\donchian_trade
New-Item -ItemType Directory -Force prod_mess | Out-Null
scp "root@45.134.218.19:/opt/donchian/backups/prod-*.tgz" prod_mess\
$t = Get-ChildItem prod_mess\prod-*.tgz | Sort-Object LastWriteTime | Select-Object -Last 1
tar -xzf $t.FullName -C prod_mess
Get-ChildItem prod_mess | Sort-Object LastWriteTime | Select-Object -Last 5
```

В архиве: `data_a/` = 5m, `data_b/` = 1m (sqlite, logs, reports, equity, peek), docker/env (без секретов), git, часы, host.
