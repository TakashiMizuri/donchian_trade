const PROFILE: Record<string, string> = {
  live: "Live",
  shadow_ls5: "Тень",
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
  time: "по каналу",
  kill: "аварийный стоп",
  watchdog: "автозакрытие",
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
    hint: "Совпадают ли реальные входы с тенью по часу и стороне.",
  },
  b: {
    title: "Исполнение",
    hint: "Насколько цена Live хуже тени. Считается по закрытым совпавшим сделкам.",
  },
  c: {
    title: "PnL vs тень",
    hint: "Отношение прибыли Live (без фандинга) к тени. Пока мало закрытых сделок — это шум.",
  },
};

const CHECK: Record<string, string> = {
  green: "норма",
  yellow: "смотреть",
  orange: "плохо",
  red: "стоп",
  na: "рано",
};

const EVENT_LEVEL: Record<string, string> = {
  error: "ошибка",
  warn: "внимание",
  info: "событие",
  debug: "отладка",
};

const EVENT_KIND: Record<string, string> = {
  entry: "вход",
  exit: "выход",
  sl: "стоп",
  stop: "стоп",
  cash: "касса",
  ws: "стрим",
  risk: "риск",
  verdict: "вердикт",
  kill: "аварийный стоп",
  watchdog: "автозакрытие",
  report: "отчёт",
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

export function eventLevelLabel(v: string) {
  return EVENT_LEVEL[v?.toLowerCase()] ?? v;
}

export function eventKindLabel(v: string) {
  return EVENT_KIND[v?.toLowerCase()] ?? v;
}

export function networkLabel(v: string, dryRun: boolean) {
  const net = v === "mainnet" ? "mainnet" : "тестнет";
  return dryRun ? `${net} · без ордеров` : net;
}

export function strategyLabel(v: string) {
  return v || "Donchian";
}

export function verdictLabel(v: string) {
  switch (v) {
    case "STOP":
      return "стоп";
    case "INVESTIGATE":
      return "разобрать";
    default:
      return "сходится";
  }
}

export function verdictCopy(v: string): { title: string; body: string } {
  switch (v) {
    case "STOP":
      return {
        title: "Стоп",
        body: "Включён аварийный стоп или Live и тень разошлись. Новые входы не идут. Тень продолжает считать сигналы.",
      };
    case "INVESTIGATE":
      return {
        title: "Нужно разобрать",
        body: "Есть расхождение входов или исполнения. Список — в блоке «Расхождения».",
      };
    default:
      return {
        title: "Всё сходится",
        body: "Live и тень торгуют одно и то же.",
      };
  }
}
