import { FormEvent, useState } from "react";
import { api } from "./api";

export default function Login({ onOk }: { onOk: () => void }) {
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setErr("");
    try {
      await api.login(password);
      onOk();
    } catch {
      setErr("Неверный пароль");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="min-h-dvh flex items-center justify-center px-5">
      <form onSubmit={submit} className="w-full max-w-sm rounded-2xl bg-slate-900/80 border border-slate-700 p-6 shadow-xl">
        <h1 className="text-xl font-semibold mb-1">Donchian Live</h1>
        <p className="text-slate-400 text-sm mb-5">Дашборд алго-бота на Lighter</p>
        <label className="block text-sm text-slate-300 mb-2">Пароль дашборда</label>
        <input
          type="password"
          autoComplete="current-password"
          className="w-full rounded-xl bg-slate-800 border border-slate-600 px-3 py-3 text-base outline-none focus:border-sky-400"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
        />
        {err && <p className="text-rose-400 text-sm mt-3">{err}</p>}
        <button
          disabled={busy}
          className="mt-5 w-full rounded-xl bg-sky-500 hover:bg-sky-400 disabled:opacity-50 py-3 font-medium text-slate-950"
        >
          {busy ? "Входим…" : "Войти"}
        </button>
      </form>
    </div>
  );
}
