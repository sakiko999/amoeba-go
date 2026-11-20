/**
 * 登录态与密钥：平台卡片每 4s 从宿主读取 credentials.json 镜像，
 * 扫码弹窗每 1.2s 轮询 LoginPoll；登录成功/退出登录后立即刷新卡片。
 */

import { useCallback, useEffect, useRef, useState } from "react";
import { Badge, Button, Card, CardContent } from "@demo/ui";
import { Loader2, ShieldCheck } from "lucide-react";
import {
  errText,
  loginCancel,
  loginPoll,
  loginStart,
  logout,
  status,
  type PlatformStatus,
} from "./host";
import { KeySection } from "./KeySection";
import { LoginDialog, type LoginState } from "./LoginDialog";

export function App(): React.ReactElement {
  const [platforms, setPlatforms] = useState<PlatformStatus[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [login, setLogin] = useState<LoginState | null>(null);
  // 出码请求序号:取消弹窗或重新刷新后,旧请求的结果不得再打开弹窗。
  const loginSeq = useRef(0);

  const refresh = useCallback(async () => {
    try {
      const s = await status();
      setPlatforms(s.platforms);
    } catch (e) {
      setError(errText(e));
    }
  }, []);

  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 4000);
    return () => clearInterval(timer);
  }, [refresh]);

  // 扫码轮询：只在二维码出现后开始——loading 阶段 LoginStart 还没建会话，
  // 提前轮询会闪「登录会话已失效」。成功后关掉会话并刷新卡片。
  const loginID = login?.platform;
  const loginPhase = login?.phase;
  useEffect(() => {
    if (loginID === undefined || loginPhase !== "qr") {
      return;
    }
    let alive = true;
    const timer = setInterval(() => {
      void (async () => {
        try {
          const r = await loginPoll(loginID);
          if (!alive) return;
          if (r.state === "success") {
            setLogin((cur) =>
              cur === null
                ? cur
                : {
                    ...cur,
                    phase: "success",
                    qr: undefined,
                    message: r.user ? `已登录：${r.user}` : undefined,
                  },
            );
            void refresh();
          } else if (r.state === "expired") {
            setLogin((cur) =>
              cur === null ? cur : { ...cur, phase: "expired", qr: undefined, message: r.message },
            );
          }
        } catch (e) {
          if (alive) {
            setLogin((cur) =>
              cur === null ? cur : { ...cur, phase: "error", message: errText(e) },
            );
          }
        }
      })();
    }, 1200);
    return () => {
      alive = false;
      clearInterval(timer);
    };
  }, [loginID, loginPhase, refresh]);

  const startLogin = async (p: Pick<PlatformStatus, "id" | "label">) => {
    const seq = ++loginSeq.current;
    setError(null);
    setLogin({ platform: p.id, label: p.label, phase: "loading" });
    try {
      const res = await loginStart(p.id);
      if (seq !== loginSeq.current) return;
      if (res.state === "already") {
        setLogin(null);
        await refresh();
        return;
      }
      setLogin({ platform: p.id, label: p.label, phase: "qr", qr: res.qr, qrText: res.qrText });
    } catch (e) {
      if (seq !== loginSeq.current) return;
      setLogin({ platform: p.id, label: p.label, phase: "error", message: errText(e) });
    }
  };

  const closeLogin = () => {
    loginSeq.current++;
    const id = login?.platform;
    setLogin(null);
    if (id !== undefined) void loginCancel(id).catch(() => {});
  };

  const doLogout = async (p: PlatformStatus) => {
    if (!window.confirm(`退出${p.label}登录？将同时清除浏览器里该平台的 cookie。`)) {
      return;
    }
    setBusy(p.id);
    setError(null);
    try {
      await logout(p.id);
      await refresh();
    } catch (e) {
      setError(errText(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-5 p-6">
      <header className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-accent">
          <ShieldCheck className="size-5" />
        </div>
        <div className="flex flex-col gap-1">
          <h1 className="text-lg font-semibold">登录态与密钥</h1>
          <p className="text-xs leading-relaxed text-muted-foreground">
            登录态直接读取浏览器载具 cookie，扫码成功后自动同步；退出登录会同时清除
            浏览器 cookie 与本地镜像。B 站经 API 出码，登录后回灌浏览器。
          </p>
        </div>
      </header>

      {error !== null && (
        <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-xs text-red-600">
          {error}
        </div>
      )}

      <section className="flex flex-col gap-3">
        <h2 className="text-sm font-medium text-muted-foreground">平台登录态</h2>
        {platforms === null ? (
          <div className="flex items-center gap-2 rounded-xl border bg-card p-6 text-sm text-muted-foreground">
            <Loader2 className="size-4 animate-spin" />
            正在读取浏览器登录态…
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            {platforms.map((p) => (
              <Card key={p.id}>
                <CardContent className="flex flex-col gap-3 p-4">
                  <div className="flex items-center gap-2">
                    <span
                      className="size-2 rounded-full"
                      style={{ backgroundColor: p.color ?? "#888" }}
                    />
                    <span className="text-sm font-medium">{p.label}</span>
                    {p.loggedIn ? (
                      <Badge className="bg-emerald-500/15 text-emerald-600 hover:bg-emerald-500/15">
                        已登录
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="text-muted-foreground">
                        未登录
                      </Badge>
                    )}
                  </div>
                  <p className="min-h-4 truncate text-xs text-muted-foreground">
                    {p.loggedIn
                      ? p.user !== undefined && p.user !== ""
                        ? `账号 ${p.user}`
                        : "浏览器已登录"
                      : "浏览器未登录"}
                    {p.stored ? " · 已同步到设置" : ""}
                  </p>
                  <div>
                    {p.loggedIn ? (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void doLogout(p)}
                        disabled={busy === p.id}
                      >
                        {busy === p.id && <Loader2 className="size-4 animate-spin" />}
                        退出登录
                      </Button>
                    ) : (
                      <Button size="sm" onClick={() => void startLogin(p)}>
                        扫码登录
                      </Button>
                    )}
                  </div>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </section>

      <KeySection />

      {login !== null && (
        <LoginDialog
          state={login}
          onClose={closeLogin}
          onRefresh={() => void startLogin({ id: login.platform, label: login.label })}
        />
      )}
    </div>
  );
}
