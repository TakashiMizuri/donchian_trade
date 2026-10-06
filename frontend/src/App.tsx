import { useEffect, useState } from "react";
import { api } from "./api";
import Dashboard from "./Dashboard";
import Login from "./Login";
import { Spinner } from "@/components/ui/spinner";

export default function App() {
  const [authed, setAuthed] = useState<boolean | null>(null);

  useEffect(() => {
    void (async () => {
      await api.instances();
      try {
        await api.status();
        setAuthed(true);
      } catch {
        setAuthed(false);
      }
    })();
  }, []);

  if (authed === null) {
    return (
      <div className="flex min-h-dvh items-center justify-center">
        <Spinner />
      </div>
    );
  }
  if (!authed) {
    return <Login onOk={() => setAuthed(true)} />;
  }
  return <Dashboard onLogout={() => setAuthed(false)} />;
}
