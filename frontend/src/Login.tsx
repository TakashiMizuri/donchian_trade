import { FormEvent, useState } from "react";
import { api } from "./api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Field, FieldDescription, FieldError, FieldGroup, FieldLabel } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";

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
    <div className="flex min-h-dvh items-center justify-center px-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Donchian</CardTitle>
          <CardDescription>Дашборд бота. Live и тень на одних свечах.</CardDescription>
        </CardHeader>
        <CardContent>
          <form id="login" onSubmit={submit}>
            <FieldGroup>
              <Field data-invalid={err ? true : undefined}>
                <FieldLabel htmlFor="dashboard-password">Пароль</FieldLabel>
                <Input
                  id="dashboard-password"
                  type="password"
                  autoComplete="current-password"
                  value={password}
                  aria-invalid={err ? true : undefined}
                  onChange={(e) => setPassword(e.target.value)}
                />
                <FieldDescription>Пароль дашборда из настроек бота.</FieldDescription>
                {err ? <FieldError>{err}</FieldError> : null}
              </Field>
            </FieldGroup>
          </form>
        </CardContent>
        <CardFooter>
          <Button type="submit" form="login" className="w-full" disabled={busy}>
            {busy ? <Spinner data-icon="inline-start" /> : null}
            {busy ? "Входим…" : "Войти"}
          </Button>
        </CardFooter>
      </Card>
    </div>
  );
}
