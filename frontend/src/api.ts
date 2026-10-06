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
  equity_shadow_twin?: number;
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
  instance_id?: string;
  instance_name?: string;
  kill_switch: boolean;
  dry_run: boolean;
  ws_connected: boolean;
  ws_error: string;
  uptime_sec: number;
  daily_pnl: number;
  equity: number;
  equity_shadow_ls5: number;
  equity_shadow_twin?: number;
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
  entry_fee?: number;
  exit_fee?: number;
  net: number;
  funding: number;
  consec_losses: number;
  entry_px_shadow?: number;
  entry_px_live?: number;
  exit_px_shadow?: number;
  exit_px_live?: number;
  entry_slip_bps?: number;
  exit_slip_bps?: number;
  side_slip_bps?: number;
  r_multiple?: number;
  risk_usd?: number;
  risk_distance?: number;
};

export type SlipStats = {
  median_bps: number;
  p90_bps: number;
  n: number;
};

export type Analysis = {
  trades: number;
  wins: number;
  losses: number;
  win_rate: number;
  avg_r: number;
  expectancy_r: number;
  net: number;
  loss_streak: number;
  max_loss_streak: number;
  days_since_go_live: number;
  go_live_ts: number;
  entry_slip: SlipStats;
  exit_slip: SlipStats;
  side_slip: SlipStats;
  sl_exit_slip: SlipStats;
  skip_counts: Record<string, number>;
  daily_pnl: { date: string; net: number }[];
  curve_points: number;
  match_rate_ltd: number;
  verdict: string;
};

export type BarLog = {
  symbol: string;
  bar_time: number;
  atr: number;
  upper_n: number;
  lower_n: number;
  close: number;
  want_baseline: boolean;
  want_live: boolean;
  live_desired: string;
  live_actual: string;
  shadow_twin: string;
  shadow_baseline: string;
  breakout_atr: number;
  vol_rank: number;
  skipped_reason: string;
};

export type CurvePoint = {
  ts: number;
  reason: string;
  equity_live: number;
  equity_live_ex_funding: number;
  upnl_live: number;
  wallet_cash: number;
  equity_shadow_ls5: number;
  equity_shadow_twin?: number;
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
export type CashFlow = {
  id: number;
  ts: number;
  amount: number;
  kind: string;
  live_equity: number;
  note: string;
};
export type DailyReport = { date_utc: string; body: string; verdict: string };
export type Stats = {
  trades: number;
  wins: number;
  losses: number;
  win_rate: number;
  profit_factor: number;
  net: number;
  max_dd: number;
};

export type InstanceInfo = {
  id: string;
  name: string;
  prefix: string;
};

const INSTANCE_KEY = "donchian_instance_id";
const DEFAULT_INSTANCES: InstanceInfo[] = [
  { id: "a", name: "main", prefix: "/api/a" },
  { id: "b", name: "alt", prefix: "/api/b" },
];

let instancesCache: InstanceInfo[] | null = null;
let currentInstanceId = localStorage.getItem(INSTANCE_KEY) || "a";

export function getInstanceId() {
  return currentInstanceId;
}

export function getApiPrefix() {
  const list = instancesCache ?? DEFAULT_INSTANCES;
  const hit = list.find((i) => i.id === currentInstanceId) ?? list[0] ?? DEFAULT_INSTANCES[0];
  return hit.prefix;
}

export function setInstanceId(id: string) {
  currentInstanceId = id;
  localStorage.setItem(INSTANCE_KEY, id);
}

export async function loadInstances(): Promise<InstanceInfo[]> {
  try {
    const r = await fetch("/api/instances", { credentials: "include" });
    if (!r.ok) throw new Error("instances");
    const body = (await r.json()) as { instances?: InstanceInfo[] };
    const list = Array.isArray(body.instances) && body.instances.length ? body.instances : DEFAULT_INSTANCES;
    instancesCache = list;
    if (!list.some((i) => i.id === currentInstanceId)) {
      setInstanceId(list[0].id);
    }
    return list;
  } catch {
    instancesCache = DEFAULT_INSTANCES;
    return DEFAULT_INSTANCES;
  }
}

function apiPath(path: string) {
  // path like "/status" or "/bar_logs?x=1"
  const p = path.startsWith("/") ? path : `/${path}`;
  return `${getApiPrefix()}${p}`;
}

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const r = await fetch(apiPath(path), { credentials: "include", ...init });
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
  instances: () => loadInstances(),
  login: async (password: string) => {
    const list = await loadInstances();
    const body = JSON.stringify({ password });
    const results = await Promise.all(
      list.map(async (inst) => {
        const r = await fetch(`${inst.prefix}/login`, {
          method: "POST",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body,
        });
        return { id: inst.id, ok: r.ok };
      }),
    );
    if (!results.some((x) => x.ok)) {
      throw new Error("auth");
    }
    return { ok: "1" };
  },
  logout: async () => {
    const list = await loadInstances();
    await Promise.all(
      list.map((inst) => fetch(`${inst.prefix}/logout`, { method: "POST", credentials: "include" })),
    );
  },
  status: () => req<Status>("/status"),
  trades: () => req<Trade[]>("/trades"),
  equity: () => req<CurvePoint[]>("/equity"),
  stats: () => req<Stats>("/stats"),
  events: () => req<EventRow[]>("/events"),
  telemetry: () => req<Report>("/telemetry"),
  mismatches: () => req<Mismatch[]>("/mismatches"),
  cash: () => req<{ cash_base: number; flows: CashFlow[] }>("/cash"),
  report: () => req<DailyReport>("/report"),
  analysis: () => req<Analysis>("/analysis"),
  barLogs: (symbol?: string, limit?: number) => {
    const q = new URLSearchParams();
    if (symbol) q.set("symbol", symbol);
    if (limit != null) q.set("limit", String(limit));
    const qs = q.toString();
    return req<BarLog[]>(`/bar_logs${qs ? `?${qs}` : ""}`);
  },
  kill: () => req<{ ok: string }>("/kill", { method: "POST" }),
  resume: () => req<{ ok: string }>("/resume", { method: "POST" }),
};

export function fmtPx(n: number) {
  if (!n) return "—";
  return n.toLocaleString("en-US", { maximumFractionDigits: 2 });
}

export function fmtUsd(n: number) {
  return n.toLocaleString("ru-RU", { style: "currency", currency: "USD", maximumFractionDigits: 2 });
}

export function fmtUsdCompact(n: number) {
  const abs = Math.abs(n);
  const sign = n < 0 ? "−" : "";
  if (abs >= 1_000_000) return `${sign}$${(abs / 1_000_000).toFixed(1)}M`;
  if (abs >= 10_000) return `${sign}$${Math.round(abs / 1000)}k`;
  return fmtUsd(n);
}

export function fmtTime(sec: number) {
  if (!sec) return "—";
  return new Date(sec * 1000).toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function fmtPct(n: number) {
  return `${(n * 100).toFixed(1)}%`;
}

export function fmtBps(n: number) {
  if (!Number.isFinite(n)) return "—";
  return `${n.toFixed(1)} б.п.`;
}

export function fmtQty(n: number) {
  if (!n) return "—";
  return n.toLocaleString("en-US", { maximumFractionDigits: 6 });
}

export function fmtUptime(sec: number) {
  if (!sec || sec < 0) return "—";
  if (sec < 60) return `${Math.floor(sec)} с`;
  const m = Math.floor(sec / 60);
  if (m < 60) return `${m} мин`;
  const h = Math.floor(m / 60);
  const rm = m % 60;
  if (h < 24) return rm ? `${h} ч ${rm} мин` : `${h} ч`;
  const d = Math.floor(h / 24);
  const rh = h % 24;
  return rh ? `${d} д ${rh} ч` : `${d} д`;
}
