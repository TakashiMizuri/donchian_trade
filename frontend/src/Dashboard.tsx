import { useEffect, useState } from "react";
import { LogOutIcon } from "lucide-react";
import {
  Analysis,
  BarLog,
  CashFlow,
  DailyReport,
  EventRow,
  InstanceInfo,
  Mismatch,
  Stats,
  Status,
  Trade,
  api,
  getInstanceId,
  setInstanceId,
} from "./api";
import { networkLabel, strategyLabel } from "./labels";
import { ExecutionTab } from "./sections/ExecutionTab";
import { FiltersTab } from "./sections/FiltersTab";
import { HealthTab } from "./sections/HealthTab";
import { OverviewTab } from "./sections/OverviewTab";
import { TradesTab } from "./sections/TradesTab";
import { VerdictBadge } from "./sections/shared";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";

export default function Dashboard({ onLogout }: { onLogout: () => void }) {
  const [tab, setTab] = useState("overview");
  const [instances, setInstances] = useState<InstanceInfo[]>([]);
  const [instanceId, setInstanceIdState] = useState(getInstanceId());
  const [status, setStatus] = useState<Status | null>(null);
  const [trades, setTrades] = useState<Trade[]>([]);
  const [equity, setEquity] = useState<Awaited<ReturnType<typeof api.equity>>>([]);
  const [stats, setStats] = useState<Stats | null>(null);
  const [events, setEvents] = useState<EventRow[]>([]);
  const [mismatches, setMismatches] = useState<Mismatch[]>([]);
  const [cashFlows, setCashFlows] = useState<CashFlow[]>([]);
  const [daily, setDaily] = useState<DailyReport | null>(null);
  const [analysis, setAnalysis] = useState<Analysis | null>(null);
  const [barLogs, setBarLogs] = useState<BarLog[]>([]);
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function refresh() {
    try {
      const [s, t, e, st, ev, mm, cash, rep, an, bars] = await Promise.all([
        api.status(),
        api.trades(),
        api.equity(),
        api.stats(),
        api.events(),
        api.mismatches(),
        api.cash().catch(() => ({ cash_base: 0, flows: [] as CashFlow[] })),
        api.report().catch(() => ({ date_utc: "", body: "", verdict: "" })),
        api.analysis().catch(() => null),
        api.barLogs(undefined, 200).catch(() => [] as BarLog[]),
      ]);
      setStatus(s);
      setTrades(Array.isArray(t) ? t : []);
      setEquity(Array.isArray(e) ? e : []);
      setStats(st);
      setEvents(Array.isArray(ev) ? ev : []);
      setMismatches(Array.isArray(mm) ? mm : []);
      setCashFlows(Array.isArray(cash.flows) ? cash.flows : []);
      setDaily(rep.date_utc ? rep : null);
      setAnalysis(an);
      setBarLogs(Array.isArray(bars) ? bars : []);
      setErr("");
    } catch (e) {
      if (String(e) === "Error: auth") onLogout();
      else setErr(String(e));
    }
  }

  useEffect(() => {
    void api.instances().then(setInstances);
  }, []);

  useEffect(() => {
    setStatus(null);
    void refresh();
    const id = setInterval(() => void refresh(), 15000);
    return () => clearInterval(id);
  }, [instanceId]);

  async function onInstanceChange(next: string) {
    if (next === instanceId) return;
    setInstanceId(next);
    setInstanceIdState(next);
  }

  const activeVerdict = status?.kill_switch ? "STOP" : status?.verdict || "WAIT";
  const symbolList = (status?.symbols ?? []).map((s) => s.symbol);
  const instanceLabel =
    status?.instance_name ||
    instances.find((i) => i.id === instanceId)?.name ||
    instanceId;

  return (
    <div className="mx-auto flex min-h-dvh max-w-5xl flex-col">
      <header className="sticky top-0 z-10 flex items-center justify-between gap-3 border-b bg-background/90 px-4 py-3 backdrop-blur">
        <div className="min-w-0">
          <div className="font-heading text-sm font-medium">Donchian</div>
          <div className="truncate text-xs text-muted-foreground">
            {status
              ? `${instanceLabel} · ${networkLabel(status.network, status.dry_run)} · ${strategyLabel(status.strategy)}`
              : "загрузка…"}
          </div>
        </div>
        <div className="flex items-center gap-2">
          {instances.length > 0 ? (
            <select
              className="h-8 max-w-[9rem] rounded-md border bg-background px-2 text-xs"
              aria-label="Инстанс"
              value={instanceId}
              onChange={(e) => void onInstanceChange(e.target.value)}
            >
              {instances.map((i) => (
                <option key={i.id} value={i.id}>
                  {i.name}
                </option>
              ))}
            </select>
          ) : null}
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
            <TabsList className="flex h-auto w-full flex-wrap gap-1 sm:w-fit">
              <TabsTrigger value="overview">Обзор</TabsTrigger>
              <TabsTrigger value="trades">Сделки</TabsTrigger>
              <TabsTrigger value="execution">Исполнение</TabsTrigger>
              <TabsTrigger value="filters">Фильтры</TabsTrigger>
              <TabsTrigger value="health">Здоровье</TabsTrigger>
            </TabsList>

            <TabsContent value="overview" className="flex flex-col gap-4">
              <OverviewTab
                status={status}
                equity={equity}
                stats={stats}
                trades={trades}
                mismatches={mismatches}
              />
            </TabsContent>

            <TabsContent value="trades" className="flex flex-col gap-4">
              <TradesTab status={status} trades={trades} stats={stats} />
            </TabsContent>

            <TabsContent value="execution" className="flex flex-col gap-4">
              <ExecutionTab analysis={analysis} trades={trades} />
            </TabsContent>

            <TabsContent value="filters" className="flex flex-col gap-4">
              <FiltersTab barLogs={barLogs} analysis={analysis} symbols={symbolList} />
            </TabsContent>

            <TabsContent value="health" className="flex flex-col gap-4">
              <HealthTab
                status={status}
                events={events}
                mismatches={mismatches}
                cashFlows={cashFlows}
                daily={daily}
                busy={busy}
                setBusy={setBusy}
                onRefresh={refresh}
              />
            </TabsContent>
          </Tabs>
        )}
      </div>
    </div>
  );
}
