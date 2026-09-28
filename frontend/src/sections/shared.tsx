import { ActivityIcon, InfoIcon } from "lucide-react";
import { CurvePoint, Trade, fmtBps, fmtPx, fmtUsdCompact } from "../api";
import { checkLabel, layerMeta } from "../labels";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig } from "@/components/ui/chart";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Field, FieldTitle } from "@/components/ui/field";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { verdictLabel } from "../labels";

export type Range = "24h" | "7d" | "30d" | "90d" | "ltd";
export type BookFilter = "all" | "live" | "shadow_ls5" | "shadow_baseline";

export const RANGE_LABEL: Record<Range, string> = {
  "24h": "сутки",
  "7d": "неделя",
  "30d": "месяц",
  "90d": "90 дней",
  ltd: "всё",
};

export const equityConfig = {
  live: { label: "Live", color: "var(--chart-1)" },
  ls5: { label: "Тень (фильтры)", color: "var(--chart-2)" },
  base: { label: "Baseline", color: "var(--chart-3)" },
  live_xf: { label: "Live без фандинга", color: "var(--chart-4)" },
} satisfies ChartConfig;

export const gapConfig = {
  gap: { label: "Live − тень", color: "var(--chart-5)" },
} satisfies ChartConfig;

export const dailyPnlConfig = {
  net: { label: "PnL за день", color: "var(--chart-1)" },
} satisfies ChartConfig;

export function shadowEquity(status: { equity_shadow_twin?: number; equity_shadow_ls5: number }) {
  return status.equity_shadow_twin ?? status.equity_shadow_ls5;
}

export function rangeCutoff(r: Range): number {
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

export function buildChart(equity: CurvePoint[], range: Range) {
  const cutoff = rangeCutoff(range);
  const byTs = new Map<number, CurvePoint>();
  for (const p of equity) {
    if (p.ts < cutoff) continue;
    const prev = byTs.get(p.ts);
    if (!prev || p.reason === "bar" || (p.reason === "boot" && prev.reason === "fill")) {
      byTs.set(p.ts, p);
    }
  }
  const rows = [...byTs.values()].sort((a, b) => a.ts - b.ts);
  const max = 720;
  const step = rows.length > max ? Math.ceil(rows.length / max) : 1;
  return rows
    .filter((_, i) => i % step === 0 || i === rows.length - 1)
    .map((p) => ({
      t: new Date(p.ts * 1000).toLocaleString("ru-RU", {
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      }),
      live: p.equity_live,
      live_xf: p.equity_live_ex_funding,
      ls5: p.equity_shadow_ls5,
      base: p.equity_shadow_baseline,
      gap: p.gap_vs_ls5,
    }));
}

export function pct3(n: number) {
  if (!n && n !== 0) return "—";
  return `${(n * 100).toFixed(1)}%`;
}

export function pricePath(t: Trade) {
  const entry = fmtPx(t.entry_px_live || t.entry_price);
  const exit = t.exit_px_live || t.exit_price;
  return exit ? `${entry} → ${fmtPx(exit)}` : `${entry} → …`;
}

export function totalFees(t: Trade) {
  return (t.entry_fee ?? 0) + (t.exit_fee ?? 0);
}

export function checkVariant(v: string): "default" | "secondary" | "destructive" | "outline" {
  switch (v) {
    case "red":
      return "destructive";
    case "green":
      return "default";
    case "na":
      return "secondary";
    default:
      return "outline";
  }
}

export function VerdictBadge({ verdict }: { verdict: string }) {
  const variant = verdict === "STOP" ? "destructive" : verdict === "INVESTIGATE" ? "outline" : "secondary";
  return <Badge variant={variant}>{verdictLabel(verdict)}</Badge>;
}

export function Hint({ text }: { text: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="Пояснение">
          <InfoIcon />
        </Button>
      </TooltipTrigger>
      <TooltipContent className="max-w-xs">{text}</TooltipContent>
    </Tooltip>
  );
}

export function QuietEmpty({ title, text }: { title: string; text: string }) {
  return (
    <Empty className="border border-dashed">
      <EmptyHeader>
        <EmptyMedia variant="icon">
          <ActivityIcon />
        </EmptyMedia>
        <EmptyTitle>{title}</EmptyTitle>
        <EmptyDescription>{text}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  );
}

export function StatCard({
  title,
  value,
  sub,
  hint,
  warn,
}: {
  title: string;
  value: string;
  sub?: string;
  hint?: string;
  warn?: boolean;
}) {
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-1">
          {title}
          {hint ? <Hint text={hint} /> : null}
        </CardTitle>
      </CardHeader>
      <CardContent>
        <div className={cn("font-heading text-xl font-medium tabular-nums", warn && "text-destructive")}>{value}</div>
        {sub ? <p className="mt-1 text-xs text-muted-foreground">{sub}</p> : null}
      </CardContent>
    </Card>
  );
}

export function CheckCard({ id, value, detail }: { id: "a" | "b" | "c"; value: string; detail: string }) {
  const meta = layerMeta(id);
  const isNa = value?.toLowerCase() === "na";
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-1">
          {meta.title}
          <Hint text={meta.hint} />
        </CardTitle>
        <CardAction>
          <Badge variant={checkVariant(value)}>{checkLabel(value)}</Badge>
        </CardAction>
      </CardHeader>
      <CardContent>
        <p className="text-sm text-muted-foreground">{isNa ? "Пока рано — мало данных для оценки." : detail}</p>
      </CardContent>
    </Card>
  );
}

export function Meta({ k, v, hint }: { k: string; v: string; hint?: string }) {
  return (
    <Field>
      <FieldTitle className="flex items-center gap-1 text-muted-foreground">
        {k}
        {hint ? <Hint text={hint} /> : null}
      </FieldTitle>
      <p className="break-words text-sm">{v}</p>
    </Field>
  );
}

export function SlipCallout({
  title,
  hint,
  stats,
}: {
  title: string;
  hint: string;
  stats: { median_bps: number; p90_bps: number; n: number };
}) {
  const empty = !stats.n;
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle className="flex items-center gap-1 text-base">
          {title}
          <Hint text={hint} />
        </CardTitle>
      </CardHeader>
      <CardContent>
        {empty ? (
          <p className="text-sm text-muted-foreground">Пока нет закрытых Live с парой цен тень/Live.</p>
        ) : (
          <div className="grid gap-1 text-sm tabular-nums">
            <p>
              медиана <span className="font-medium">{fmtBps(stats.median_bps)}</span>
            </p>
            <p>
              p90 <span className="font-medium">{fmtBps(stats.p90_bps)}</span>
            </p>
            <p className="text-xs text-muted-foreground">по {stats.n} сделкам</p>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

export { fmtUsdCompact };
