import { useEffect, useMemo, useState } from "react";
import {
  api,
  CurvePoint,
  EventRow,
  fmtPct,
  fmtPx,
  fmtTime,
  fmtUsd,
  Mismatch,
  Stats,
  Status,
  Trade,
} from "./api";
import { Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from "recharts";

type Tab = "overview" | "trades" | "health";
type Range = "24h" | "7d" | "30d" | "90d" | "ltd";

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState<Tab>("overview");
  const [status, setStatus] = useState<Status | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [equity, setEquity] = useState<CurvePoint[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [events, setEvents] = useState<EventRow[]>([]);
  const [mismatches, setMismatches] = useState<Mismatch[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [range, setRange] = useState<Range>("ltd");
  const [showBase, setShowBase] = useState(false);
  const [showXF, setShowXF] = useState(false);
  const [symFilter, setSymFilter] = useState<"ALL" | string>("ALL");

  async function refresh() {
    try {
      const [s, t, e, st, ev, mm] = await Promise.all([
        api.status(),
        api.trades(),
        api.equity(),
        api.stats(),
        api.events(),
        api.mismatches(),
      ]);
      setStatus(s);
      setTrades(Array.isArray(t) ? t : []);
      setEquity(Array.isArray(e) ? e : []);
      setStats(st);
      setEvents(Array.isArray(ev) ? ev : []);
      setMismatches(Array.isArray(mm) ? mm : []);
      setErr("");
    } catch (e) {
      if (String(e) === "Error: auth") onLogout();
      else setErr(String(e));
    }
  }

  useEffect(() => {
    void refresh();
    const id = setInterval(() => void refresh(), 15000);
    return () => clearInterval(id);
  }, []);

  const chart = useMemo(() => {
    const cutoff = rangeCutoff(range);
    return equity
      .filter((p) => p.ts >= cutoff)
      .map((p) => ({
        t: new Date(p.ts * 1000).toLocaleString(),
        live: p.equity_live,
        live_xf: p.equity_live_ex_funding,
        ls5: p.equity_shadow_ls5,
        base: p.equity_shadow_baseline,
        gap: p.gap_vs_ls5,
      }));
  }, [equity, range]);

  const markers = useMemo(() => {
    return trades.filter((t) => {
      if (symFilter !== "ALL" && t.symbol !== symFilter) return false;
      return t.profile === "live" || t.profile === "shadow_ls5";
    });
  }, [trades, symFilter]);

  async function kill() {
    if (!confirm("Закрыть все позиции и включить kill-switch?")) return;
    setBusy(true);
    try {
      await api.kill();
      await refresh();
    } finally {
      setBusy(false);
    }
  }

  async function resume() {
    setBusy(true);
    try {
      await api.resume();
      await refresh();
    } finally {
      setBusy(false);
    }
  }

  const r = status?.report;
  const ratio = r?.pnl_ratio_ok ? r.pnl_ratio_xf.toFixed(2) : "NA";

  return (
    <div className="min-h-dvh pb-24">
      <header className="sticky top-0 z-10 backdrop-blur bg-slate-950/80 border-b border-slate-800 px-4 py-3 flex items-center justify-between">
        <div>
          <div className="font-semibold">Donchian Live</div>
          <div className="text-xs text-slate-400">
            {status ? `${status.network}${status.dry_run ? " · dry-run" : ""}` : "…"}
            {status?.strategy ? ` · ${status.strategy}` : ""}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {status?.verdict && <VerdictPill v={status.verdict} />}
          <Pill ok={!!status?.ws_connected} label={status?.ws_connected ? "WS" : "WS down"} />
        </div>
      </header>

      {err && <div className="m-4 rounded-xl bg-rose-950/60 border border-rose-800 px-3 py-2 text-sm">{err}</div>}

      {tab === "overview" && status && (
        <div className="p-4 space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <Card label="Live" value={fmtUsd(status.equity)} sub={r ? `DD ${fmtPct(r.dd_live)}` : undefined} />
            <Card label="Shadow ls5" value={fmtUsd(status.equity_shadow_ls5)} sub={r ? `DD ${fmtPct(r.dd_shadow_ls5)}` : undefined} />
            <Card
              label="Gap vs ls5"
              value={fmtUsd(status.gap_usd)}
              sub={fmtPct(status.gap_pct)}
              bad={status.gap_usd < -50}
            />
            <Card
              label="Cash base"
              value={fmtUsd(status.cash_base || status.start_equity)}
              sub={
                status.last_cash_ts
                  ? `${status.last_cash_kind} ${fmtUsd(status.last_cash_amount)}`
                  : "ждём пополнение"
              }
            />
            <Card label="pnl_ratio_xf" value={ratio} sub={`A/B/C ${status.status_a}/${status.status_b}/${status.status_c}`} />
          </div>

          <div className="flex flex-wrap gap-2 text-xs">
            {(["24h", "7d", "30d", "90d", "ltd"] as Range[]).map((id) => (
              <button
                key={id}
                onClick={() => setRange(id)}
                className={`px-2 py-1 rounded-full border ${range === id ? "border-sky-500 text-sky-300" : "border-slate-700 text-slate-400"}`}
              >
                {id.toUpperCase()}
              </button>
            ))}
            <button
              onClick={() => setShowBase((v) => !v)}
              className={`px-2 py-1 rounded-full border ${showBase ? "border-amber-500 text-amber-300" : "border-slate-700 text-slate-400"}`}
            >
              baseline
            </button>
            <button
              onClick={() => setShowXF((v) => !v)}
              className={`px-2 py-1 rounded-full border ${showXF ? "border-violet-500 text-violet-300" : "border-slate-700 text-slate-400"}`}
            >
              ex-funding
            </button>
            {["ALL", ...((status.symbols ?? []).map((s) => s.symbol) as string[])].map((id) => (
              <button
                key={id}
                onClick={() => setSymFilter(id)}
                className={`px-2 py-1 rounded-full border ${symFilter === id ? "border-emerald-500 text-emerald-300" : "border-slate-700 text-slate-400"}`}
              >
                {id}
              </button>
            ))}
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-3 h-56">
            {chart.length > 1 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chart}>
                  <XAxis dataKey="t" hide />
                  <YAxis hide domain={["auto", "auto"]} />
                  <Tooltip contentStyle={{ background: "#0f172a", border: "1px solid #334155" }} />
                  <Line type="monotone" dataKey="live" stroke="#38bdf8" strokeWidth={2.4} dot={false} name="Live" />
                  <Line type="monotone" dataKey="ls5" stroke="#a3e635" strokeWidth={2} dot={false} name="Shadow ls5" />
                  {showBase && (
                    <Line type="monotone" dataKey="base" stroke="#94a3b8" strokeWidth={1} strokeDasharray="4 3" dot={false} name="Baseline" />
                  )}
                  {showXF && (
                    <Line type="monotone" dataKey="live_xf" stroke="#c084fc" strokeWidth={1.2} dot={false} name="Live xf" />
                  )}
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-slate-500 text-sm">Ждём первую закрытую 1h — без кривых телеметрия не запущена</div>
            )}
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-3 h-28">
            {chart.length > 1 ? (
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chart}>
                  <XAxis dataKey="t" hide />
                  <YAxis hide domain={["auto", "auto"]} />
                  <Tooltip contentStyle={{ background: "#0f172a", border: "1px solid #334155" }} />
                  <Line type="monotone" dataKey="gap" stroke="#f472b6" strokeWidth={1.6} dot={false} name="Gap" />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <div className="h-full flex items-center justify-center text-slate-500 text-sm">gap vs shadow-ls5</div>
            )}
          </div>

          <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-4 text-sm grid grid-cols-2 gap-2">
            <span>match LTD {pct3(status.match_rate_ltd)}</span>
            <span>match 7d {pct3(status.match_rate_7d)}</span>
            <span>live_only 7d {r?.live_only_7d ?? "—"}</span>
            <span>shadow_only 7d {r?.shadow_only_7d ?? "—"}</span>
            <span>slip med {status.side_slip_median_bps.toFixed(1)} bps</span>
            <span>slip p90 {r ? r.side_slip_p90_bps.toFixed(1) : "—"} bps</span>
            <span>funding {fmtUsd(status.funding)}</span>
            <span>SL exit slip {r ? r.sl_exit_slip_median_bps.toFixed(1) : "—"} bps</span>
            <span>WR live {stats ? fmtPct(stats.win_rate) : "—"}</span>
            <span className="text-slate-500">daily PnL {fmtUsd(status.daily_pnl)} (не kill)</span>
          </div>

          {(status.symbols ?? []).map((s) => (
            <div key={s.symbol} className="rounded-2xl border border-slate-800 bg-slate-900/50 p-4">
              <div className="flex justify-between items-baseline">
                <div className="text-lg font-semibold">{s.symbol}</div>
                <div className="text-sky-300">{fmtPx(s.last_price)}</div>
              </div>
              <div className="mt-2 grid grid-cols-2 gap-2 text-sm text-slate-300">
                <span>Live: {s.position}</span>
                <span>Shadow ls5: {s.shadow_ls5 || "FLAT"}</span>
                <span>Qty: {s.qty || "—"}</span>
                <span>Baseline: {s.shadow_baseline || "FLAT"}</span>
                <span>Entry: {fmtPx(s.entry)}</span>
                <span>Stop: {fmtPx(s.stop)}</span>
                <span>Loss streak L/S: {s.consec_losses}/{s.shadow_consec_ls5}</span>
                <span>{s.paused ? "PAUSE ls5" : "active"}</span>
                <span className="col-span-2 text-xs text-slate-500">last bar {fmtTime(s.last_bar_time)} · {s.bars} bars</span>
              </div>
            </div>
          ))}

          {mismatches.length > 0 && (
            <div className="rounded-2xl border border-amber-900 bg-amber-950/30 p-4 space-y-2">
              <div className="text-sm font-medium text-amber-200">Mismatches</div>
              {mismatches.slice(0, 8).map((m, i) => (
                <div key={i} className="text-xs text-slate-300">
                  {m.kind} {m.symbol} {m.direction} · {fmtTime(m.signal_time)} — {m.note}
                </div>
              ))}
            </div>
          )}

          {markers.length > 0 && (
            <div className="text-xs text-slate-500">
              Сделки на графике: {markers.filter((t) => t.profile === "live").length} live / {markers.filter((t) => t.profile === "shadow_ls5").length} shadow-ls5
            </div>
          )}

          {status.kill_switch ? (
            <button disabled={busy} onClick={() => void resume()} className="w-full rounded-2xl bg-emerald-600 py-4 font-medium">
              Снять kill-switch
            </button>
          ) : (
            <button disabled={busy} onClick={() => void kill()} className="w-full rounded-2xl bg-rose-600 py-4 font-medium">
              Kill-switch / flatten
            </button>
          )}
        </div>
      )}

      {tab === "trades" && (
        <div className="p-4 space-y-3">
          {stats && (
            <div className="rounded-2xl border border-slate-800 p-4 text-sm grid grid-cols-2 gap-2">
              <span>Сделок live: {stats.trades}</span>
              <span>PF: {stats.profit_factor.toFixed(2)}</span>
              <span>W/L: {stats.wins}/{stats.losses}</span>
              <span>Net: {fmtUsd(stats.net)}</span>
            </div>
          )}
          {trades.map((t) => (
            <div key={`${t.profile}-${t.id}`} className="rounded-2xl border border-slate-800 bg-slate-900/50 p-4 text-sm">
              <div className="flex justify-between">
                <span className="font-medium">
                  {t.symbol} {t.direction} <span className="text-slate-500">{t.profile}</span>
                </span>
                <span className={t.net >= 0 ? "text-emerald-400" : "text-rose-400"}>{fmtUsd(t.net)}</span>
              </div>
              <div className="text-slate-400 mt-1">
                {t.outcome || "open"} · {fmtPx(t.entry_px_live || t.entry_price)} → {t.exit_px_live || t.exit_price ? fmtPx(t.exit_px_live || t.exit_price) : "…"}
                {t.profile === "live" && t.entry_px_shadow ? (
                  <span className="text-slate-600"> · shadow {fmtPx(t.entry_px_shadow)}</span>
                ) : null}
              </div>
              <div className="text-xs text-slate-500 mt-1">{fmtTime(t.entry_time || t.signal_time)}</div>
            </div>
          ))}
          {trades.length === 0 && <p className="text-slate-500">Сделок пока нет</p>}
        </div>
      )}

      {tab === "health" && status && (
        <div className="p-4 space-y-3 text-sm">
          <Row k="Network" v={status.network} />
          <Row k="Kill-switch" v={String(status.kill_switch)} />
          <Row k="Dry-run" v={String(status.dry_run)} />
          <Row k="Verdict" v={`${status.verdict} · A ${status.status_a} / B ${status.status_b} / C ${status.status_c}`} />
          <Row k="Cash base (shadow)" v={fmtUsd(status.cash_base || status.start_equity)} />
          <Row
            k="Last cash-flow"
            v={
              status.last_cash_ts
                ? `${status.last_cash_kind} ${fmtUsd(status.last_cash_amount)} · ${fmtTime(status.last_cash_ts)}`
                : "ждём первое пополнение счёта"
            }
          />
          <Row k="Go-live" v={fmtTime(status.go_live_ts)} />
          <Row k="WS" v={status.ws_connected ? "connected" : status.ws_error || "down"} />
          <Row k="Uptime" v={`${Math.floor(status.uptime_sec / 60)} мин`} />
          <Row k="Last error" v={status.last_error || "—"} />
          <p className="text-xs text-slate-500">
            Kill только операционный. Не стопать из-за дня/недели/месяца vs дашборд +9%. Матрица §17.13.
          </p>
          <h2 className="pt-2 font-medium">События</h2>
          {events.map((e, i) => (
            <div key={i} className="rounded-xl border border-slate-800 p-3">
              <div className="text-xs text-slate-500">
                {fmtTime(e.ts)} · {e.level}/{e.kind}
              </div>
              <div className="whitespace-pre-wrap">{e.message}</div>
            </div>
          ))}
          <button
            className="w-full rounded-2xl border border-slate-600 py-3"
            onClick={() => {
              void api.logout();
              onLogout();
            }}
          >
            Выйти
          </button>
        </div>
      )}

      <nav className="fixed bottom-0 left-0 right-0 bg-slate-950/95 border-t border-slate-800 grid grid-cols-3">
        {(["overview", "trades", "health"] as Tab[]).map((id) => (
          <button
            key={id}
            onClick={() => setTab(id)}
            className={`py-3 text-sm ${tab === id ? "text-sky-300" : "text-slate-400"}`}
          >
            {id === "overview" ? "Обзор" : id === "trades" ? "Сделки" : "Health"}
          </button>
        ))}
      </nav>
    </div>
  );
}

function rangeCutoff(r: Range): number {
  const now = Date.now() / 1000;
  switch (r) {
    case "24h":
      return now - 86400;
    case "7d":
      return now - 7 * 86400;
    case "30d":
      return now - 30 * 86400;
    case "90d":
      return now - 90 * 86400;
    default:
      return 0;
  }
}

function pct3(n: number) {
  if (!n && n !== 0) return "—";
  return `${(n * 100).toFixed(1)}%`;
}

function Card({ label, value, sub, bad }: { label: string; value: string; sub?: string; bad?: boolean }) {
  return (
    <div className="rounded-2xl border border-slate-800 bg-slate-900/50 p-4">
      <div className="text-xs text-slate-400">{label}</div>
      <div className={`text-lg font-semibold mt-1 ${bad ? "text-rose-400" : ""}`}>{value}</div>
      {sub && <div className="text-xs text-slate-500 mt-1">{sub}</div>}
    </div>
  );
}

function Pill({ ok, label }: { ok: boolean; label: string }) {
  return (
    <span className={`text-xs px-2 py-1 rounded-full border ${ok ? "border-emerald-700 text-emerald-300" : "border-rose-700 text-rose-300"}`}>
      {label}
    </span>
  );
}

function VerdictPill({ v }: { v: string }) {
  const cls =
    v === "STOP" ? "border-rose-600 text-rose-300" : v === "INVESTIGATE" ? "border-amber-600 text-amber-300" : "border-sky-600 text-sky-300";
  return <span className={`text-xs px-2 py-1 rounded-full border ${cls}`}>{v}</span>;
}

function Row({ k, v }: { k: string; v: string }) {
  return (
    <div className="flex justify-between gap-3 rounded-xl border border-slate-800 px-3 py-2">
      <span className="text-slate-400">{k}</span>
      <span className="text-right break-all">{v}</span>
    </div>
  );
}
