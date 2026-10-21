# Операции на проде

Каталог на VPS: `/opt/donchian`.  
`bot_a` = 5m, `bot_b` = 1m.  
Файлы `.env.a` и `.env.b` **не удалять**.

Перед полным сбросом на **обоих** аккаунтах Lighter не должно быть открытых позиций и висящих стопов. После пустой базы бот считает себя flat и чужую позицию на бирже закроет рынком.

Менял `.env` → всегда `docker compose up -d --force-recreate …`, не просто `restart`.  
`KILL_SWITCH=false` в файле ≠ в процессе, пока не было recreate.

---

## 1.0 Остановка, полный сброс, pull, перезапуск — без бэкапа

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
docker compose exec bot_a printenv KILL_SWITCH
docker compose exec bot_b printenv KILL_SWITCH
# оба: false

docker compose exec bot_a wget -qO- http://127.0.0.1:8080/api/health

docker compose logs --tail=80 bot_a
docker compose logs -f bot_b
```

В логах: `bootstrapped` / `boot catchup ok`, `http listen`.  
У `bot_b` (1m) прогрев **долгий** — контейнер не убивать.

Дашборд: аварийный стоп выкл, Live/тень ≈ депозит, позиции FLAT, лаг свечей сотни мс (не минуты).  
Первый бар может не дать сделку — нормально.

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
docker compose exec -T bot_a printenv KILL_SWITCH INSTANCE_ID INSTANCE_NAME DONCHIAN_TIMEFRAME DONCHIAN_TAG 2>/dev/null > "$OUT/env_runtime_a.txt"
docker compose exec -T bot_b printenv KILL_SWITCH INSTANCE_ID INSTANCE_NAME DONCHIAN_TIMEFRAME DONCHIAN_TAG 2>/dev/null > "$OUT/env_runtime_b.txt"

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
