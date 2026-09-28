import { useMemo } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import { Analysis, Trade, fmtBps, fmtTime, fmtUsd } from "../api";
import { outcomeLabel, sideLabel } from "../labels";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { Meta, QuietEmpty, SlipCallout, StatCard, dailyPnlConfig, fmtUsdCompact } from "./shared";

type Props = {
  analysis: Analysis | null;
  trades: Trade[];
};

export function ExecutionTab({ analysis, trades }: Props) {
  const dailyChart = useMemo(() => {
    if (!analysis?.daily_pnl?.length) return [];
    return analysis.daily_pnl.map((d) => ({
      date: d.date.slice(5),
      net: d.net,
      full: d.date,
    }));
  }, [analysis]);

  const outliers = useMemo(() => {
    if (!analysis) return [];
    const p90 = analysis.side_slip.p90_bps;
    if (!analysis.side_slip.n || !Number.isFinite(p90)) return [];
    return trades
      .filter((t) => t.profile === "live" && t.outcome && t.outcome !== "open")
      .filter((t) => (t.side_slip_bps ?? 0) >= p90 && (t.side_slip_bps ?? 0) > 0)
      .sort((a, b) => (b.side_slip_bps ?? 0) - (a.side_slip_bps ?? 0))
      .slice(0, 15);
  }, [analysis, trades]);

  if (!analysis) {
    return <QuietEmpty title="Нет данных исполнения" text="Не удалось загрузить /api/analysis." />;
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard title="Закрыто Live" value={String(analysis.trades)} sub={`${analysis.wins} / ${analysis.losses} ±`} />
        <StatCard title="Win rate" value={`${(analysis.win_rate * 100).toFixed(1)}%`} />
        <StatCard title="Средний R" value={analysis.avg_r.toFixed(2)} sub={`expectancy ${analysis.expectancy_r.toFixed(2)} R`} />
        <StatCard title="Итог net" value={fmtUsd(analysis.net)} warn={analysis.net < 0} />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Проскальзывание vs тень</CardTitle>
          <CardDescription>Медиана и p90 по закрытым Live (б.п., больше — хуже для Live).</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <SlipCallout
            title="Вход"
            hint="Разница цены входа Live и тени в базисных пунктах."
            stats={analysis.entry_slip}
          />
          <SlipCallout
            title="Выход"
            hint="Разница цены выхода Live и тени."
            stats={analysis.exit_slip}
          />
          <SlipCallout
            title="Сторона (вход+выход)"
            hint="Суммарное проскальзывание по сделке — ключ для слоя B."
            stats={analysis.side_slip}
          />
          <SlipCallout
            title="Выход по стопу"
            hint="Только исход «стоп» — где проскальзывание особенно больно."
            stats={analysis.sl_exit_slip}
          />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>PnL по дням</CardTitle>
          <CardDescription>Закрытые Live-сделки, сумма net по дате выхода (UTC).</CardDescription>
        </CardHeader>
        <CardContent>
          {dailyChart.length === 0 ? (
            <QuietEmpty title="Пока пусто" text="Появится после первых закрытых Live." />
          ) : (
            <ChartContainer config={dailyPnlConfig} className="aspect-auto h-56 w-full">
              <BarChart accessibilityLayer data={dailyChart}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="date" tickLine={false} axisLine={false} minTickGap={16} />
                <YAxis tickLine={false} axisLine={false} width={56} tickFormatter={(v) => fmtUsdCompact(Number(v))} />
                <ChartTooltip
                  content={
                    <ChartTooltipContent
                      labelFormatter={(_, payload) => {
                        const p = payload?.[0]?.payload as { full?: string } | undefined;
                        return p?.full ?? "";
                      }}
                    />
                  }
                />
                <Bar dataKey="net" fill="var(--color-net)" radius={4} />
              </BarChart>
            </ChartContainer>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Выбросы p90 (сторона)</CardTitle>
          <CardDescription>
            Сделки с проскальзыванием ≥ p90 ({fmtBps(analysis.side_slip.p90_bps)} при n={analysis.side_slip.n}).
          </CardDescription>
        </CardHeader>
        <CardContent>
          {outliers.length === 0 ? (
            <QuietEmpty
              title="Выбросов нет"
              text={analysis.side_slip.n ? "Ни одна сделка не превысила p90." : "Мало данных для p90."}
            />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Выход</TableHead>
                  <TableHead>Сделка</TableHead>
                  <TableHead>Исход</TableHead>
                  <TableHead className="text-right">Сторона</TableHead>
                  <TableHead className="text-right">Вход</TableHead>
                  <TableHead className="text-right">Выход</TableHead>
                  <TableHead className="text-right">Net</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {outliers.map((t) => (
                  <TableRow key={t.id}>
                    <TableCell>{fmtTime(t.exit_time)}</TableCell>
                    <TableCell>
                      {t.symbol} {sideLabel(t.direction)}
                    </TableCell>
                    <TableCell>{outcomeLabel(t.outcome)}</TableCell>
                    <TableCell className="text-right tabular-nums">{fmtBps(t.side_slip_bps ?? NaN)}</TableCell>
                    <TableCell className="text-right tabular-nums text-muted-foreground">{fmtBps(t.entry_slip_bps ?? NaN)}</TableCell>
                    <TableCell className="text-right tabular-nums text-muted-foreground">{fmtBps(t.exit_slip_bps ?? NaN)}</TableCell>
                    <TableCell className={cn("text-right tabular-nums", t.net < 0 && "text-destructive")}>{fmtUsd(t.net)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Контекст</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2">
          <Meta k="Дней с go-live" v={String(analysis.days_since_go_live)} />
          <Meta k="Старт Live" v={fmtTime(analysis.go_live_ts)} />
          <Meta k="Точек кривой" v={String(analysis.curve_points)} />
          <Meta k="Match rate LTD" v={`${(analysis.match_rate_ltd * 100).toFixed(1)}%`} />
        </CardContent>
      </Card>
    </div>
  );
}
