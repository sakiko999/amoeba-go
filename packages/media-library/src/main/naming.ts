/**
 * 文件名季集号解析 + 特典识别 + 命名生成（移植自旧 core 的 organize/naming.rs，
 * 测试用例见 naming.test.ts —— 那是从 Rust 测试直译的黄金集）。
 *
 * 原则：宁可不解析，也不猜。解析不出的文件由调用方跳过并报告。
 * 模式优先级：S01E01 → 1x01 → 第N集 → EP01 → [NN] → 「 - NN 」。
 * 特典判定先于一切模式：`[01][Preview]` 含集号但不是正片，必须归入 Season 00。
 */

export interface SeasonEpisode {
  season: number;
  episode: number;
}

/** 正片集数（Season NN）或特典（Season 00，编号由调用方按排序统一分配） */
export type EpisodeKind =
  { kind: "regular"; se: SeasonEpisode } | { kind: "special"; hint?: number };

const SEPARATORS = ["_", "-", " ", "."];

function stemOf(filename: string): string {
  const name = filename.split(/[\\/]/).at(-1) ?? filename;
  const dot = name.lastIndexOf(".");
  return dot > 0 ? name.slice(0, dot) : name;
}

/** 只返回正片集数；特典视为无法解析 */
export function parseEpisode(filename: string): SeasonEpisode | undefined {
  const k = parseKind(filename);
  return k?.kind === "regular" ? k.se : undefined;
}

export function parseKind(filename: string): EpisodeKind | undefined {
  const stem = stemOf(filename);
  if (isSpecial(stem)) {
    return { kind: "special", hint: specialNumberHint(stem) };
  }
  const se = parseRegular(stem);
  return se === undefined ? undefined : { kind: "regular", se };
}

function parseRegular(stem: string): SeasonEpisode | undefined {
  return (
    matchSxe(stem) ??
    match1x01(stem) ??
    matchCjk(stem) ??
    matchEp(stem) ??
    matchBracketNumber(stem) ??
    matchDashNumber(stem)
  );
}

// —— 模式 1：S01E01（容忍 s1e1 / S01.E01 / S01 E01）
const RE_SXE = /\bS(\d{1,2})[\s._-]*E(\d{1,3})\b/i;

function matchSxe(s: string): SeasonEpisode | undefined {
  const m = RE_SXE.exec(s);
  return m ? { season: Number(m[1]), episode: Number(m[2]) } : undefined;
}

// —— 模式 2：1x01（「前面不是数字」排除 1920x1080）
const RE_1X01 = /(?:^|[^\dx])(\d{1,2})x(\d{1,3})(?:[^\d]|$)/i;

function match1x01(s: string): SeasonEpisode | undefined {
  const m = RE_1X01.exec(s);
  if (!m) return undefined;
  const season = Number(m[1]);
  // 季号 0 通常意味着误匹配（分辨率）；真 Season 0 用 S00Exx 形式，由模式 1 接管
  if (season === 0 || Number(m[2]) === 0) return undefined;
  return { season, episode: Number(m[2]) };
}

// —— 模式 3：第N集 / 第N话 / 第N話 / 第N回（无季信息，按单季）
const RE_CJK = /第\s*(\d{1,4})\s*[集话話回]/;

function matchCjk(s: string): SeasonEpisode | undefined {
  const m = RE_CJK.exec(s);
  return m && Number(m[1]) > 0
    ? { season: 1, episode: Number(m[1]) }
    : undefined;
}

// —— 模式 4：EP01 / E01 / ep.01（排除 1080p 之类数字+字母组合）
const RE_EP = /(?:^|[^A-Za-z])(?:EP|E)[\s._-]?(\d{1,3})(?:[^\dA-Za-z]|$)/i;

function matchEp(s: string): SeasonEpisode | undefined {
  const m = RE_EP.exec(s);
  return m && Number(m[1]) > 0
    ? { season: 1, episode: Number(m[1]) }
    : undefined;
}

// —— 模式 5：[NN] 方括号裸集号（字幕组主流命名）
// 正则只做粗筛，真判定手工做：数字 1-3 位、其后是分隔符或组结束（排除 1080p/10bit/2014）。
// 取最后一个满足约束的候选（集号通常靠后）。
const RE_BRACKET = /\[(\d{1,3})(?=[\]\s._-])/g;

function matchBracketNumber(s: string): SeasonEpisode | undefined {
  const matches = [...s.matchAll(RE_BRACKET)];
  for (const m of matches.reverse()) {
    const close = s.indexOf("]", m.index);
    if (close < 0) continue;
    const inner = s.slice(m.index + 1, close);
    const digits = inner.match(/^\d+/)?.[0] ?? "";
    if (digits.length === 0 || digits.length > 3) continue;
    const after = inner.slice(digits.length);
    if (after !== "" && !SEPARATORS.includes(after.charAt(0))) continue;
    const episode = Number(digits);
    if (episode > 0) return { season: 1, episode };
  }
  return undefined;
}

// —— 模式 6：「 - NN 」（短横两侧必须有空格，否则 SP-01 / 1080-2011 会中招）
const RE_DASH = /\s-\s(\d{1,4})(?:\s|$)/;

function matchDashNumber(s: string): SeasonEpisode | undefined {
  const m = RE_DASH.exec(s);
  return m && Number(m[1]) > 0
    ? { season: 1, episode: Number(m[1]) }
    : undefined;
}

// —— 特典识别：只放足够特异的词（裸 OP/ED/SP 易出现在剧名里，宁漏勿错）

const SPECIAL_KEYWORDS = [
  "preview",
  "trailer",
  "予告",
  "特典",
  "映像特典",
  "おまけ",
  "ncop",
  "nced",
  "non-credit",
  "creditless",
  "menu",
  "completebox",
  "commercial",
  "pv",
  "onair",
];

function bracketGroups(stem: string): string[] {
  const out: string[] = [];
  let rest = stem;
  for (;;) {
    const start = rest.indexOf("[");
    if (start < 0) break;
    const after = rest.slice(start + 1);
    const end = after.indexOf("]");
    if (end < 0) break;
    out.push(after.slice(0, end));
    rest = after.slice(end + 1);
  }
  return out;
}

/** CM<数字>：CM 后必须紧跟数字（防误伤 cmx/cms） */
function cmNumber(s: string): number | undefined {
  const lower = s.toLowerCase();
  let from = 0;
  for (;;) {
    const i = lower.indexOf("cm", from);
    if (i < 0) return undefined;
    const after = lower.slice(i + 2).replace(/^_+/, "");
    const digits = after.match(/^\d+/)?.[0];
    if (digits !== undefined && digits !== "") return Number(digits);
    from = i + 2;
    if (from >= lower.length) return undefined;
  }
}

/** `[00_SP]` / `[01_ncop]` 这类「集号 + 特典后缀」 */
function isSpecialSuffix(group: string): boolean {
  const lower = group.toLowerCase();
  if (lower.startsWith("00")) return true;
  for (const sep of SEPARATORS) {
    const idx = lower.indexOf(sep);
    if (idx >= 0) {
      const t = lower.slice(idx + 1).trim();
      if (SPECIAL_KEYWORDS.includes(t) || cmNumber(t) !== undefined)
        return true;
    }
  }
  return false;
}

function isSpecial(stem: string): boolean {
  for (const group of bracketGroups(stem)) {
    if (group.length > 40) continue;
    const lower = group.toLowerCase();
    if (SPECIAL_KEYWORDS.some((k) => lower.includes(k))) return true;
    if (isSpecialSuffix(lower)) return true;
    if (cmNumber(lower) !== undefined) return true;
  }
  // 无括号场景：按分隔符切词整词比较
  for (const token of stem.split(/[-_ .[]/)) {
    const lower = token.replaceAll("]", "").trim().toLowerCase();
    if (lower === "" || lower.length > 20) continue;
    if (SPECIAL_KEYWORDS.includes(lower) || cmNumber(lower) !== undefined)
      return true;
  }
  return false;
}

function specialNumberHint(stem: string): number | undefined {
  for (const group of bracketGroups(stem)) {
    const n = cmNumber(group);
    if (n !== undefined) return n;
  }
  for (const token of stem.split(/[-_ .[\]]/)) {
    const n = cmNumber(token);
    if (n !== undefined) return n;
  }
  return undefined;
}

// —— 命名生成

function seTag(se: SeasonEpisode): string {
  return `S${String(se.season).padStart(2, "0")}E${String(se.episode).padStart(2, "0")}`;
}

/** 规范文件名：S01E01.<原扩展>（无有效扩展回退 mkv） */
export function canonicalFilename(original: string, se: SeasonEpisode): string {
  const dot = original.lastIndexOf(".");
  const ext = dot > 0 ? original.slice(dot + 1) : "";
  return `${seTag(se)}.${ext !== "" && ext.length <= 5 ? ext : "mkv"}`;
}

/** 特典规范文件名：S00E<顺序号>.<原扩展> */
export function canonicalSpecialFilename(
  original: string,
  index: number,
): string {
  return canonicalFilename(original, { season: 0, episode: index });
}

export function seasonDir(season: number): string {
  return `Season ${String(season).padStart(2, "0")}`;
}

/** 作品目录名：`标题 (年份)`；非法字符替换 _，空标题回退「未命名」 */
export function showDir(title: string, year?: string): string {
  const sanitized = title
    .replaceAll(/[/\\:*?"<>|]/g, "_")
    .trim()
    .replace(/\.+$/, "");
  const base = sanitized === "" ? "未命名" : sanitized;
  return year !== undefined && year !== "" ? `${base} (${year})` : base;
}
