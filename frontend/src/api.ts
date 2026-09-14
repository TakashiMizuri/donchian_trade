export type SymbolSnap = {
  symbol: string;
  market_id: number;
  last_bar_time: number;
  last_price: number;
  bars: number;
  position: string;
  entry: number;
  stop: number;
  qty: number;
  consec_losses: number;
  pause_until_time: number;
  paused: boolean;
  shadow_ls5: string;
  shadow_baseline: string;
  shadow_consec_ls5: number;
};

export type Report = {
  equity_live: number;
  equity_shadow_ls5: number;
  equity_shadow_baseline: number;
  equity_live_ex_funding: number;
  gap_usd: number;
  gap_pct: number;
  dd_live: number;
  dd_shadow_ls5: number;
  match_rate_ltd: number;
  match_rate_dice: number;
  match_rate_7d: number;
  live_only_7d: number;
  shadow_only_7d: number;
  n_matched: number;
  n_union: number;
  side_slip_median_bps: number;
  side_slip_p90_bps: number;
  sl_exit_slip_median_bps: number;
  live_net: number;
  live_net_xf: number;
  shadow_net: number;
  funding: number;
  pnl_ratio_xf: number;
  pnl_ratio_ok: boolean;
  status_a: string;
  status_b: string;
  status_c: string;
  verdict: string;
  mismatches: { Symbol?: string; symbol?: string; SignalTime?: number; signal_time?: number; Direction?: string; direction?: string }[];
};

export type Status = {
  network: string;
  strategy: string;
  kill_switch: boolean;
  dry_run: boolean;
  ws_connected: boolean;
  ws_error: string;
  uptime_sec: number;
  daily_pnl: number;
  equity: number;
  equity_shadow_ls5: number;
  equity_shadow_baseline: number;
  equity_live_ex_funding: number;
  gap_usd: number;
  gap_pct: number;
  pnl_ratio_xf: number;
  pnl_ratio_ok: boolean;
  verdict: string;
  status_a: string;
  status_b: string;
  status_c: string;
  match_rate_ltd: number;
  match_rate_7d: number;
  side_slip_median_bps: number;
  funding: number;
  start_equity: number;
  cash_base: number;
  last_cash_kind: string;
  last_cash_amount: number;
  last_cash_ts: number;
  go_live_ts: number;
  available: number;
  last_error: string;
  symbols: SymbolSnap[];
  report: Report;
};

export type Trade = {
  id: number;
  symbol: string;
  profile: string;
  direction: string;
  signal_time: number;
  entry_time: number;
  entry_price: number;
  stop: number;
  exit_time: number;
  exit_price: number;
  outcome: string;
  quantity: number;
  gross: number;
  net: number;
  funding: number;
  consec_losses: number;
  entry_px_shadow?: number;
  entry_px_live?: number;
  exit_px_shadow?: number;
  exit_px_live?: number;
};

export type CurvePoint = {
  ts: number;
  reason: string;
  equity_live: number;
  equity_live_ex_funding: number;
  upnl_live: number;
  wallet_cash: number;
  equity_shadow_ls5: number;
  equity_shadow_baseline: number;
  gap_vs_ls5: number;
  cum_funding: number;
  cum_fees_live: number;
  cum_fees_shadow_ls5: number;
  match_rate_ltd: number;
  side_slip_median_ltd_bps: number;
  verdict: string;
};

export type Mismatch = {
  ts: number;
  kind: string;
  symbol: string;
  direction: string;
  signal_time: number;
  note: string;
};

export type EventRow = { ts: number; level: string; kind: string; message: string };
export type Stats = {
  trades: number;
  wins: number;
  losses: number;
  win_rate: number;
  profit_factor: number;
  net: number;
  max_dd: number;
};

async function req<T>(url: string, init?: RequestInit): Promise<T> {
  const r = await fetch(url, { credentials: "include", ...init });
  if (r.status === 401) {
    throw new Error("auth");
  }
  if (!r.ok) {
    const t = await r.text();
    throw new Error(t || r.statusText);
  }
  return r.json() as Promise<T>;
}

export const api = {
  login: (password: string) =>
    req<{ ok: string }>("/api/login", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password }),
    }),
  logout: () => fetch("/api/logout", { method: "POST", credentials: "include" }),
  status: () => req<Status>("/api/status"),
  trades: () => req<Trade[]>("/api/trades"),
  equity: () => req<CurvePoint[]>("/api/equity"),
  stats: () => req<Stats>("/api/stats"),
  events: () => req<EventRow[]>("/api/events"),
  telemetry: () => req<Report>("/api/telemetry"),
  mismatches: () => req<Mismatch[]>("/api/mismatches"),
  kill: () => req<{ ok: string }>("/api/kill", { method: "POST" }),
  resume: () => req<{ ok: string }>("/api/resume", { method: "POST" }),
};

export function fmtPx(n: number) {
  if (!n) return "—";
  return n.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

export function fmtUsd(n: number) {
  return n.toLocaleString("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 });
}

export function fmtTime(sec: number) {
  if (!sec) return "—";
  return new Date(sec * 1000).toLocaleString();
}

export function fmtPct(n: number) {
  return `${(n * 100).toFixed(1)}%`;
}
