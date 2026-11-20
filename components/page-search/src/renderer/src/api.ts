/**
 * 宿主 Search 服务的类型镜像与调用层（DTO 见 components/service-search/service.go
 * 与 platform/feed.go 的 Item；不改 @host/sdk —— 契约漂移由真机验收兜底）。
 * 流式：Query 的第三个参数是 mygo channel，每平台完成即推一条 PlatformGroup，
 * promise 结算 = 全部完成（其 groups 按 Sources 序，是权威快照）。
 */

export type Scope = "live" | "entity";

export interface Source {
  namespace: string;
  label?: string;
}

export interface Author {
  name: string;
  avatar?: string;
  handle?: string;
}

export interface Item {
  id: string;
  sourceId: string;
  kind: string;
  title: string;
  fetchedAt: number;
  url?: string;
  summary?: string;
  thumbnail?: string;
  author?: Author;
  publishedAt?: number;
  /** Live 项（live 房间搜索才有；entity 结果可能为空，平台身份看 group.namespace） */
  platform?: string;
  roomId?: string;
  liveStatus?: "live" | "offline" | "unknown";
  online?: number;
}

export interface GroupError {
  op?: string;
  code: string; // timeout | canceled | platform | internal
  message: string;
}

export interface PlatformGroup {
  namespace: string;
  label?: string;
  items: Item[];
  total?: number;
  hasMore?: boolean;
  cursor?: string;
  error?: GroupError;
  elapsedMs: number;
}

export interface QueryParams {
  keyword: string;
  scope: Scope;
  platforms?: string[];
}

export interface Result {
  keyword: string;
  scope: Scope;
  groups: PlatformGroup[];
  elapsedMs: number;
}

interface MygoChannel {
  close(): void;
}

interface MygoBridge {
  call(method: string, ...args: unknown[]): Promise<unknown>;
  channel<T>(onmessage?: (value: T) => void): MygoChannel;
}

function bridge(): MygoBridge {
  if (window.mygo === undefined) {
    throw new Error("window.mygo 未注入（页面不是 trusted page？）");
  }
  return window.mygo;
}

export async function sources(scope: Scope): Promise<Source[]> {
  return (await bridge().call("Search.Sources", scope)) as Source[];
}

export interface SearchHandle {
  /** 全部平台完成时结算；cancel 后仍会结算（Go 侧把取消记为分组数据） */
  done: Promise<Result>;
  /** 关闭流（Go 侧取消剩余平台）；供新搜索世代守卫与组件卸载用 */
  cancel(): void;
}

/**
 * 点击结果卡片:宿主经网关解析平台流,统一交给 player-h5 页面播放。
 */
export async function play(item: Item, namespace: string): Promise<void> {
  await bridge().call("Player.Play", {
    namespace,
    url: item.url,
    title: item.title,
    kind: item.kind,
  });
}

/**
 * 打开(或复用)个人主页 tab。平台身份由宿主按 namespace + 条目解析
 * (B站 uid / 斗鱼虎牙房间号 / twitch login / youtube channelId)。
 */
export async function openProfile(item: Item, namespace: string): Promise<void> {
  await bridge().call("Profile.Open", { namespace, item });
}

export async function openExternal(url: string): Promise<void> {
  await bridge().call("Tabs.OpenExternal", url);
}

export function query(
  params: QueryParams,
  onGroup: (group: PlatformGroup) => void,
): SearchHandle {
  const rt = bridge();
  const chan = rt.channel<PlatformGroup>(onGroup);
  const done = rt.call("Search.Query", params, chan).then(
    (raw) => raw as Result,
  );
  return { done, cancel: () => chan.close() };
}
