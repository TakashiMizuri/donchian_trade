# Как работает наша стратегия

Этот файл — объяснение простыми словами. Не инструкция по деплою и не research-отчёт.

**Текущий live-кандидат:** `5m_N360_M180_ATR240_R1.0_brk0_voloff_tstop20h_late18h_2R`  
Контракт: [`DONCHIAN_LIVE_STARTPACK_5M_BRK0_VOLOFF_TSTOP20_LATE_TRAIL2.md`](DONCHIAN_LIVE_STARTPACK_5M_BRK0_VOLOFF_TSTOP20_LATE_TRAIL2.md).  
(Primary research default по-прежнему описывается 15m-паком; 5m — отдельный run_id.)

Числа живут в `.env` (`DONCHIAN_*`). Смена TF/N/M/tstop — **новый** `bot.db`.

---

## 1. Одной фразой

Пробой Дончиана на **5-минутных** свечах (календарно тот же горизонт, что 1h N30 / 15m N120: **360×5m ≈ 30 часов**). Live входит на любой пробой канала (brk/vol **выкл**). Выход: ATR-стоп, короткий канал M, **time-stop 20 часов** (240 баров), или **late trail** с 18-го часа (peak прошлого бара − 2R), если MFE ≥ 6R.

---

## 2. Параметры (не крутить по живой неделе)

| Что | Значение | Смысл |
|-----|----------|--------|
| TF | 5m | Закрытые 5m свечи |
| N / M / ATR | 360 / 180 / 240×1.5 | Канал входа / выхода / стоп |
| Риск | 1%, cap $625 | На стоп |
| brk / vol | **0 / 0 (off)** | Live без фильтров входа |
| vol окна | bars 240, lookback 12000 | Не используются при vol=0 |
| tstop | **240 баров = 20h** | Live и twin |
| late trail | **216 / 2R / MFE 6** | Peak лагается; fill на trail_stop |

Warmup vol_rank ≈ **42+ дня** истории.

---

## 3. Книги

| Книга | Фильтры | Tstop | Зачем |
|-------|---------|-------|--------|
| Live | нет | 20h + trail 2R | Реальные деньги |
| Тень (DB `shadow_ls5`) | нет | 20h + trail 2R | Та же стратегия, бумажные филлы |
| Baseline | нет | выкл | Чистый канал без trail |

---

## 4. Выходы

1. **ATR-стоп** — важнее всего (fill на orig стопе, `outcome=sl`).  
2. Иначе **late trail** (если bars_held ≥ 216 и lagged MFE ≥ 6R) — wick через `peak − 2R`, fill **сейчас** на trail_stop, `outcome=time`.  
3. Иначе **канал M** или **tstop** → market на open следующего бара, `outcome=time`.

На 5m профиль чувствителен к slip (~4–5 bps half-EW в research). Не поднимать size, пока Live↔Тень и slip не выглядят нормально.

---

## 5. Чего не делать

- Не копировать tstop «48h wall-clock» (=576 баров на 5m) — research failed.  
- Не лить 5m в старый 15m `bot.db`.  
- Не ретюнить 240 / 216 / 2 / 6 / N/M и не возвращать brk/vol по первым красным неделям.

Логи анализа: `LOG_DIR` → `bot-YYYYMMDD.jsonl`. Детект close: [`LATENCY_CANDLE_CLOSE.md`](LATENCY_CANDLE_CLOSE.md).
