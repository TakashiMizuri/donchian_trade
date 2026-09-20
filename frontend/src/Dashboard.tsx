import { useEffect, useMemo, useState } from "react";
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts";
import { ActivityIcon, InfoIcon, LogOutIcon } from "lucide-react";
import { toast } from "sonner";
import {
  api,
  CurvePoint,
  CashFlow,
  DailyReport,
  EventRow,
  fmtBps,
  fmtPct,
  fmtPx,
  fmtQty,
  fmtTime,
  fmtUsd,
  fmtUsdCompact,
  fmtUptime,
  Mismatch,
  Stats,
  Status,
  SymbolSnap,
  Trade,
} from "./api";
import {
  cashLabel,
  checkLabel,
  eventKindLabel,
  eventLevelLabel,
  layerMeta,
  mismatchLabel,
  networkLabel,
  outcomeLabel,
  profileLabel,
  sideLabel,
  strategyLabel,
  verdictCopy,
  verdictLabel,
} from "./labels";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardAction, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartLegend, ChartLegendContent, ChartTooltip, ChartTooltipContent } from "@/components/ui/chart";
import { Empty, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { Field, FieldTitle } from "@/components/ui/field";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

type Range = "24h" | "7d" | "30d" | "90d" | "ltd";
type BookFilter = "all" | "live" | "shadow_ls5" | "shadow_baseline";

const RANGE_LABEL: Record<Range, string> = {
  "24h": "сутки",
  "7d": "неделя",
  "30d": "месяц",
  "90d": "90 дней",
  ltd: "всё",
};

const equityConfig = {
  live: { label: "Live", color: "var(--chart-1)" },
  ls5: { label: "Тень", color: "var(--chart-2)" },
  base: { label: "Без паузы", color: "var(--chart-3)" },
  live_xf: { label: "Live без фандинга", color: "var(--chart-4)" },
} satisfies ChartConfig;

const gapConfig = {
  gap: { label: "Live − тень", color: "var(--chart-5)" },
} satisfies ChartConfig;

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState("overview");
  const [status, setStatus] = useState<Status | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [equity, setEquity] = useState<CurvePoint[]>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [events, setEvents] = useState<EventRow[]>([]);
  const [mismatches, setMismatches] = useState<Mismatch[]>([]);
  const [cashFlows, setCashFlows] = useState<CashFlow[]>([]);
  const [daily, setDaily] = useState<DailyReport | null>(null);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  const [range, setRange] = useState<Range>("ltd");
  const [extras, setExtras] = useState<string[]>([]);
  const [symFilter, setSymFilter] = useState("ALL");
  const [bookFilter, setBookFilter] = useState<BookFilter>("all");

  async function refresh() {
    try {
      const [s, t, e, st, ev, mm, cash, rep] = await Promise.all([
        api.status(),
        api.trades(),
        api.equity(),
        api.stats(),
        api.events(),
        api.mismatches(),
        api.cash().catch(() => ({ cash_base: 0, flows: [] as CashFlow[] })),
        api.report().catch(() => ({ date_utc: "", body: "", verdict: "" })),
      ]);
      setStatus(s);
      setTrades(Array.isArray(t) ? t : []);
      setEquity(Array.isArray(e) ? e : []);
      setStats(st);
      setEvents(Array.isArray(ev) ? ev : []);
      setMismatches(Array.isArray(mm) ? mm : []);
      setCashFlows(Array.isArray(cash.flows) ? cash.flows : []);
      setDaily(rep.date_utc ? rep : null);
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

  const chart = useMemo(() => buildChart(equity, range), [equity, range]);

  const liveTrades = useMemo(() => trades.filter((t) => t.profile === "live"), [trades]);
  const openLive = useMemo(
    () => liveTrades.filter((t) => !t.outcome || t.outcome === "open"),
    [liveTrades],
  );
  const closedLive = useMemo(
    () => liveTrades.filter((t) => t.outcome && t.outcome !== "open"),
    [liveTrades],
  );
  const overviewLive = useMemo(
    () => [...openLive, ...closedLive].slice(0, 12),
    [openLive, closedLive],
  );
  const shownTrades = useMemo(() => {
    const rows = trades
      .filter((t) => (bookFilter === "all" ? true : t.profile === bookFilter))
      .filter((t) => (symFilter === "ALL" ? true : t.symbol === symFilter));
    return [...rows].sort((a, b) => {
      const ao = !a.outcome || a.outcome === "open" ? 0 : 1;
      const bo = !b.outcome || b.outcome === "open" ? 0 : 1;
      if (ao !== bo) return ao - bo;
      return (b.entry_time || b.signal_time) - (a.entry_time || a.signal_time);
    });
  }, [trades, bookFilter, symFilter]);

  async function kill() {
    setBusy(true);
    try {
      await api.kill();
      await refresh();
      toast("Аварийный стоп включён. Live-позиции закрыты.");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setBusy(false);
    }
  }

  async function resume() {
    setBusy(true);
    try {
      await api.resume();
      await refresh();
      toast("Аварийный стоп снят. Новые входы снова разрешены.");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setBusy(false);
    }
  }

  const r = status?.report;
  const activeVerdict = status?.kill_switch ? "STOP" : status?.verdict || "WAIT";
  const verdict = verdictCopy(activeVerdict);
  const matched = r?.n_matched ?? 0;
  const union = r?.n_union ?? 0;
  const ratio = r?.pnl_ratio_ok ? r.pnl_ratio_xf.toFixed(2) : "рано";
  const symbols = status?.symbols ?? [];
  const showBase = extras.includes("base");
  const showXF = extras.includes("xf");
  const hasChart = chart.length > 1;

  return (
    <div className="mx-auto flex min-h-dvh max-w-5xl flex-col">
      <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b bg-background/90 px-4 py-3 backdrop-blur">
        <div className="min-w-0">
          <div className="font-heading text-sm font-medium">Donchian</div>
          <div className="truncate text-xs text-muted-foreground">
            {status
              ? `${networkLabel(status.network, status.dry_run)} · ${strategyLabel(status.strategy)}`
              : "загрузка…"}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {status ? <VerdictBadge verdict={activeVerdict} /> : null}
          <Badge variant={status?.ws_connected ? "secondary" : "destructive"}>
            {status?.ws_connected ? "стрим" : "нет стрима"}
          </Badge>
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="ghost"
                size="icon-sm"
                type="button"
                aria-label="Выйти"
                onClick={() => {
                  void api.logout();
                  onLogout();
                }}
              >
                <LogOutIcon />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Выйти</TooltipContent>
          </Tooltip>
        </div>
      </header>

      <div className="flex flex-1 flex-col gap-4 p-4">
        {err ? (
          <Alert variant="destructive">
            <AlertTitle>Нет связи с ботом</AlertTitle>
            <AlertDescription>{err}</AlertDescription>
          </Alert>
        ) : null}

        {!status ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-24" />
            <Skeleton className="h-72 sm:col-span-2 lg:col-span-4" />
          </div>
        ) : (
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList className="w-full sm:w-fit">
              <TabsTrigger value="overview">Обзор</TabsTrigger>
              <TabsTrigger value="trades">Сделки</TabsTrigger>
              <TabsTrigger value="system">Система</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="flex flex-col gap-4">
              <Alert variant={activeVerdict === "STOP" ? "destructive" : "default"}>
                <AlertTitle>{verdict.title}</AlertTitle>
                <AlertDescription>{verdict.body}</AlertDescription>
              </Alert>

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  title="Live"
                  hint="Реальный счёт на бирже."
                  value={fmtUsd(status.equity)}
                  sub={`просадка ${r ? fmtPct(r.dd_live) : "—"}`}
                />
                <StatCard
                  title="Тень"
                  hint="Тот же сигнал без ордеров. После пяти убытков подряд — пауза, как у Live."
                  value={fmtUsd(status.equity_shadow_ls5)}
                  sub={`просадка ${r ? fmtPct(r.dd_shadow_ls5) : "—"}`}
                />
                <StatCard
                  title="Разрыв"
                  hint="Live без фандинга минус тень. Большой минус значит, что Live отстаёт от сигнала."
                  value={fmtUsd(status.gap_usd)}
                  sub={fmtPct(status.gap_pct)}
                  warn={status.gap_usd < -50}
                />
                <StatCard
                  title="Совпадения"
                  hint="Сколько входов Live и тени совпали по часу и стороне."
                  value={union ? `${matched} из ${union}` : "пока нет"}
                  sub={union ? `за неделю ${pct3(status.match_rate_7d)}` : "нужны закрытые сделки"}
                />
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>Эквити</CardTitle>
                  <CardDescription>Live и тень на одних свечах. Линии должны идти рядом.</CardDescription>
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
                        <ToggleGroupItem value="base">тень без паузы</ToggleGroupItem>
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
                        <YAxis
                          tickLine={false}
                          axisLine={false}
                          width={56}
                          tickFormatter={(v) => fmtUsdCompact(Number(v))}
                        />
                        <ChartTooltip content={<ChartTooltipContent indicator="line" />} />
                        <ChartLegend content={<ChartLegendContent />} />
                        <Line type="monotone" dataKey="live" stroke="var(--color-live)" strokeWidth={2} dot={false} />
                        <Line type="monotone" dataKey="ls5" stroke="var(--color-ls5)" strokeWidth={1.6} dot={false} />
                        {showBase ? (
                          <Line
                            type="monotone"
                            dataKey="base"
                            stroke="var(--color-base)"
                            strokeWidth={1.2}
                            strokeDasharray="4 4"
                            dot={false}
                          />
                        ) : null}
                        {showXF ? (
                          <Line
                            type="monotone"
                            dataKey="live_xf"
                            stroke="var(--color-live_xf)"
                            strokeWidth={1.2}
                            dot={false}
                          />
                        ) : null}
                      </LineChart>
                    </ChartContainer>
                    </>
                  ) : (
                    <QuietEmpty
                      title="Кривой ещё нет"
                      text="Появится после первой закрытой часовой свечи."
                    />
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Разрыв</CardTitle>
                  <CardDescription>На сколько Live без фандинга отстаёт от тени. Около нуля — хорошо.</CardDescription>
                </CardHeader>
                <CardContent>
                  {hasChart ? (
                    <ChartContainer key={`gap-${range}`} config={gapConfig} className="aspect-auto h-36 w-full">
                      <LineChart accessibilityLayer data={chart}>
                        <CartesianGrid vertical={false} />
                        <XAxis dataKey="t" tickLine={false} axisLine={false} minTickGap={28} tickMargin={8} />
                        <YAxis
                          tickLine={false}
                          axisLine={false}
                          width={56}
                          tickFormatter={(v) => fmtUsdCompact(Number(v))}
                        />
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
                  <Meta k="Фандинг" v={fmtUsd(status.funding)} hint="Плата за удержание позиции. В сравнении с тенью Live смотрим без неё." />
                  <Meta k="За день" v={fmtUsd(status.daily_pnl)} hint="PnL за текущие сутки. Красный день сам по себе ничего не стопает." />
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
                      <Meta k="Доля плюсовых" v={fmtPct(stats.win_rate)} hint="Доля прибыльных закрытых Live. У Donchian обычно низкая." />
                      <Meta k="Сделок Live" v={String(stats.trades)} hint={`${stats.wins} плюс / ${stats.losses} минус`} />
                    </>
                  ) : null}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Рынки</CardTitle>
                  <CardDescription>Позиции Live и тени по каждому инструменту.</CardDescription>
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
                          <TableHead>Объём</TableHead>
                          <TableHead>Вход</TableHead>
                          <TableHead>Стоп</TableHead>
                          <TableHead>Убытки</TableHead>
                          <TableHead>Статус</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {symbols.map((s) => (
                          <TableRow key={s.symbol}>
                            <TableCell className="font-medium">{s.symbol}</TableCell>
                            <TableCell>{fmtPx(s.last_price)}</TableCell>
                            <TableCell>{sideLabel(s.position)}</TableCell>
                            <TableCell>{sideLabel(s.shadow_ls5)}</TableCell>
                            <TableCell className="tabular-nums">{fmtQty(s.qty)}</TableCell>
                            <TableCell>{fmtPx(s.entry)}</TableCell>
                            <TableCell>{fmtPx(s.stop)}</TableCell>
                            <TableCell className="tabular-nums">
                              {s.consec_losses} / {s.shadow_consec_ls5}
                            </TableCell>
                            <TableCell>
                              <Badge variant={s.paused ? "outline" : "secondary"}>{pauseLabel(s)}</Badge>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>История Live</CardTitle>
                  <CardDescription>Открытые сейчас и последние закрытые. Полный журнал — во вкладке «Сделки».</CardDescription>
                </CardHeader>
                <CardContent>
                  {overviewLive.length === 0 ? (
                    <QuietEmpty title="Сделок Live ещё нет" text="Donchian на часе входит редко. Пусто в первые сутки — нормально." />
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
                            <TableCell className={cn("text-right tabular-nums", t.net < 0 && "text-destructive")}>
                              {fmtUsd(t.net)}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Расхождения</CardTitle>
                  <CardDescription>Вход, который был только у Live или только у тени.</CardDescription>
                </CardHeader>
                <CardContent>
                  {mismatches.length === 0 ? (
                    <QuietEmpty title="Расхождений нет" text="Так и должно быть, если Live повторяет тень." />
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Что случилось</TableHead>
                          <TableHead>Рынок</TableHead>
                          <TableHead>Когда</TableHead>
                          <TableHead>Заметка</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {mismatches.slice(0, 12).map((m, i) => (
                          <TableRow key={`${m.signal_time}-${i}`}>
                            <TableCell>
                              {mismatchLabel(m.kind)} · {sideLabel(m.direction)}
                            </TableCell>
                            <TableCell>{m.symbol}</TableCell>
                            <TableCell>{fmtTime(m.signal_time)}</TableCell>
                            <TableCell className="max-w-56 truncate text-muted-foreground">{m.note || "—"}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Аварийный стоп</CardTitle>
                  <CardDescription>Только руками, если сломалось исполнение.</CardDescription>
                </CardHeader>
                <CardFooter className="justify-end">
                  {status.kill_switch ? (
                    <Button disabled={busy} onClick={() => void resume()}>
                      {busy ? <Spinner data-icon="inline-start" /> : null}
                      Снять стоп
                    </Button>
                  ) : (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" disabled={busy}>
                          Закрыть позиции и остановить входы
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Закрыть Live и остановить входы?</AlertDialogTitle>
                          <AlertDialogDescription>
                            Все позиции на бирже закроются. Новые входы не пойдут. Тень продолжит считать сигналы.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Отмена</AlertDialogCancel>
                          <AlertDialogAction variant="destructive" onClick={() => void kill()}>
                            Да, остановить
                          </AlertDialogAction>
                        </AlertDialogFooter>
                      </AlertDialogContent>
                    </AlertDialog>
                  )}
                </CardFooter>
              </Card>
            </TabsContent>

            <TabsContent value="trades" className="flex flex-col gap-4">
              {stats ? (
                <div className="grid gap-3 sm:grid-cols-4">
                  <StatCard title="Сделок Live" value={String(stats.trades)} sub={`${stats.wins} плюс / ${stats.losses} минус`} />
                  <StatCard title="Итог Live" value={fmtUsd(stats.net)} warn={stats.net < 0} />
                  <StatCard title="Профит-фактор" value={stats.profit_factor.toFixed(2)} />
                  <StatCard title="Просадка" value={fmtPct(stats.max_dd)} />
                </div>
              ) : null}

              <Card>
                <CardHeader>
                  <CardTitle>Журнал</CardTitle>
                  <CardDescription>Live — биржа. Тень — тот же сигнал на бумаге. «Без паузы» — тень без паузы после убытков. Открытые сверху.</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-3">
                  <div className="flex flex-wrap gap-4">
                    <Field>
                      <FieldTitle id="book-label">Счёт</FieldTitle>
                      <ToggleGroup
                        type="single"
                        size="sm"
                        variant="outline"
                        value={bookFilter}
                        aria-labelledby="book-label"
                        onValueChange={(v) => {
                          if (v) setBookFilter(v as BookFilter);
                        }}
                      >
                        <ToggleGroupItem value="all">все</ToggleGroupItem>
                        <ToggleGroupItem value="live">Live</ToggleGroupItem>
                        <ToggleGroupItem value="shadow_ls5">тень</ToggleGroupItem>
                        <ToggleGroupItem value="shadow_baseline">без паузы</ToggleGroupItem>
                      </ToggleGroup>
                    </Field>
                    <Field>
                      <FieldTitle id="trade-sym-label">Рынок</FieldTitle>
                      <ToggleGroup
                        type="single"
                        size="sm"
                        variant="outline"
                        value={symFilter}
                        aria-labelledby="trade-sym-label"
                        onValueChange={(v) => {
                          if (v) setSymFilter(v);
                        }}
                      >
                        <ToggleGroupItem value="ALL">все</ToggleGroupItem>
                        {symbols.map((s) => (
                          <ToggleGroupItem key={s.symbol} value={s.symbol}>
                            {s.symbol}
                          </ToggleGroupItem>
                        ))}
                      </ToggleGroup>
                    </Field>
                  </div>
                  {shownTrades.length === 0 ? (
                    <QuietEmpty title="Сделок пока нет" text="Donchian на часе входит редко. Пустой журнал в первые сутки — нормально." />
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Когда</TableHead>
                          <TableHead>Счёт</TableHead>
                          <TableHead>Сделка</TableHead>
                          <TableHead>Статус</TableHead>
                          <TableHead>Вход → выход</TableHead>
                          <TableHead>Стоп</TableHead>
                          <TableHead>Объём</TableHead>
                          <TableHead>Тень</TableHead>
                          <TableHead className="text-right">Итог</TableHead>
                          <TableHead className="text-right">Фандинг</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {shownTrades.map((t) => (
                          <TableRow key={`${t.profile}-${t.id}`}>
                            <TableCell>{fmtTime(t.entry_time || t.signal_time)}</TableCell>
                            <TableCell>{profileLabel(t.profile)}</TableCell>
                            <TableCell>
                              {t.symbol} {sideLabel(t.direction)}
                            </TableCell>
                            <TableCell>
                              <Badge variant={!t.outcome || t.outcome === "open" ? "secondary" : "outline"}>
                                {outcomeLabel(t.outcome)}
                              </Badge>
                            </TableCell>
                            <TableCell>{pricePath(t)}</TableCell>
                            <TableCell>{fmtPx(t.stop)}</TableCell>
                            <TableCell className="tabular-nums">{fmtQty(t.quantity)}</TableCell>
                            <TableCell className="text-muted-foreground">
                              {t.entry_px_shadow ? fmtPx(t.entry_px_shadow) : "—"}
                              {t.exit_px_shadow ? ` → ${fmtPx(t.exit_px_shadow)}` : ""}
                            </TableCell>
                            <TableCell className={cn("text-right tabular-nums", t.net < 0 && "text-destructive")}>
                              {fmtUsd(t.net)}
                            </TableCell>
                            <TableCell className="text-right tabular-nums text-muted-foreground">
                              {t.funding ? fmtUsd(t.funding) : "—"}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
                {liveTrades.length > 0 ? (
                  <CardFooter>
                    <p className="text-xs text-muted-foreground">
                      {closedLive.length} закрытых Live из {liveTrades.length}
                    </p>
                  </CardFooter>
                ) : null}
              </Card>
            </TabsContent>

            <TabsContent value="system" className="flex flex-col gap-4">
              <Card>
                <CardHeader>
                  <CardTitle>Состояние бота</CardTitle>
                  <CardDescription>Вердикт считается по совпадению с тенью, не по знаку дня.</CardDescription>
                </CardHeader>
                <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
                  <Meta k="Сеть" v={networkLabel(status.network, status.dry_run)} />
                  <Meta k="Аварийный стоп" v={status.kill_switch ? "включён" : "выкл"} />
                  <Meta k="Вердикт" v={verdictLabel(status.verdict)} />
                  <Meta
                    k="Входы / исполнение / PnL"
                    v={`${checkLabel(status.status_a)} · ${checkLabel(status.status_b)} · ${checkLabel(status.status_c)}`}
                  />
                  <Meta k="Стрим" v={status.ws_connected ? "подключён" : status.ws_error || "нет, REST ещё торгует"} />
                  <Meta k="Аптайм" v={fmtUptime(status.uptime_sec)} />
                  <Meta k="Старт Live" v={fmtTime(status.go_live_ts)} />
                  <Meta k="Свободно" v={fmtUsd(status.available)} />
                  <Meta k="Последняя ошибка" v={status.last_error || "нет"} />
                </CardContent>
              </Card>

              {daily?.date_utc ? (
                <Card>
                  <CardHeader>
                    <CardTitle>Последний суточный отчёт</CardTitle>
                    <CardDescription>
                      {daily.date_utc} UTC · {verdictLabel(daily.verdict)}. Тот же текст уходит в Telegram.
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <pre className="max-h-80 overflow-auto rounded-lg bg-muted/40 p-3 whitespace-pre-wrap text-xs text-muted-foreground">
                      {daily.body}
                    </pre>
                  </CardContent>
                </Card>
              ) : null}

              <Card>
                <CardHeader>
                  <CardTitle>Касса</CardTitle>
                  <CardDescription>Пополнения и выводы по кошельку. Тень копирует те же суммы.</CardDescription>
                </CardHeader>
                <CardContent>
                  {cashFlows.length === 0 ? (
                    <QuietEmpty title="Записей нет" text="Появятся после первого снимка счёта или перевода." />
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Когда</TableHead>
                          <TableHead>Тип</TableHead>
                          <TableHead className="text-right">Сумма</TableHead>
                          <TableHead>Заметка</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {cashFlows.map((f) => (
                          <TableRow key={f.id}>
                            <TableCell>{fmtTime(f.ts)}</TableCell>
                            <TableCell>{cashLabel(f.kind)}</TableCell>
                            <TableCell className={cn("text-right tabular-nums", f.amount < 0 && "text-destructive")}>
                              {fmtUsd(f.amount)}
                            </TableCell>
                            <TableCell className="max-w-64 truncate text-muted-foreground">{f.note || "—"}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>События</CardTitle>
                </CardHeader>
                <CardContent>
                  {events.length === 0 ? (
                    <QuietEmpty title="Журнал пуст" text="Сюда попадут входы, ошибки и системные сообщения." />
                  ) : (
                    <div className="flex flex-col gap-3">
                      {events.slice(0, 40).map((e, i) => (
                        <div key={`${e.ts}-${i}`} className="flex flex-col gap-1">
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <span>{fmtTime(e.ts)}</span>
                            <Badge variant={e.level === "error" ? "destructive" : "outline"}>
                              {eventLevelLabel(e.level)}
                              {e.kind ? ` · ${eventKindLabel(e.kind)}` : ""}
                            </Badge>
                          </div>
                          <p className="whitespace-pre-wrap text-sm">{e.message}</p>
                          {i < Math.min(events.length, 40) - 1 ? <Separator /> : null}
                        </div>
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            </TabsContent>
          </Tabs>
        )}
      </div>
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

function buildChart(equity: CurvePoint[], range: Range) {
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

function pct3(n: number) {
  if (!n && n !== 0) return "—";
  return `${(n * 100).toFixed(1)}%`;
}

function pricePath(t: Trade) {
  const entry = fmtPx(t.entry_px_live || t.entry_price);
  const exit = t.exit_px_live || t.exit_price;
  return exit ? `${entry} → ${fmtPx(exit)}` : `${entry} → …`;
}

function pauseLabel(s: SymbolSnap) {
  if (!s.paused) return "активен";
  if (s.pause_until_time) return `пауза до ${fmtTime(s.pause_until_time)}`;
  return "пауза";
}

function checkVariant(v: string): "default" | "secondary" | "destructive" | "outline" {
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

function VerdictBadge({ verdict }: { verdict: string }) {
  const variant = verdict === "STOP" ? "destructive" : verdict === "INVESTIGATE" ? "outline" : "secondary";
  return <Badge variant={variant}>{verdictLabel(verdict)}</Badge>;
}

function Hint({ text }: { text: string }) {
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <Button type="button" variant="ghost" size="icon-xs" aria-label="Пояснение">
          <InfoIcon />
        </Button>
      </TooltipTrigger>
      <TooltipContent>{text}</TooltipContent>
    </Tooltip>
  );
}

function QuietEmpty({ title, text }: { title: string; text: string }) {
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

function StatCard({
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

function CheckCard({ id, value, detail }: { id: "a" | "b" | "c"; value: string; detail: string }) {
  const meta = layerMeta(id);
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
        <p className="text-sm text-muted-foreground">{detail}</p>
      </CardContent>
    </Card>
  );
}

function Meta({ k, v, hint }: { k: string; v: string; hint?: string }) {
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
