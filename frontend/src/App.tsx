import { useEffect, useState } from "react";
import { api } from "./api";
import Dashboard from "./Dashboard";
import Login from "./Login";

export default function App() {
  const [authed, setAuthed] = useState<boolean | null>(null);

  useEffect(() => {
    api
      .status()
      .then(() => setAuthed(true))
      .catch(() => setAuthed(false));
  }, []);

  if (authed === null) {
    return <div className="min-h-dvh flex items-center justify-center text-slate-400">Загрузка…</div>;
  }
  if (!authed) {
    return <Login onOk={() => setAuthed(true)} />;
  }
  return <Dashboard onLogout={() => setAuthed(false)} />;
}
