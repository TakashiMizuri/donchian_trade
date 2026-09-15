const PROFILE: Record<string, string> = {
  live: "Live",
  shadow_ls5: "Тень ls5",
  shadow_baseline: "Без паузы",
};

const SIDE: Record<string, string> = {
  BUY: "лонг",
  SELL: "шорт",
  FLAT: "нет позиции",
  LONG: "лонг",
  SHORT: "шорт",
};

const OUTCOME: Record<string, string> = {
  open: "открыта",
  sl: "стоп",
  time: "выход по каналу",
  kill: "kill-switch",
  watchdog: "сторож",
  rejected: "биржа отказала",
};

const MISMATCH: Record<string, string> = {
  live_only: "Live вошёл, тени не было",
  shadow_only: "Тень вошла, Live не вошёл",
};

const CASH: Record<string, string> = {
  seed: "старт счёта",
  deposit: "пополнение",
  withdraw: "вывод",
  unexplained: "необъяснённый остаток",
};

const LAYER: Record<string, { title: string; hint: string }> = {
  a: {
    title: "Входы",
    hint: "Совпадают ли реальные входы с тенью. Красный — торгуете не ту стратегию.",
  },
  b: {
    title: "Исполнение",
    hint: "Насколько хуже Live входит и выходит, чем тень. Это проскальзывание, не сигнал.",
  },
  c: {
    title: "PnL vs тень",
    hint: "Отношение прибыли Live (без funding) к тени. До ~20 закрытых пар это шум.",
  },
};

const CHECK: Record<string, string> = {
  green: "норма",
  yellow: "смотреть",
  orange: "плохо",
  red: "стоп",
  na: "рано",
};

export function profileLabel(v: string) {
  return PROFILE[v] ?? v;
}

export function sideLabel(v: string) {
  if (!v) return "нет позиции";
  return SIDE[v] ?? v;
}

export function outcomeLabel(v: string) {
  if (!v) return "открыта";
  return OUTCOME[v] ?? v;
}

export function mismatchLabel(v: string) {
  return MISMATCH[v] ?? v;
}

export function cashLabel(v: string) {
  return CASH[v] ?? v;
}

export function layerMeta(id: "a" | "b" | "c") {
  return LAYER[id];
}

export function checkLabel(v: string) {
  return CHECK[v?.toLowerCase()] ?? v ?? "—";
}

export function networkLabel(v: string, dryRun: boolean) {
  const net = v === "mainnet" ? "mainnet" : "тестнет";
  return dryRun ? `${net} · без ордеров` : net;
}

export function strategyLabel(v: string) {
  return v || "Donchian";
}

export function verdictCopy(v: string): { title: string; body: string } {
  switch (v) {
    case "STOP":
      return {
        title: "Стоп",
        body: "Либо включён kill-switch, либо Live и тень разошлись критично. Новые входы не идут. Тень продолжает считать сигналы.",
      };
    case "INVESTIGATE":
      return {
        title: "Нужно разобрать",
        body: "Есть расхождение входов или исполнения. Тест можно вести, на mainnet не переходить. Смотрите расхождения ниже.",
      };
    default:
      return {
        title: "Всё сходится",
        body: "Live и тень торгуют одно и то же. Это не разрешение на mainnet — только что сейчас нет разъезда.",
      };
  }
}
