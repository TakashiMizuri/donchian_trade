#!/usr/bin/env python3
"""Compare old vs new Lighter candle-close detection latency on 1m bars.

Old: dual-candle WS payload only (pre LATENCY_CANDLE_CLOSE patch).
New: WS t-advance on cached live bar + boundary REST polls at +200/500/1000/2000ms.

Runs against mainnet public WS/REST. No API key required.
"""

from __future__ import annotations

import argparse
import json
import statistics
import threading
import time
import urllib.parse
import urllib.request
from dataclasses import dataclass, field
from datetime import datetime, timezone

import websocket

WS_URL = "wss://mainnet.zklighter.elliot.ai/stream"
REST = "https://mainnet.zklighter.elliot.ai/api/v1/candles"
TF_SEC = 60


def utc_now() -> float:
    return time.time()


def bar_close_wall(bar_open_sec: int) -> float:
    return float(bar_open_sec + TF_SEC)


def candle_time_sec(t: int | float) -> int:
    t = int(t)
    if t > 1_000_000_000_000:
        return t // 1000
    return t


def fetch_candles(market_id: int, count_back: int = 5) -> list[dict]:
    end = int(time.time() * 1000)
    start = end - 2 * 3600 * 1000
    q = urllib.parse.urlencode(
        {
            "market_id": market_id,
            "resolution": "1m",
            "start_timestamp": start,
            "end_timestamp": end,
            "count_back": count_back,
        }
    )
    with urllib.request.urlopen(f"{REST}?{q}", timeout=10) as r:
        data = json.loads(r.read().decode())
    return data.get("c") or data.get("candles") or []


@dataclass
class Detector:
    name: str
    lag_by_bar: dict[int, float] = field(default_factory=dict)
    source_by_bar: dict[int, str] = field(default_factory=dict)
    lock: threading.Lock = field(default_factory=threading.Lock)

    def note(self, bar_open_sec: int, source: str, detect_ts: float | None = None) -> None:
        with self.lock:
            if bar_open_sec in self.lag_by_bar:
                return
            detect = detect_ts if detect_ts is not None else utc_now()
            lag = (detect - bar_close_wall(bar_open_sec)) * 1000
            self.lag_by_bar[bar_open_sec] = lag
            self.source_by_bar[bar_open_sec] = source
            wall = datetime.fromtimestamp(bar_close_wall(bar_open_sec), tz=timezone.utc).strftime("%H:%M:%S")
            print(
                f"[{self.name:3}] close@{wall}Z  lag={lag:7.0f} ms  via={source}",
                flush=True,
            )

    def lags(self) -> list[float]:
        return list(self.lag_by_bar.values())


class Bench:
    def __init__(self, market_id: int, duration_sec: float):
        self.market_id = market_id
        self.duration_sec = duration_sec
        self.started = utc_now()
        self.deadline = self.started + duration_sec
        # Only score bars whose official close is at/after bench start (ignore REST history).
        self.min_close_wall = self.started
        self.old = Detector("OLD")
        self.new = Detector("NEW")
        self.last_live_t: int | None = None
        self.stop = threading.Event()
        self.ws_msgs = 0
        self.ws_dual = 0
        self.ws_single = 0

    def in_window(self, bar_open_sec: int) -> bool:
        return bar_close_wall(bar_open_sec) >= self.min_close_wall

    def on_candles(self, candles: list[dict]) -> None:
        if not candles:
            return
        detect = utc_now()
        if len(candles) >= 2:
            self.ws_dual += 1
            closed_t = candle_time_sec(candles[0]["t"])
            live_t = candle_time_sec(candles[1]["t"])
            if self.in_window(closed_t):
                self.old.note(closed_t, "ws_dual", detect)
                self.new.note(closed_t, "ws_dual", detect)
            self.last_live_t = live_t
            return

        self.ws_single += 1
        live = candles[0]
        live_t = candle_time_sec(live["t"])
        if self.last_live_t is not None and live_t > self.last_live_t:
            if self.in_window(self.last_live_t):
                self.new.note(self.last_live_t, "ws_t_advance", detect)
        self.last_live_t = live_t

    def boundary_loop(self) -> None:
        offsets = (0.2, 0.5, 1.0, 2.0)
        while not self.stop.is_set() and utc_now() < self.deadline:
            now = utc_now()
            boundary = (int(now) // TF_SEC) * TF_SEC + TF_SEC
            for off in offsets:
                target = boundary + off
                wait = target - utc_now()
                if wait > 0 and self.stop.wait(wait):
                    return
                if utc_now() >= self.deadline:
                    return
                try:
                    rows = fetch_candles(self.market_id, count_back=4)
                except Exception as e:
                    print(f"[REST] error: {e}", flush=True)
                    continue
                detect = utc_now()
                cutoff = (int(detect) // TF_SEC) * TF_SEC
                for c in rows:
                    t = candle_time_sec(c["t"])
                    if t < cutoff and self.in_window(t):
                        self.new.note(t, f"rest+{int(off * 1000)}ms", detect)
            nxt = (int(utc_now()) // TF_SEC) * TF_SEC + TF_SEC
            wait = nxt - utc_now()
            if wait > 0 and self.stop.wait(wait):
                return

    def run_ws(self) -> None:
        sub = json.dumps({"type": "subscribe", "channel": f"candle/{self.market_id}/1m"})

        def on_message(_ws, message: str) -> None:
            self.ws_msgs += 1
            try:
                msg = json.loads(message)
            except json.JSONDecodeError:
                return
            ch = msg.get("channel") or ""
            typ = msg.get("type") or ""
            if "candle" not in ch and "candle" not in typ:
                return
            self.on_candles(msg.get("candles") or [])
            if utc_now() >= self.deadline:
                try:
                    _ws.close()
                except Exception:
                    pass

        def on_open(ws) -> None:
            print(f"WS connected, subscribe candle/{self.market_id}/1m", flush=True)
            ws.send(sub)

        def on_error(_ws, err) -> None:
            print(f"WS error: {err}", flush=True)

        while utc_now() < self.deadline and not self.stop.is_set():
            ws = websocket.WebSocketApp(
                WS_URL,
                on_open=on_open,
                on_message=on_message,
                on_error=on_error,
            )
            t = threading.Thread(
                target=lambda: ws.run_forever(ping_interval=20, ping_timeout=10),
                daemon=True,
            )
            t.start()
            while t.is_alive() and utc_now() < self.deadline and not self.stop.is_set():
                time.sleep(0.2)
            try:
                ws.close()
            except Exception:
                pass
            t.join(timeout=2)
            if utc_now() < self.deadline and not self.stop.is_set():
                time.sleep(1)

    def summary(self, det: Detector) -> str:
        xs = det.lags()
        if not xs:
            return f"{det.name}: no closes detected"
        src = {}
        for s in det.source_by_bar.values():
            src[s] = src.get(s, 0) + 1
        return (
            f"{det.name}: n={len(xs)}  "
            f"med={statistics.median(xs):.0f} ms  "
            f"mean={statistics.mean(xs):.0f} ms  "
            f"min={min(xs):.0f}  max={max(xs):.0f}  "
            f"sources={src}"
        )


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--market-id", type=int, default=1, help="Lighter market id (BTC=1)")
    ap.add_argument("--minutes", type=float, default=5.0)
    args = ap.parse_args()
    duration = args.minutes * 60
    print(
        f"Bench candle-close lag · market={args.market_id} · 1m · {args.minutes} min · "
        f"start {datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')}Z",
        flush=True,
    )
    print("OLD = dual-candle WS only | NEW = t-advance WS + boundary REST", flush=True)
    print("---", flush=True)

    b = Bench(args.market_id, duration)
    rest_t = threading.Thread(target=b.boundary_loop, daemon=True)
    rest_t.start()
    try:
        b.run_ws()
    finally:
        b.stop.set()
        rest_t.join(timeout=5)

    print("---", flush=True)
    print(f"WS messages={b.ws_msgs} dual={b.ws_dual} single={b.ws_single}", flush=True)
    print(b.summary(b.old), flush=True)
    print(b.summary(b.new), flush=True)

    common = sorted(set(b.old.lag_by_bar) & set(b.new.lag_by_bar))
    if common:
        deltas = [b.old.lag_by_bar[k] - b.new.lag_by_bar[k] for k in common]
        print(
            f"Paired bars={len(common)}  OLD-NEW median save={statistics.median(deltas):.0f} ms  "
            f"mean save={statistics.mean(deltas):.0f} ms  "
            f"(positive = NEW faster)",
            flush=True,
        )
        for k in common:
            wall = datetime.fromtimestamp(bar_close_wall(k), tz=timezone.utc).strftime("%H:%M:%S")
            o, n = b.old.lag_by_bar[k], b.new.lag_by_bar[k]
            print(
                f"  {wall}Z  OLD={o:7.0f} ({b.old.source_by_bar[k]})  "
                f"NEW={n:7.0f} ({b.new.source_by_bar[k]})  save={o-n:.0f} ms",
                flush=True,
            )
    only_new = sorted(set(b.new.lag_by_bar) - set(b.old.lag_by_bar))
    if only_new:
        print(f"Bars seen only by NEW (OLD missed in window): {len(only_new)}", flush=True)


if __name__ == "__main__":
    main()
