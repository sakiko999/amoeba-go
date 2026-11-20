/**
 * credentials 与宿主的契约（Go 侧 components/service-credentials/Service）。
 *
 * 载具按任务启停，Status 只读 credentials.json 镜像（登录成功/退出时更新）；
 * 登录流程内（LoginStart/LoginPoll）才按需读取载具。
 * 密钥只回传脱敏值（masked），明文只上行。
 */

export interface PlatformStatus {
  id: string;
  label: string;
  color?: string;
  loggedIn: boolean;
  user?: string;
  /** credentials.json 里有 cookie 镜像（HTTP 渠道可复用） */
  stored: boolean;
}

export interface StatusResult {
  platforms: PlatformStatus[];
}

export interface StartResult {
  state: "qr" | "already";
  /** 二维码图片（data URL / 远程图） */
  qr?: string;
  /** 二维码内容（B 站等纯 API 平台只给内容，页面本地渲染成图片） */
  qrText?: string;
  user?: string;
  expiresIn?: number;
}

export interface PollResult {
  state: "pending" | "success" | "expired";
  user?: string;
  message?: string;
}

export interface LogoutResult {
  cleared: number;
}

export interface KeyStatus {
  name: string;
  label: string;
  set: boolean;
  masked?: string;
}

export function errText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

function bridge(): NonNullable<Window["mygo"]> {
  if (window.mygo === undefined) {
    throw new Error("window.mygo 未注入（页面不是 trusted page？）");
  }
  return window.mygo;
}

export async function status(): Promise<StatusResult> {
  return (await bridge().call("Credentials.Status")) as StatusResult;
}

export async function loginStart(platform: string): Promise<StartResult> {
  return (await bridge().call("Credentials.LoginStart", { platform })) as StartResult;
}

export async function loginPoll(platform: string): Promise<PollResult> {
  return (await bridge().call("Credentials.LoginPoll", { platform })) as PollResult;
}

export async function loginCancel(platform: string): Promise<void> {
  await bridge().call("Credentials.LoginCancel", { platform });
}

export async function logout(platform: string): Promise<LogoutResult> {
  return (await bridge().call("Credentials.Logout", { platform })) as LogoutResult;
}

export async function keys(): Promise<KeyStatus[]> {
  return (await bridge().call("Credentials.Keys")) as KeyStatus[];
}

export async function setKey(name: string, value: string): Promise<void> {
  await bridge().call("Credentials.SetKey", { name, value });
}
