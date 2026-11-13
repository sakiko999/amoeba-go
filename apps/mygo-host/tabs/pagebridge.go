// 页面侧宿主桥（对应 Electron 版 apps/desktop/src/preload/pageBridge.ts）。
// mygo 用 PageOptions.PreloadScript 在页面自有脚本之前注入，此时 window.mygo 已可用；
// 暴露 window.tab / window.hostState，能力经 Tabs 服务回到主线程。
// 已知上限：hostState.initial 固定 dark（mygo 无同步调用），页面应以
// theme()/onThemeChanged 为准。
package tabs

import "fmt"

const pageBridge = `(function () {
  const instanceId = %q, kind = %q;
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
    initial: { theme: "dark" },
    onChanged: (cb) => { window.mygo.on("host.state", cb); },
  };
})();`

func pageBridgeScript(instanceID, kind string) string {
	return fmt.Sprintf(pageBridge, instanceID, kind)
}
