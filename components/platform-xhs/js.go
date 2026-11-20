// xhs 页面表达式(逐字对齐 TS platform-xhs 源码):SSR 等待、风控框、
// 登录判定链与 __INITIAL_STATE__ 用户投影。表达式全部是字面量,不嵌入外部值;
// 投影只回传标量 JSON 字符串(整棵 state 含循环引用,returnByValue 会失败)。

package xhs

// ssrReadyExpr 等 SSR 挂载;登录重定向也算「有结果」,由登录闸门给出可操作错误。
const ssrReadyExpr = `typeof window.__INITIAL_STATE__ !== 'undefined' || location.pathname.startsWith('/login')`

// riskBoxExpr 风控验证框(.fe-verify-box)。
const riskBoxExpr = `!!document.querySelector('.fe-verify-box')`

// loginPageExpr 当前页是否被重定向到登录页(必须同时确认在 xhs origin)。
const loginPageExpr = `location.origin === "https://www.xiaohongshu.com" && location.pathname.startsWith('/login')`

// pageLoggedInExpr 页面 SSR 登录位;null = 读不到(未挂载/导航过渡/结构变化)。
const pageLoggedInExpr = `(() => {
  const u = window.__INITIAL_STATE__?.user;
  const l = u?.loggedIn?._rawValue ?? u?.loggedIn;
  return typeof l === 'boolean' ? l : null;
})()`

// domLoggedInExpr 登录后侧栏有指向自己的「我」入口;匿名页没有。
const domLoggedInExpr = `(() => {
  const a = [...document.querySelectorAll('a[href*="/user/profile/"]')]
    .find((n) => n.textContent && n.textContent.trim() === '我');
  return !!a;
})()`

// clickUserTabExpr 点搜索结果页的「用户」Tab(默认落在「全部」,切换才发
// usersearch 请求)。找不到返回 false;用 dispatchEvent 而非 el.click()。
const clickUserTabExpr = `(() => {
  const nodes = [...document.querySelectorAll('a,div,span,li,button')];
  const el = nodes.find((e) => e.children.length === 0 && /^用户\s*[(（]?[\d.万]*[)）]?$/.test(e.textContent.trim()));
  if (!el) return false;
  el.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, view: window }));
  return true;
})()`

// ssrUsersExpr 从 __INITIAL_STATE__ 深度遍历用户形状对象,最多 40 条。
// 字段优先级与 TS user-search.ts 的 readSearchStateViaBrowser 一字不差。
const ssrUsersExpr = `(() => {
  const un = (v) => (v && typeof v === "object" && "_rawValue" in v) ? v._rawValue : v;
  const str = (v) => {
    v = un(v);
    return typeof v === "string" ? v : (v && typeof v === "object" ? String(un(v.url ?? v.urlDefault ?? "")) : "");
  };
  const out = [];
  const seen = new Set();
  const visit = (v, d) => {
    v = un(v);
    if (!v || typeof v !== "object" || d > 10 || seen.has(v)) return;
    seen.add(v);
    const userish = v.red_id != null || v.fans != null || v.xsec_token != null || v.note_count != null || v.sub_title != null;
    const id = String(un(v.user_id ?? v.userId ?? v.userid ?? (userish ? v.id : "") ?? ""));
    const name = String(un(v.nickname ?? v.name ?? ""));
    if (id && name && !v.note_id && !v.noteId) {
      out.push({
        userId: id,
        nickname: name,
        avatar: str(v.image ?? v.avatar ?? v.imageb),
        redId: String(un(v.red_id ?? v.redId ?? "")),
        fans: Number(un(v.fans ?? v.follower_count ?? v.fans_count)) || 0,
      });
    }
    if (Array.isArray(v)) { for (const x of v) visit(x, d + 1); return; }
    for (const k of Object.keys(v)) visit(v[k], d + 1);
  };
  visit(window.__INITIAL_STATE__ || {}, 0);
  return JSON.stringify(out.slice(0, 40));
})()`
