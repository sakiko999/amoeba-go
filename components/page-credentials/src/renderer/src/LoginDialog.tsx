/** 扫码登录弹窗：加载中 / 二维码 / 成功 / 过期 / 错误五态，刷新重新出码。 */

import {
  Button,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@demo/ui";
import { CheckCircle2, Clock, Loader2, RefreshCw, XCircle } from "lucide-react";
import qrcode from "qrcode-generator";

export interface LoginState {
  platform: string;
  label: string;
  phase: "loading" | "qr" | "success" | "expired" | "error";
  qr?: string;
  /** 二维码内容（页面本地编码成图片） */
  qrText?: string;
  message?: string;
}

/** 页面本地把二维码内容编码成 data URL（B 站只提供内容字符串）。 */
function qrImage(state: LoginState): string | undefined {
  if (state.qr !== undefined) return state.qr;
  if (state.qrText === undefined || state.qrText === "") return undefined;
  const code = qrcode(0, "M");
  code.addData(state.qrText);
  code.make();
  return code.createDataURL(8, 2);
}

const HINTS: Record<LoginState["phase"], string> = {
  loading: "正在打开登录页并获取二维码…",
  qr: "请用手机 App 扫码并确认登录，完成后会自动同步。",
  success: "登录成功，浏览器登录态已同步。",
  expired: "二维码已过期，请刷新后重新扫码。",
  error: "扫码登录失败。",
};

export function LoginDialog({
  state,
  onClose,
  onRefresh,
}: {
  state: LoginState;
  onClose: () => void;
  onRefresh: () => void;
}): React.ReactElement {
  const src = state.phase === "qr" ? qrImage(state) : undefined;
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
    >
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{state.label}扫码登录</DialogTitle>
          <DialogDescription>{state.message ?? HINTS[state.phase]}</DialogDescription>
        </DialogHeader>

        <div className="flex min-h-52 items-center justify-center rounded-lg border bg-accent/30 p-4">
          {state.phase === "loading" && (
            <Loader2 className="size-8 animate-spin text-muted-foreground" />
          )}
          {state.phase === "qr" && src !== undefined && (
            <img
              src={src}
              alt={`${state.label}登录二维码`}
              className="size-48 rounded-md bg-white p-2"
            />
          )}
          {state.phase === "success" && (
            <CheckCircle2 className="size-12 text-emerald-500" />
          )}
          {state.phase === "expired" && <Clock className="size-12 text-muted-foreground" />}
          {state.phase === "error" && <XCircle className="size-12 text-red-500" />}
        </div>

        <DialogFooter className="gap-2">
          {state.phase !== "success" && (
            <Button variant="outline" onClick={onRefresh} disabled={state.phase === "loading"}>
              <RefreshCw className="size-4" />
              刷新二维码
            </Button>
          )}
          <Button onClick={onClose}>{state.phase === "success" ? "完成" : "取消"}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
