import { Fragment, useMemo, useState } from "react";
import { ChevronDownIcon, ChevronRightIcon } from "lucide-react";
import { Stats, Status, Trade, fmtBps, fmtPx, fmtQty, fmtTime, fmtUsd } from "../api";
import { outcomeLabel, profileLabel, sideLabel } from "../labels";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldTitle } from "@/components/ui/field";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { cn } from "@/lib/utils";
import { BookFilter, Meta, QuietEmpty, StatCard, pricePath, totalFees } from "./shared";

type Props = {
  status: Status;
  trades: Trade[];
  stats: Stats | null;
};

export function TradesTab({ status, trades, stats }: Props) {
  const [bookFilter, setBookFilter] = useState<BookFilter>("all");
  const [symFilter, setSymFilter] = useState("ALL");
  const [outcomeFilter, setOutcomeFilter] = useState("all");
  const [expanded, setExpanded] = useState<number | null>(null);

  const symbols = status.symbols ?? [];

  const outcomes = useMemo(() => {
    const set = new Set<string>();
    for (const t of trades) {
      set.add(t.outcome || "open");
    }
    return [...set].sort();
  }, [trades]);

  const shownTrades = useMemo(() => {
    return trades
      .filter((t) => (bookFilter === "all" ? true : t.profile === bookFilter))
      .filter((t) => (symFilter === "ALL" ? true : t.symbol === symFilter))
      .filter((t) => {
        if (outcomeFilter === "all") return true;
        const o = t.outcome || "open";
        return o === outcomeFilter;
      })
      .sort((a, b) => {
        const ao = !a.outcome || a.outcome === "open" ? 0 : 1;
        const bo = !b.outcome || b.outcome === "open" ? 0 : 1;
        if (ao !== bo) return ao - bo;
        return (b.entry_time || b.signal_time) - (a.entry_time || a.signal_time);
      });
  }, [trades, bookFilter, symFilter, outcomeFilter]);

  const liveTrades = useMemo(() => trades.filter((t) => t.profile === "live"), [trades]);
  const closedLive = useMemo(
    () => liveTrades.filter((t) => t.outcome && t.outcome !== "open"),
    [liveTrades],
  );

  return (
    <div className="flex flex-col gap-4">
      {stats ? (
        <div className="grid gap-3 sm:grid-cols-4">
          <StatCard title="Сделок Live" value={String(stats.trades)} sub={`${stats.wins} плюс / ${stats.losses} минус`} />
          <StatCard title="Итог Live" value={fmtUsd(stats.net)} warn={stats.net < 0} />
          <StatCard title="Профит-фактор" value={stats.profit_factor.toFixed(2)} />
          <StatCard title="Просадка" value={`${(stats.max_dd * 100).toFixed(1)}%`} />
        </div>
      ) : null}

      <Card>
        <CardHeader>
          <CardTitle>Журнал сделок</CardTitle>
          <CardDescription>
            Live — биржа. «Тень (фильтры)» и Baseline — бумажные книги. Раскройте строку для цен, проскальзывания и R.
          </CardDescription>
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
                <ToggleGroupItem value="shadow_baseline">baseline</ToggleGroupItem>
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
            <Field>
              <FieldTitle id="outcome-label">Исход</FieldTitle>
              <ToggleGroup
                type="single"
                size="sm"
                variant="outline"
                value={outcomeFilter}
                aria-labelledby="outcome-label"
                onValueChange={(v) => {
                  if (v) setOutcomeFilter(v);
                }}
              >
                <ToggleGroupItem value="all">все</ToggleGroupItem>
                {outcomes.map((o) => (
                  <ToggleGroupItem key={o} value={o}>
                    {outcomeLabel(o)}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Field>
          </div>

          {shownTrades.length === 0 ? (
            <QuietEmpty title="Сделок пока нет" text="Фильтры слишком жёсткие или журнал пуст." />
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-8" />
                  <TableHead>Когда</TableHead>
                  <TableHead>Счёт</TableHead>
                  <TableHead>Сделка</TableHead>
                  <TableHead>Исход</TableHead>
                  <TableHead>Live цена</TableHead>
                  <TableHead className="text-right">Итог</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {shownTrades.map((t) => {
                  const key = t.id;
                  const open = expanded === key;
                  return (
                    <Fragment key={`${t.profile}-${t.id}`}>
                      <TableRow className="cursor-pointer" onClick={() => setExpanded(open ? null : key)}>
                        <TableCell>
                          <Button type="button" variant="ghost" size="icon-xs" tabIndex={-1} aria-label={open ? "Свернуть" : "Развернуть"}>
                            {open ? <ChevronDownIcon /> : <ChevronRightIcon />}
                          </Button>
                        </TableCell>
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
                        <TableCell className={cn("text-right tabular-nums", t.net < 0 && "text-destructive")}>{fmtUsd(t.net)}</TableCell>
                      </TableRow>
                      {open ? (
                        <TableRow className="bg-muted/30 hover:bg-muted/30">
                          <TableCell colSpan={7}>
                            <TradeDetail t={t} />
                          </TableCell>
                        </TableRow>
                      ) : null}
                    </Fragment>
                  );
                })}
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
    </div>
  );
}

function TradeDetail({ t }: { t: Trade }) {
  const fees = totalFees(t);
  const hasSlip = t.profile === "live" && (t.entry_slip_bps != null || t.side_slip_bps != null);
  return (
    <div className="grid gap-3 p-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
      <Meta k="Сигнал" v={fmtTime(t.signal_time)} />
      <Meta k="Вход" v={fmtTime(t.entry_time)} />
      <Meta k="Выход" v={t.exit_time ? fmtTime(t.exit_time) : "—"} />
      <Meta k="Стоп" v={fmtPx(t.stop)} />
      <Meta k="Объём" v={fmtQty(t.quantity)} />
      <Meta k="Риск (USD)" v={t.risk_usd ? fmtUsd(t.risk_usd) : "—"} hint="distance × qty для Live." />
      <Meta k="R" v={t.r_multiple != null && t.outcome !== "open" ? t.r_multiple.toFixed(2) : "—"} hint="net / риск в USD." />
      <Meta k="Комиссии" v={fees ? fmtUsd(fees) : "—"} />
      <Meta k="Фандинг" v={t.funding ? fmtUsd(t.funding) : "—"} />
      <Meta
        k="Тень → Live (вход)"
        v={`${fmtPx(t.entry_px_shadow || t.entry_price)} → ${fmtPx(t.entry_px_live || t.entry_price)}`}
      />
      <Meta
        k="Тень → Live (выход)"
        v={
          t.exit_time
            ? `${fmtPx(t.exit_px_shadow || t.exit_price)} → ${fmtPx(t.exit_px_live || t.exit_price)}`
            : "—"
        }
      />
      {hasSlip ? (
        <>
          <Meta k="Проскальзывание вход" v={fmtBps(t.entry_slip_bps ?? NaN)} />
          <Meta k="Проскальзывание выход" v={fmtBps(t.exit_slip_bps ?? NaN)} />
          <Meta k="Суммарное (сторона)" v={fmtBps(t.side_slip_bps ?? NaN)} hint="Хуже для Live относительно тени, в б.п." />
        </>
      ) : (
        <Meta k="Проскальзывание" v="—" hint="Считается для Live относительно тени." />
      )}
      <div className="sm:col-span-2 lg:col-span-3">
        <p className="text-xs font-medium text-muted-foreground">Хронология</p>
        <p className="mt-1 text-sm">
          {fmtTime(t.signal_time)} сигнал → {fmtTime(t.entry_time || t.signal_time)} вход
          {t.exit_time ? ` → ${fmtTime(t.exit_time)} выход (${outcomeLabel(t.outcome)})` : " → позиция открыта"}
        </p>
      </div>
    </div>
  );
}
