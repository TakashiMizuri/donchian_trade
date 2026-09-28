import { useMemo, useState } from "react";
import { BarLog, fmtPx, fmtTime } from "../api";
import { sideLabel, skipReasonLabel } from "../labels";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldTitle } from "@/components/ui/field";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group";
import { QuietEmpty } from "./shared";
import type { Analysis } from "../api";

type Props = {
  barLogs: BarLog[];
  analysis: Analysis | null;
  symbols: string[];
};

export function FiltersTab({ barLogs, analysis, symbols }: Props) {
  const [symFilter, setSymFilter] = useState("ALL");

  const rows = useMemo(() => {
    if (symFilter === "ALL") return barLogs;
    return barLogs.filter((b) => b.symbol === symFilter);
  }, [barLogs, symFilter]);

  const skipSummary = analysis?.skip_counts ?? {};

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Почему бар пропущен</CardTitle>
          <CardDescription>Агрегат skipped_reason из журнала баров (вся история в БД).</CardDescription>
        </CardHeader>
        <CardContent>
          {Object.keys(skipSummary).length === 0 ? (
            <QuietEmpty title="Статистики нет" text="Журнал bar_logs пуст или ещё не пишется." />
          ) : (
            <div className="flex flex-wrap gap-2">
              {Object.entries(skipSummary)
                .sort((a, b) => b[1] - a[1])
                .map(([reason, count]) => (
                  <Badge key={reason} variant="outline">
                    {skipReasonLabel(reason)} · {count}
                  </Badge>
                ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Последние бары</CardTitle>
          <CardDescription>Решение фильтров: breakout ATR, vol rank, skipped_reason.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          <Field>
            <FieldTitle id="bar-sym-label">Рынок</FieldTitle>
            <ToggleGroup
              type="single"
              size="sm"
              variant="outline"
              value={symFilter}
              aria-labelledby="bar-sym-label"
              onValueChange={(v) => {
                if (v) setSymFilter(v);
              }}
            >
              <ToggleGroupItem value="ALL">все</ToggleGroupItem>
              {symbols.map((s) => (
                <ToggleGroupItem key={s} value={s}>
                  {s}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </Field>

          {rows.length === 0 ? (
            <QuietEmpty title="Записей нет" text="bar_logs появятся после обработки часовых баров." />
          ) : (
            <div className="max-h-[32rem] overflow-auto rounded-md border">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Время</TableHead>
                    <TableHead>Рынок</TableHead>
                    <TableHead>Close</TableHead>
                    <TableHead>breakout ATR</TableHead>
                    <TableHead>vol rank</TableHead>
                    <TableHead>Причина</TableHead>
                    <TableHead>Хотели</TableHead>
                    <TableHead>Live</TableHead>
                    <TableHead>Тень</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rows.map((b) => (
                    <TableRow key={`${b.symbol}-${b.bar_time}`}>
                      <TableCell className="whitespace-nowrap">{fmtTime(b.bar_time)}</TableCell>
                      <TableCell className="font-medium">{b.symbol}</TableCell>
                      <TableCell>{fmtPx(b.close)}</TableCell>
                      <TableCell className="tabular-nums">{b.breakout_atr ? b.breakout_atr.toFixed(3) : "—"}</TableCell>
                      <TableCell className="tabular-nums">{b.vol_rank ? b.vol_rank.toFixed(3) : "—"}</TableCell>
                      <TableCell>
                        <Badge variant={b.skipped_reason === "ok" ? "secondary" : "outline"}>
                          {skipReasonLabel(b.skipped_reason)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-xs text-muted-foreground">
                        {b.want_live ? "live " : ""}
                        {b.want_baseline ? "base" : ""}
                        {!b.want_live && !b.want_baseline ? "—" : ""}
                      </TableCell>
                      <TableCell>{sideLabel(b.live_actual || b.live_desired)}</TableCell>
                      <TableCell>{sideLabel(b.shadow_twin)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
