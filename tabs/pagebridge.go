// 页面侧宿主桥（对应 Electron 版 apps/desktop/src/preload/pageBridge.ts）。
// mygo 用 PageOptions.PreloadScript 在页面自有脚本之前注入，此时 window.mygo 已可用；
// 暴露 window.tab / window.hostState，能力经 Tabs 服务回到主线程。
// initial.theme 取注入时刻的真实主题（页面创建时同步可得）；之后的切换经
// host.state 事件广播。
package tabs

import "fmt"

const pageBridge = `(function () {
  const instanceId = %q, kind = %q, theme = %q;
  const call = (method, ...args) => window.mygo.call("Tabs." + method, ...args);
  Object.defineProperty(window, "tab", {
    configurable: true,
    value: {
      instanceId,
      kind,
      setTitle: (title) => { void call("SetTitle", instanceId, title); },
      close: () => { void call("Close", instanceId); },
      ping: (value) => call("Ping", instanceId, value),
      systemInfo: () => call("SystemInfo"),
      state: () => call("State"),
      onStateChanged: (cb) => { window.mygo.on("host.state", cb); },
      theme: () => call("State").then((state) => state.theme),
      onThemeChanged: (cb) => { window.mygo.on("host.state", (state) => cb(state.theme)); },
    },
  });
  window.hostState = {
    initial: { theme },
    onChanged: (cb) => { window.mygo.on("host.state", cb); },
  };
})();`

func pageBridgeScript(instanceID, kind, theme string) string {
	return fmt.Sprintf(pageBridge, instanceID, kind, theme)
}
