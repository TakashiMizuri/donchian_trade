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
  fmtTime,
  fmtUsd,
  fmtUsdCompact,
  Mismatch,
  Stats,
  Status,
  Trade,
} from "./api";
import {
  cashLabel,
  checkLabel,
  layerMeta,
  mismatchLabel,
  networkLabel,
  outcomeLabel,
  profileLabel,
  sideLabel,
  strategyLabel,
  verdictCopy,
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
  ls5: { label: "Тень ls5", color: "var(--chart-2)" },
  base: { label: "Без паузы", color: "var(--chart-3)" },
  live_xf: { label: "Live без funding", color: "var(--chart-4)" },
} satisfies ChartConfig;

const gapConfig = {
  gap: { label: "Разрыв Live − тень", color: "var(--chart-5)" },
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

  const chart = useMemo(() => {
    const cutoff = rangeCutoff(range);
    return equity
      .filter((p) => p.ts >= cutoff)
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
  }, [equity, range]);

  const liveTrades = useMemo(() => trades.filter((t) => t.profile === "live"), [trades]);
  const openLive = useMemo(
    () => liveTrades.filter((t) => !t.outcome || t.outcome === "open"),
    [liveTrades],
  );
  const shownTrades = useMemo(() => {
    const rows = trades.filter((t) => (bookFilter === "all" ? true : t.profile === bookFilter)).filter((t) =>
      symFilter === "ALL" ? true : t.symbol === symFilter,
    );
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
      toast("Kill-switch включён. Live-позиции закрыты.");
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
      toast("Kill-switch снят. Новые входы снова разрешены.");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setBusy(false);
    }
  }

  const r = status?.report;
  const verdict = verdictCopy(status?.kill_switch ? "STOP" : status?.verdict || "WAIT");
  const matched = r?.n_matched ?? 0;
  const union = r?.n_union ?? 0;
  const ratio = r?.pnl_ratio_ok ? r.pnl_ratio_xf.toFixed(2) : "рано";
  const symbols = status?.symbols ?? [];
  const showBase = extras.includes("base");
  const showXF = extras.includes("xf");

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
          {status ? <VerdictBadge verdict={status.kill_switch ? "STOP" : status.verdict} /> : null}
          <Badge variant={status?.ws_connected ? "secondary" : "destructive"}>
            {status?.ws_connected ? "стрим ок" : "стрим молчит"}
          </Badge>
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
          <div className="grid gap-3 sm:grid-cols-2">
            <Skeleton className="h-28" />
            <Skeleton className="h-28" />
            <Skeleton className="h-64 sm:col-span-2" />
          </div>
        ) : (
          <Tabs value={tab} onValueChange={setTab}>
            <TabsList className="w-full sm:w-fit">
              <TabsTrigger value="overview">Обзор</TabsTrigger>
              <TabsTrigger value="trades">Сделки</TabsTrigger>
              <TabsTrigger value="system">Система</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="flex flex-col gap-4">
              <Alert>
                <AlertTitle>{verdict.title}</AlertTitle>
                <AlertDescription>{verdict.body}</AlertDescription>
              </Alert>

              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                <StatCard
                  title="Live"
                  hint="Реальный счёт на Lighter."
                  value={fmtUsd(status.equity)}
                  sub={`просадка ${r ? fmtPct(r.dd_live) : "—"}`}
                />
                <StatCard
                  title="Тень ls5"
                  hint="Тот же сигнал, без ордеров. Пауза после 5 убытков подряд — как у Live."
                  value={fmtUsd(status.equity_shadow_ls5)}
                  sub={`просадка ${r ? fmtPct(r.dd_shadow_ls5) : "—"}`}
                />
                <StatCard
                  title="Разрыв"
                  hint="Live без funding минус тень. Большой минус — Live отстаёт от того, что должна давать стратегия."
                  value={fmtUsd(status.gap_usd)}
                  sub={fmtPct(status.gap_pct)}
                  warn={status.gap_usd < -50}
                />
                <StatCard
                  title="Совпало"
                  hint="Сколько входов Live и тени совпали по часу и стороне. 3 из 5 после бага SDK — это не разъезд стратегии."
                  value={union ? `${matched} из ${union}` : "пока нет"}
                  sub={`за неделю ${pct3(status.match_rate_7d)}`}
                />
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>Два счёта на одних свечах</CardTitle>
                  <CardDescription>
                    Жирная линия — реальные деньги. Серая — тень. Они должны идти рядом. День в минусе сам по себе
                    ничего не стопает.
                  </CardDescription>
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
                      <FieldTitle id="extra-label">Линии</FieldTitle>
                      <ToggleGroup
                        type="multiple"
                        size="sm"
                        variant="outline"
                        value={extras}
                        aria-labelledby="extra-label"
                        onValueChange={setExtras}
                      >
                        <ToggleGroupItem value="base">без паузы</ToggleGroupItem>
                        <ToggleGroupItem value="xf">без funding</ToggleGroupItem>
                      </ToggleGroup>
                    </Field>
                    <Field>
                      <FieldTitle id="sym-label">Рынок</FieldTitle>
                      <ToggleGroup
                        type="single"
                        size="sm"
                        variant="outline"
                        value={symFilter}
                        aria-labelledby="sym-label"
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
                  {chart.length > 1 ? (
                    <ChartContainer config={equityConfig} className="aspect-auto h-64 w-full">
                      <LineChart accessibilityLayer data={chart}>
                        <CartesianGrid vertical={false} />
                        <XAxis dataKey="t" tickLine={false} axisLine={false} minTickGap={28} tickMargin={8} />
                        <YAxis
                          tickLine={false}
                          axisLine={false}
                          width={56}
                          tickFormatter={(v) => fmtUsdCompact(Number(v))}
                        />
                        <ChartTooltip
                          content={<ChartTooltipContent indicator="line" />}
                        />
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
                  ) : (
                    <Empty className="border border-dashed">
                      <EmptyHeader>
                        <EmptyMedia variant="icon">
                          <ActivityIcon />
                        </EmptyMedia>
                        <EmptyTitle>Кривой ещё нет</EmptyTitle>
                        <EmptyDescription>
                          Телеметрия пишется после первой закрытой часовой свечи. Пока бот только стартовал — это нормально.
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Разрыв</CardTitle>
                  <CardDescription>На сколько Live без funding отстаёт от тени. Ровная линия около нуля — хорошо.</CardDescription>
                </CardHeader>
                <CardContent>
                  {chart.length > 1 ? (
                    <ChartContainer config={gapConfig} className="aspect-auto h-36 w-full">
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
                    <p className="text-sm text-muted-foreground">Появится вместе с кривой эквити.</p>
                  )}
                </CardContent>
              </Card>

              <div className="grid gap-3 md:grid-cols-3">
                <CheckCard id="a" value={status.status_a} detail={`за всё время ${pct3(status.match_rate_ltd)}`} />
                <CheckCard
                  id="b"
                  value={status.status_b}
                  detail={`медиана ${fmtBps(status.side_slip_median_bps)}${r ? ` · p90 ${fmtBps(r.side_slip_p90_bps)}` : ""}`}
                />
                <CheckCard
                  id="c"
                  value={status.status_c}
                  detail={`Live / тень = ${ratio}${r?.pnl_ratio_ok ? "" : " — тень ещё почти не изменилась"}`}
                />
              </div>

              <Card>
                <CardHeader>
                  <CardTitle>Как читать цифры</CardTitle>
                </CardHeader>
                <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
                  <Meta k="Funding" v={fmtUsd(status.funding)} hint="Плата за удержание. В вердикте Live смотрим без неё." />
                  <Meta k="День" v={fmtUsd(status.daily_pnl)} hint="Не стоп-сигнал. Красный день у Donchian — обычное дело." />
                  <Meta
                    k="Только Live / только тень"
                    v={`${r?.live_only_7d ?? 0} / ${r?.shadow_only_7d ?? 0} за неделю`}
                    hint="Входы, которых не было у пары. После починки SDK смотрите новые, не старые."
                  />
                  <Meta
                    k="Касса тени"
                    v={fmtUsd(status.cash_base || status.start_equity)}
                    hint={
                      status.last_cash_ts
                        ? `${cashLabel(status.last_cash_kind)} ${fmtUsd(status.last_cash_amount)} · ${fmtTime(status.last_cash_ts)}`
                        : "ждём первое пополнение счёта"
                    }
                  />
                  {stats ? (
                    <>
                      <Meta k="Win rate Live" v={fmtPct(stats.win_rate)} hint="Не KPI. Turtle часто около 40%." />
                      <Meta k="Сделок Live" v={String(stats.trades)} hint={`${stats.wins} плюс / ${stats.losses} минус`} />
                    </>
                  ) : null}
                </CardContent>
              </Card>

              {symbols.map((s) => (
                <Card key={s.symbol} size="sm">
                  <CardHeader>
                    <CardTitle>{s.symbol}</CardTitle>
                    <CardDescription>{fmtPx(s.last_price)}</CardDescription>
                    <CardAction>
                      <Badge variant={s.paused ? "outline" : "secondary"}>{s.paused ? "пауза ls5" : "активен"}</Badge>
                    </CardAction>
                  </CardHeader>
                  <CardContent className="grid gap-2 text-sm sm:grid-cols-2">
                    <Meta k="Live" v={sideLabel(s.position)} />
                    <Meta k="Тень ls5" v={sideLabel(s.shadow_ls5)} />
                    <Meta k="Объём" v={s.qty ? String(s.qty) : "—"} />
                    <Meta k="Без паузы" v={sideLabel(s.shadow_baseline)} />
                    <Meta k="Вход" v={fmtPx(s.entry)} />
                    <Meta k="Стоп" v={fmtPx(s.stop)} />
                    <Meta k="Убытки подряд" v={`${s.consec_losses} / ${s.shadow_consec_ls5}`} hint="Live / тень. На 5-м — пауза." />
                    <Meta k="Последняя свеча" v={`${fmtTime(s.last_bar_time)} · ${s.bars} шт.`} />
                  </CardContent>
                </Card>
              ))}

              <Card>
                <CardHeader>
                  <CardTitle>История Live</CardTitle>
                  <CardDescription>
                    Открытые сейчас и последние закрытые. Полный журнал со всеми книгами — во вкладке «Сделки».
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {liveTrades.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Сделок Live ещё нет.</p>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Когда</TableHead>
                          <TableHead>Сделка</TableHead>
                          <TableHead>Статус</TableHead>
                          <TableHead>Ход</TableHead>
                          <TableHead className="text-right">Net</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {[...openLive, ...liveTrades.filter((t) => t.outcome && t.outcome !== "open")].slice(0, 12).map((t) => (
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
                            <TableCell>
                              {fmtPx(t.entry_px_live || t.entry_price)}
                              {t.exit_px_live || t.exit_price ? ` → ${fmtPx(t.exit_px_live || t.exit_price)}` : " → …"}
                            </TableCell>
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
                  <CardDescription>
                    Вход, который был только у Live или только у тени. Старые строки до починки SDK можно не трогать.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {mismatches.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Пока пусто — так и должно быть.</p>
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
                            <TableCell className="max-w-56 truncate text-muted-foreground">{m.note}</TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Kill-switch</CardTitle>
                  <CardDescription>
                    Только руками, если сломалось исполнение. Не из-за дня, недели или win rate.
                  </CardDescription>
                </CardHeader>
                <CardFooter className="justify-end">
                  {status.kill_switch ? (
                    <Button disabled={busy} onClick={() => void resume()}>
                      {busy ? <Spinner data-icon="inline-start" /> : null}
                      Снять kill-switch
                    </Button>
                  ) : (
                    <AlertDialog>
                      <AlertDialogTrigger asChild>
                        <Button variant="destructive" disabled={busy}>
                          Закрыть позиции и стопнуть входы
                        </Button>
                      </AlertDialogTrigger>
                      <AlertDialogContent>
                        <AlertDialogHeader>
                          <AlertDialogTitle>Закрыть Live и включить стоп?</AlertDialogTitle>
                          <AlertDialogDescription>
                            Все позиции на бирже закроются. Новые входы не пойдут. Тень продолжит считать сигналы — её
                            это не выключает.
                          </AlertDialogDescription>
                        </AlertDialogHeader>
                        <AlertDialogFooter>
                          <AlertDialogCancel>Отмена</AlertDialogCancel>
                          <AlertDialogAction variant="destructive" onClick={() => void kill()}>
                            Да, стопнуть
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
                  <StatCard title="Сделок Live" value={String(stats.trades)} sub={`${stats.wins} / ${stats.losses}`} />
                  <StatCard title="Net Live" value={fmtUsd(stats.net)} warn={stats.net < 0} />
                  <StatCard title="Profit factor" value={stats.profit_factor.toFixed(2)} />
                  <StatCard title="Просадка" value={fmtPct(stats.max_dd)} />
                </div>
              ) : null}

              <Card>
                <CardHeader>
                  <CardTitle>Журнал</CardTitle>
                  <CardDescription>
                    Live — биржа. Тень ls5 — тот же сигнал на бумаге. «Без паузы» — эталон без ls5. Открытые сверху.
                  </CardDescription>
                  <CardAction>
                    <ToggleGroup
                      type="single"
                      size="sm"
                      variant="outline"
                      value={bookFilter}
                      onValueChange={(v) => {
                        if (v) setBookFilter(v as BookFilter);
                      }}
                    >
                      <ToggleGroupItem value="all">все</ToggleGroupItem>
                      <ToggleGroupItem value="live">Live</ToggleGroupItem>
                      <ToggleGroupItem value="shadow_ls5">тень</ToggleGroupItem>
                      <ToggleGroupItem value="shadow_baseline">без паузы</ToggleGroupItem>
                    </ToggleGroup>
                  </CardAction>
                </CardHeader>
                <CardContent>
                  {shownTrades.length === 0 ? (
                    <Empty className="border border-dashed">
                      <EmptyHeader>
                        <EmptyTitle>Сделок пока нет</EmptyTitle>
                        <EmptyDescription>
                          Donchian на часе редко входит. Пустой журнал в первые сутки — норма.
                        </EmptyDescription>
                      </EmptyHeader>
                    </Empty>
                  ) : (
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>Когда</TableHead>
                          <TableHead>Книга</TableHead>
                          <TableHead>Сделка</TableHead>
                          <TableHead>Статус</TableHead>
                          <TableHead>Вход → выход</TableHead>
                          <TableHead>Стоп</TableHead>
                          <TableHead>Qty</TableHead>
                          <TableHead>Тень</TableHead>
                          <TableHead className="text-right">Net</TableHead>
                          <TableHead className="text-right">Funding</TableHead>
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
                            <TableCell>
                              {fmtPx(t.entry_px_live || t.entry_price)}
                              {t.exit_px_live || t.exit_price ? ` → ${fmtPx(t.exit_px_live || t.exit_price)}` : " → …"}
                            </TableCell>
                            <TableCell>{fmtPx(t.stop)}</TableCell>
                            <TableCell className="tabular-nums">{t.quantity || "—"}</TableCell>
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
                      {liveTrades.filter((t) => t.outcome && t.outcome !== "open").length} закрытых Live из{" "}
                      {liveTrades.length}
                    </p>
                  </CardFooter>
                ) : null}
              </Card>
            </TabsContent>

            <TabsContent value="system" className="flex flex-col gap-4">
              <Card>
                <CardHeader>
                  <CardTitle>Состояние бота</CardTitle>
                  <CardDescription>
                    Вердикт считается по совпадению с тенью, не по знаку дня. Красная неделя сама по себе — не повод
                    выключать.
                  </CardDescription>
                </CardHeader>
                <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
                  <Meta k="Сеть" v={networkLabel(status.network, status.dry_run)} />
                  <Meta k="Kill-switch" v={status.kill_switch ? "включён" : "выкл"} />
                  <Meta k="Вердикт" v={status.verdict} />
                  <Meta
                    k="Слои A / B / C"
                    v={`${checkLabel(status.status_a)} · ${checkLabel(status.status_b)} · ${checkLabel(status.status_c)}`}
                  />
                  <Meta k="Стрим" v={status.ws_connected ? "подключён" : status.ws_error || "молчит, REST ещё торгует"} />
                  <Meta k="Аптайм" v={`${Math.floor(status.uptime_sec / 60)} мин`} />
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
                      {daily.date_utc} UTC · {daily.verdict}. Тот же текст уходит в Telegram после 23:00 UTC.
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <pre className="max-h-80 overflow-auto whitespace-pre-wrap text-xs text-muted-foreground">
                      {daily.body}
                    </pre>
                  </CardContent>
                </Card>
              ) : null}

              <Card>
                <CardHeader>
                  <CardTitle>Касса</CardTitle>
                  <CardDescription>Пополнения и выводы, которые бот увидел по кошельку. Тень копирует те же суммы.</CardDescription>
                </CardHeader>
                <CardContent>
                  {cashFlows.length === 0 ? (
                    <p className="text-sm text-muted-foreground">Пока нет записей — будет seed с первого equity.</p>
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
                    <p className="text-sm text-muted-foreground">Журнал пуст.</p>
                  ) : (
                    <div className="flex flex-col gap-3">
                      {events.slice(0, 40).map((e, i) => (
                        <div key={`${e.ts}-${i}`} className="flex flex-col gap-1">
                          <div className="flex items-center gap-2 text-xs text-muted-foreground">
                            <span>{fmtTime(e.ts)}</span>
                            <Badge variant={e.level === "error" ? "destructive" : "outline"}>
                              {e.level} · {e.kind}
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

function pct3(n: number) {
  if (!n && n !== 0) return "—";
  return `${(n * 100).toFixed(1)}%`;
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
  const label = verdict === "STOP" ? "стоп" : verdict === "INVESTIGATE" ? "разобрать" : "сходится";
  return <Badge variant={variant}>{label}</Badge>;
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
