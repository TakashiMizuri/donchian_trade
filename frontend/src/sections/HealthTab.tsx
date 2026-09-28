import { toast } from "sonner";
import {
  CashFlow,
  DailyReport,
  EventRow,
  Mismatch,
  Status,
  api,
  fmtTime,
  fmtUsd,
  fmtUptime,
} from "../api";
import {
  cashLabel,
  checkLabel,
  eventKindLabel,
  eventLevelLabel,
  mismatchLabel,
  networkLabel,
  sideLabel,
  verdictLabel,
} from "../labels";
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
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Spinner } from "@/components/ui/spinner";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { Meta, QuietEmpty } from "./shared";

type Props = {
  status: Status;
  events: EventRow[];
  mismatches: Mismatch[];
  cashFlows: CashFlow[];
  daily: DailyReport | null;
  busy: boolean;
  setBusy: (v: boolean) => void;
  onRefresh: () => Promise<void>;
};

export function HealthTab({ status, events, mismatches, cashFlows, daily, busy, setBusy, onRefresh }: Props) {
  async function kill() {
    setBusy(true);
    try {
      await api.kill();
      await onRefresh();
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
      await onRefresh();
      toast("Аварийный стоп снят. Новые входы снова разрешены.");
    } catch (e) {
      toast.error(String(e));
    } finally {
      setBusy(false);
    }
  }

  const reconcileEvents = events.filter((e) => {
    const k = (e.kind || "").toLowerCase();
    const m = (e.message || "").toLowerCase();
    return k === "ws" || k === "risk" || k === "verdict" || m.includes("reconcile") || m.includes("mismatch");
  });

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Состояние</CardTitle>
          <CardDescription>Связь, аптайм и слои A/B/C.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 text-sm sm:grid-cols-2">
          <Meta k="Сеть" v={networkLabel(status.network, status.dry_run)} />
          <Meta k="Аварийный стоп" v={status.kill_switch ? "включён" : "выкл"} />
          <Meta k="Вердикт" v={verdictLabel(status.verdict)} />
          <Meta
            k="Входы / исполнение / PnL"
            v={`${checkLabel(status.status_a)} · ${checkLabel(status.status_b)} · ${checkLabel(status.status_c)}`}
          />
          <Meta
            k="WebSocket"
            v={status.ws_connected ? "подключён" : status.ws_error || "нет, REST ещё торгует"}
          />
          <Meta k="Аптайм" v={fmtUptime(status.uptime_sec)} />
          <Meta k="Старт Live" v={fmtTime(status.go_live_ts)} />
          <Meta k="Свободно" v={fmtUsd(status.available)} />
          <Meta k="Последняя ошибка" v={status.last_error || "нет"} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Расхождения входов</CardTitle>
          <CardDescription>Вход только у Live или только у тени.</CardDescription>
        </CardHeader>
        <CardContent>
          {mismatches.length === 0 ? (
            <QuietEmpty title="Расхождений нет" text="Live повторяет тень по входам." />
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
                {mismatches.map((m, i) => (
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
          <CardTitle>Сверка и стрим</CardTitle>
          <CardDescription>WS, вердикт, reconcile из журнала событий.</CardDescription>
        </CardHeader>
        <CardContent>
          {reconcileEvents.length === 0 ? (
            <QuietEmpty title="Записей мало" text="События ws/verdict/reconcile появятся по мере работы." />
          ) : (
            <div className="flex max-h-64 flex-col gap-3 overflow-auto">
              {reconcileEvents.slice(0, 30).map((e, i) => (
                <div key={`${e.ts}-${i}`} className="flex flex-col gap-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{fmtTime(e.ts)}</span>
                    <Badge variant={e.level === "error" ? "destructive" : "outline"}>
                      {eventLevelLabel(e.level)}
                      {e.kind ? ` · ${eventKindLabel(e.kind)}` : ""}
                    </Badge>
                  </div>
                  <p className="whitespace-pre-wrap text-sm">{e.message}</p>
                  {i < Math.min(reconcileEvents.length, 30) - 1 ? <Separator /> : null}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {daily?.date_utc ? (
        <Card>
          <CardHeader>
            <CardTitle>Суточный отчёт</CardTitle>
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
          <CardDescription>Пополнения и выводы. Тень копирует те же суммы.</CardDescription>
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
          <CardTitle>Все события</CardTitle>
        </CardHeader>
        <CardContent>
          {events.length === 0 ? (
            <QuietEmpty title="Журнал пуст" text="Входы, ошибки и системные сообщения." />
          ) : (
            <div className="flex max-h-96 flex-col gap-3 overflow-auto">
              {events.slice(0, 50).map((e, i) => (
                <div key={`${e.ts}-${i}`} className="flex flex-col gap-1">
                  <div className="flex items-center gap-2 text-xs text-muted-foreground">
                    <span>{fmtTime(e.ts)}</span>
                    <Badge variant={e.level === "error" ? "destructive" : "outline"}>
                      {eventLevelLabel(e.level)}
                      {e.kind ? ` · ${eventKindLabel(e.kind)}` : ""}
                    </Badge>
                  </div>
                  <p className="whitespace-pre-wrap text-sm">{e.message}</p>
                  {i < Math.min(events.length, 50) - 1 ? <Separator /> : null}
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Аварийный стоп</CardTitle>
          <CardDescription>Закрыть Live и остановить новые входы.</CardDescription>
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
    </div>
  );
}
