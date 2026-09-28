import { useMemo, useState } from "react";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";
import {
  CurvePoint,
  Mismatch,
  Stats,
  Status,
  Trade,
  fmtBps,
  fmtPct,
  fmtPx,
  fmtQty,
  fmtTime,
  fmtUsd,
  fmtUptime,
} from "../api";
import { cashLabel, outcomeLabel, sideLabel, verdictCopy } from "../labels";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { Field, FieldTitle } from "@/components/ui/field";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { cn } from "@/lib/utils";
import {
  CheckCard,
  Meta,
  QuietEmpty,
  Range,
  RANGE_LABEL,
  StatCard,
  buildChart,
  equityConfig,
  gapConfig,
  pct3,
  pricePath,
  shadowEquity,
  fmtUsdCompact,
} from "./shared";

type Props = {
  status: Status;
  equity: CurvePoint[];
  stats: Stats | null;
  trades: Trade[];
  mismatches: Mismatch[];
};

export function OverviewTab({ status, equity, stats, trades, mismatches }: Props) {
  const [range, setRange] = useState<Range>("ltd");
  const [extras, setExtras] = useState<string[]>([]);

  const r = status.report;
  const activeVerdict = status.kill_switch ? "STOP" : status.verdict || "WAIT";
  const verdict = verdictCopy(activeVerdict);
  const matched = r?.n_matched ?? 0;
  const union = r?.n_union ?? 0;
  const ratio = r?.pnl_ratio_ok ? r.pnl_ratio_xf.toFixed(2) : "рано";
  const symbols = status.symbols ?? [];
  const showBase = extras.includes("base");
  const showXF = extras.includes("xf");
  const chart = useMemo(() => buildChart(equity, range), [equity, range]);
  const hasChart = chart.length > 1;
  const shadowEq = shadowEquity(status);

  const liveTrades = useMemo(() => trades.filter((t) => t.profile === "live"), [trades]);
  const overviewLive = useMemo(() => {
    const open = liveTrades.filter((t) => !t.outcome || t.outcome === "open");
    const closed = liveTrades.filter((t) => t.outcome && t.outcome !== "open");
    return [...open, ...closed].slice(0, 8);
  }, [liveTrades]);

  const todayNotes = buildTodayNotes(status, mismatches, activeVerdict);

  return (
    <div className="flex flex-col gap-4">
      <Alert variant={activeVerdict === "STOP" ? "destructive" : "default"}>
        <AlertTitle>{verdict.title}</AlertTitle>
        <AlertDescription>{verdict.body}</AlertDescription>
      </Alert>

      <Card>
        <CardHeader>
          <CardTitle>Что важно сегодня</CardTitle>
          <CardDescription>Коротко: связь, вердикт и день.</CardDescription>
        </CardHeader>
        <CardContent>
          <ul className="list-disc space-y-1 pl-4 text-sm text-muted-foreground">
            {todayNotes.map((line, i) => (
              <li key={i}>{line}</li>
            ))}
          </ul>
        </CardContent>
      </Card>

      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          title="Live"
          hint="Реальный счёт на бирже."
          value={fmtUsd(status.equity)}
          sub={`просадка ${r ? fmtPct(r.dd_live) : "—"}`}
        />
        <StatCard
          title="Тень (фильтры)"
          hint="Тот же сигнал на бумаге с теми же фильтрами, что у Live."
          value={fmtUsd(shadowEq)}
          sub={`просадка ${r ? fmtPct(r.dd_shadow_ls5) : "—"}`}
        />
        <StatCard
          title="Baseline"
          hint="Тень без фильтров vol rank — эталон «сырого» канала."
          value={fmtUsd(status.equity_shadow_baseline)}
          sub="для сравнения с фильтрованной тенью"
        />
        <StatCard
          title="Разрыв"
          hint="Live без фандинга минус тень (фильтры). Около нуля — хорошо."
          value={fmtUsd(status.gap_usd)}
          sub={fmtPct(status.gap_pct)}
          warn={status.gap_usd < -50}
        />
      </div>

      <div className="grid gap-3 sm:grid-cols-2">
        <StatCard
          title="Совпадения входов"
          hint="Сколько входов Live и тени совпали по часу и стороне."
          value={union ? `${matched} из ${union}` : "пока нет"}
          sub={union ? `за неделю ${pct3(status.match_rate_7d)}` : "нужны закрытые сделки"}
        />
        <StatCard
          title="Аптайм · стрим"
          hint="Время работы процесса и WebSocket к бирже."
          value={fmtUptime(status.uptime_sec)}
          sub={status.ws_connected ? "WebSocket подключён" : status.ws_error || "нет WebSocket"}
          warn={!status.ws_connected}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Эквити</CardTitle>
          <CardDescription>Live, тень (фильтры) и опционально Baseline на одних свечах.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <div className="flex flex-wrap gap-4">
            <Field>
              <FieldTitle id="range-label">Период</FieldTitle>
              <ToggleGroup
                type="single"
                size="sm"
                variant="outline"
                spacing={0}
                value={range}
                aria-labelledby="range-label"
                onValueChange={(v) => {
                  if (v) setRange(v as Range);
                }}
              >
                {(Object.keys(RANGE_LABEL) as Range[]).map((id) => (
                  <ToggleGroupItem key={id} value={id}>
                    {RANGE_LABEL[id]}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Field>
            <Field>
              <FieldTitle id="extra-label">Дополнительно</FieldTitle>
              <ToggleGroup
                type="multiple"
                size="sm"
                variant="outline"
                value={extras}
                aria-labelledby="extra-label"
                onValueChange={(v) => setExtras(v ?? [])}
              >
                <ToggleGroupItem value="base">Baseline</ToggleGroupItem>
                <ToggleGroupItem value="xf">без фандинга</ToggleGroupItem>
              </ToggleGroup>
            </Field>
          </div>
          {hasChart ? (
            <>
              <p className="text-xs text-muted-foreground">
                {RANGE_LABEL[range]} · {chart.length} точек
              </p>
              <ChartContainer key={`eq-${range}-${showBase}-${showXF}`} config={equityConfig} className="aspect-auto h-64 w-full">
                <LineChart accessibilityLayer data={chart}>
                  <CartesianGrid vertical={false} />
                  <XAxis dataKey="t" tickLine={false} axisLine={false} minTickGap={28} tickMargin={8} />
                  <YAxis tickLine={false} axisLine={false} width={56} tickFormatter={(v) => fmtUsdCompact(Number(v))} />
                  <ChartTooltip content={<ChartTooltipContent indicator="line" />} />
                  <ChartLegend content={<ChartLegendContent />} />
                  <Line type="monotone" dataKey="live" stroke="var(--color-live)" strokeWidth={2} dot={false} />
                  <Line type="monotone" dataKey="ls5" stroke="var(--color-ls5)" strokeWidth={1.6} dot={false} />
                  {showBase ? (
                    <Line type="monotone" dataKey="base" stroke="var(--color-base)" strokeWidth={1.2} strokeDasharray="4 4" dot={false} />
                  ) : null}
                  {showXF ? (
                    <Line type="monotone" dataKey="live_xf" stroke="var(--color-live_xf)" strokeWidth={1.2} dot={false} />
                  ) : null}
                </LineChart>
              </ChartContainer>
            </>
          ) : (
            <QuietEmpty title="Кривой ещё нет" text="Появится после первой закрытой часовой свечи." />
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Разрыв</CardTitle>
          <CardDescription>На сколько Live без фандинга отстаёт от тени (фильтры).</CardDescription>
        </CardHeader>
        <CardContent>
          {hasChart ? (
            <ChartContainer key={`gap-${range}`} config={gapConfig} className="aspect-auto h-36 w-full">
              <LineChart accessibilityLayer data={chart}>
                <CartesianGrid vertical={false} />
                <XAxis dataKey="t" tickLine={false} axisLine={false} minTickGap={28} tickMargin={8} />
                <YAxis tickLine={false} axisLine={false} width={56} tickFormatter={(v) => fmtUsdCompact(Number(v))} />
                <ChartTooltip content={<ChartTooltipContent indicator="line" />} />
                <Line type="monotone" dataKey="gap" stroke="var(--color-gap)" strokeWidth={1.6} dot={false} />
              </LineChart>
            </ChartContainer>
          ) : (
            <QuietEmpty title="Пока пусто" text="Появится вместе с кривой эквити." />
          )}
        </CardContent>
      </Card>

      <div className="grid gap-3 md:grid-cols-3">
        <CheckCard
          id="a"
          value={status.status_a}
          detail={union ? `за всё время ${pct3(status.match_rate_ltd)}` : "пока нет совпавших входов"}
        />
        <CheckCard
          id="b"
          value={status.status_b}
          detail={
            status.status_b === "na"
              ? "нужны закрытые совпавшие сделки"
              : `медиана ${fmtBps(status.side_slip_median_bps)}${r ? ` · p90 ${fmtBps(r.side_slip_p90_bps)}` : ""}`
          }
        />
        <CheckCard
          id="c"
          value={status.status_c}
          detail={r?.pnl_ratio_ok ? `Live / тень = ${ratio}` : "пока рано — мало закрытых сделок"}
        />
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Сводка</CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
          <Meta k="Фандинг" v={fmtUsd(status.funding)} hint="Плата за удержание позиции." />
          <Meta k="За день" v={fmtUsd(status.daily_pnl)} hint="PnL за текущие сутки UTC." />
          <Meta
            k="Только Live / только тень"
            v={`${r?.live_only_7d ?? 0} / ${r?.shadow_only_7d ?? 0} за неделю`}
            hint="Вход только у Live или только у тени за 7 дней."
          />
          <Meta
            k="Касса"
            v={fmtUsd(status.cash_base || status.start_equity)}
            hint={
              status.last_cash_ts
                ? `${cashLabel(status.last_cash_kind)} ${fmtUsd(status.last_cash_amount)} · ${fmtTime(status.last_cash_ts)}`
                : "база тени после первого снимка счёта"
            }
          />
          {stats ? (
            <>
              <Meta k="Доля плюсовых Live" v={fmtPct(stats.win_rate)} hint="По закрытым сделкам Live." />
              <Meta k="Сделок Live" v={String(stats.trades)} hint={`${stats.wins} плюс / ${stats.losses} минус`} />
            </>
          ) : null}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Рынки</CardTitle>
          <CardDescription>Позиции Live и тени по инструменту.</CardDescription>
        </CardHeader>
        <CardContent>
          {symbols.length === 0 ? (
            <QuietEmpty title="Рынков нет" text="Список появится, когда бот поднимет инструменты." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Рынок</TableHead>
                  <TableHead>Цена</TableHead>
                  <TableHead>Live</TableHead>
                  <TableHead>Тень</TableHead>
                  <TableHead>Baseline</TableHead>
                  <TableHead>Объём</TableHead>
                  <TableHead>Вход</TableHead>
                  <TableHead>Стоп</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {symbols.map((s) => (
                  <TableRow key={s.symbol}>
                    <TableCell className="font-medium">{s.symbol}</TableCell>
                    <TableCell>{fmtPx(s.last_price)}</TableCell>
                    <TableCell>{sideLabel(s.position)}</TableCell>
                    <TableCell>{sideLabel(s.shadow_ls5)}</TableCell>
                    <TableCell>{sideLabel(s.shadow_baseline)}</TableCell>
                    <TableCell className="tabular-nums">{fmtQty(s.qty)}</TableCell>
                    <TableCell>{fmtPx(s.entry)}</TableCell>
                    <TableCell>{fmtPx(s.stop)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Последние Live</CardTitle>
          <CardDescription>Открытые и недавно закрытые. Полный журнал — «Сделки».</CardDescription>
        </CardHeader>
        <CardContent>
          {overviewLive.length === 0 ? (
            <QuietEmpty title="Сделок Live ещё нет" text="Donchian на часе входит редко." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Когда</TableHead>
                  <TableHead>Сделка</TableHead>
                  <TableHead>Статус</TableHead>
                  <TableHead>Цена</TableHead>
                  <TableHead className="text-right">Итог</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {overviewLive.map((t) => (
                  <TableRow key={`ov-${t.id}`}>
                    <TableCell>{fmtTime(t.entry_time || t.signal_time)}</TableCell>
                    <TableCell>
                      {t.symbol} {sideLabel(t.direction)}
                    </TableCell>
                    <TableCell>
                      <Badge variant={!t.outcome || t.outcome === "open" ? "secondary" : "outline"}>
                        {outcomeLabel(t.outcome)}
                      </Badge>
                    </TableCell>
                    <TableCell>{pricePath(t)}</TableCell>
                    <TableCell className={cn("text-right tabular-nums", t.net < 0 && "text-destructive")}>{fmtUsd(t.net)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>

      {mismatches.length > 0 ? (
        <Card>
          <CardHeader>
            <CardTitle>Расхождения (кратко)</CardTitle>
            <CardDescription>Полный список — вкладка «Здоровье».</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm text-muted-foreground">{mismatches.length} записей — разберите во вкладке «Здоровье».</p>
          </CardContent>
        </Card>
      ) : null}
    </div>
  );
}

function buildTodayNotes(status: Status, mismatches: Mismatch[], verdict: string): string[] {
  const lines: string[] = [];
  if (!status.ws_connected) {
    lines.push(`Нет WebSocket${status.ws_error ? `: ${status.ws_error}` : ""}. Бот может опираться на REST.`);
  } else {
    lines.push("WebSocket в норме.");
  }
  if (status.kill_switch) {
    lines.push("Аварийный стоп включён — новые входы не идут.");
  } else if (verdict === "INVESTIGATE") {
    lines.push("Вердикт «разобрать» — проверьте расхождения и исполнение.");
  } else if (verdict === "WAIT") {
    lines.push("Вердикт ещё не готов — мало данных после старта.");
  } else {
    lines.push("Вердикт: Live и тень сходятся.");
  }
  lines.push(`PnL за сегодня: ${fmtUsd(status.daily_pnl)}.`);
  if (mismatches.length) {
    lines.push(`Расхождений входов в журнале: ${mismatches.length}.`);
  }
  if (status.last_error) {
    lines.push(`Последняя ошибка: ${status.last_error}`);
  }
  return lines;
}
