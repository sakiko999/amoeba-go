/**
 * Host renderer（React 19 + Tailwind 4 + @demo/ui，与 media-manager 同栈）：
 * Chrome 风格 tab 条（视觉移植自 cherry-studio AppShellTabBar，交互态/动画见 styles.css）。
 * 状态仍全部来自主进程（tabs:changed 投影），操作经 window.host 发回；
 * 关闭先本地播 collapse 动画，200ms 后才上报主进程。
 * “+” 与右键菜单走主进程 Menu.popup：host DOM 会被 tab 的原生子视图遮挡。
 * 内容过渡页：tab 的 view 在 did-finish-load 前不可见，由这里渲染 loading 占位。
 */

import { useEffect, useLayoutEffect, useState } from "react";
import { createRoot } from "react-dom/client";
// 按子路径导入：barrel（"@demo/ui"）在 dev 下没有 tree-shaking，会把 60+ 组件及其
// 全部依赖都转换/请求一遍，host 首屏要多花十几秒
import { Button } from "@demo/ui/components/ui/button";
import {
  DndContext,
  PointerSensor,
  closestCenter,
  useSensor,
  useSensors,
  type DragEndEvent,
} from "@dnd-kit/core";
import {
  SortableContext,
  arrayMove,
  horizontalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable";
import { restrictToHorizontalAxis } from "@dnd-kit/modifiers";
import { CSS } from "@dnd-kit/utilities";
import { Bug, Moon, Plus, Sun, X } from "lucide-react";
import {
  hostApi,
  type HostState,
  type TabManifest,
  type TabSummary,
  type Theme,
} from "@host/sdk";
import "./styles.css";

const api = hostApi();

/** 冒烟观察点：无头环境页面加载极快，loading=true/false 会被 React 合并成一次提交，
 *  过渡页 DOM 可能从未出现；记录主进程投影确实到达 host，供 smoke 断言。 */
declare global {
  interface Window {
    __loadingSeen?: string[];
  }
}
function noteLoadingSeen(tabs: TabSummary[]): void {
  const seen = window.__loadingSeen ?? [];
  window.__loadingSeen = seen;
  for (const tab of tabs) {
    if (tab.loading && !seen.includes(tab.kind)) seen.push(tab.kind);
  }
}

interface TabItemProps {
  tab: TabSummary;
  color: string;
  closing: boolean;
  onClose: () => void;
}

/** 与 styles.css 的 .tab transition 对应；dnd-kit 只负责 transform 位移，其余过渡保留 */
const TAB_TRANSITION =
  "background-color 200ms ease-in-out, color 200ms ease-in-out, flex 200ms ease, margin 200ms ease, padding 200ms ease, opacity 40ms linear 140ms";

/** 双击（detail=2）与中键（auxclick button=1）都直接关闭，对照 Chrome/cherry-studio */
function TabItem({ tab, color, closing, onClose }: TabItemProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: tab.instanceId });

  return (
    <button
      ref={setNodeRef}
      type="button"
      className={`tab flex h-[30px] min-w-14 max-w-40 flex-1 basis-0 items-center gap-1.5 rounded-[10px] px-2 text-left text-xs${tab.active ? " active" : ""}${closing ? " closing" : ""}${isDragging ? " dragging" : ""}`}
      style={{
        transform: CSS.Transform.toString(transform),
        transition: transition ? `${transition}, ${TAB_TRANSITION}` : undefined,
      }}
      {...attributes}
      {...listeners}
      role="tab"
      aria-selected={tab.active}
      aria-label={tab.title}
      title={tab.title}
      onClick={(e) => {
        if (e.detail === 2) return; // 第二击交给 onDoubleClick，避免再 activate
        api.activateTab(tab.instanceId);
      }}
      onDoubleClick={(e) => {
        e.preventDefault();
        onClose();
      }}
      onAuxClick={(e) => {
        if (e.button === 1) {
          e.preventDefault();
          onClose();
        }
      }}
      onContextMenu={(e) => {
        e.preventDefault();
        api.openTabContextMenu(tab.instanceId);
      }}
    >
      <span
        className="size-2.5 shrink-0 rounded-full"
        style={{ background: color }}
      />
      <span className="label min-w-0 flex-1">{tab.title}</span>
      {/* biome-ignore lint/a11y/useSemanticElements: HTML 不允许 button 嵌套，关闭钮只能用 span 模拟 */}
      <span
        className="close"
        role="button"
        tabIndex={0}
        aria-label="关闭 tab"
        onClick={(e) => {
          e.stopPropagation();
          onClose();
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.stopPropagation();
            onClose();
          }
        }}
      >
        <X size={10} aria-hidden="true" />
      </span>
    </button>
  );
}

function App() {
  const [tabs, setTabs] = useState<TabSummary[]>([]);
  const [kinds, setKinds] = useState<TabManifest[]>([]);
  /** 本地正在播关闭动画的 tab：动画结束后才真正 close */
  const [closingIds, setClosingIds] = useState<ReadonlySet<string>>(
    () => new Set<string>(),
  );
  const [theme, setTheme] = useState<Theme>("dark");

  // HostApi 无 unsubscribe（demo 取舍），注册一次放 mount effect
  useEffect(() => {
    api.onTabsChanged((next) => {
      noteLoadingSeen(next);
      setTabs(next);
      setClosingIds((prev) => {
        if (prev.size === 0) return prev;
        // 主进程已移除的 tab 清出动画集，避免重开同名实例时误判 closing
        const alive = new Set(next.map((t) => t.instanceId));
        const kept = new Set([...prev].filter((id) => alive.has(id)));
        return kept.size === prev.size ? prev : kept;
      });
    });
    const refreshKinds = (): void => {
      void api.listKinds().then(setKinds);
    };
    api.onKindsChanged(refreshKinds);
    refreshKinds();
    api.onStateChanged((state: HostState) => setTheme(state.theme));
    void api.state().then((state) => setTheme(state.theme));
    // React 首帧提交后再撤掉内联启动骨架（index.html 的 #boot），避免空白交接
    document.getElementById("boot")?.remove();
  }, []);

  // @demo/ui 主题用 .dark class；用 layout effect 在首帧 paint 前切换，暗色启动不会闪亮色
  useLayoutEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "dark");
    document.documentElement.dataset.theme = theme;
  }, [theme]);

  const closeTab = (tab: TabSummary): void => {
    setClosingIds((prev) => {
      if (prev.has(tab.instanceId)) return prev;
      const next = new Set(prev);
      next.add(tab.instanceId);
      return next;
    });
    window.setTimeout(() => api.closeTab(tab.instanceId), 200);
  };

  const colorOf = (kind: string): string =>
    kinds.find((k) => k.id === kind)?.color ?? "#565f89";
  // loading 的 active tab：view 还没显示，host 用过渡页占住内容区
  const loading = tabs.find((t) => t.active && t.loading);

  // 5px 激活阈值：区分单击（激活 tab）与拖拽排序
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  );

  const onDragEnd = (event: DragEndEvent): void => {
    const { active, over } = event;
    if (over === null || active.id === over.id) return;
    // 乐观更新；主进程按同一规则重排后广播，最终以主进程为准
    setTabs((prev) => {
      const from = prev.findIndex((t) => t.instanceId === active.id);
      const to = prev.findIndex((t) => t.instanceId === over.id);
      return from < 0 || to < 0 ? prev : arrayMove(prev, from, to);
    });
    api.moveTab(String(active.id), String(over.id));
  };

  return (
    <>
      <header
        id="tabbar"
        className="flex h-11 items-center gap-1 border-b border-border bg-sidebar px-2"
      >
        <DndContext
          sensors={sensors}
          collisionDetection={closestCenter}
          modifiers={[restrictToHorizontalAxis]}
          onDragEnd={onDragEnd}
        >
          <div
            id="strip"
            role="tablist"
            aria-label="Tabs"
            className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto pr-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
          >
            <SortableContext
              items={tabs.map((tab) => tab.instanceId)}
              strategy={horizontalListSortingStrategy}
            >
              {tabs.map((tab) => (
                <TabItem
                  key={tab.instanceId}
                  tab={tab}
                  color={colorOf(tab.kind)}
                  closing={closingIds.has(tab.instanceId)}
                  onClose={() => closeTab(tab)}
                />
              ))}
            </SortableContext>
            {/* Launchpad 式 “+”：sticky 贴最后 tab、不随滚动消失 */}
            <Button
              variant="ghost"
              size="icon"
              className="sticky right-0 h-7 w-7 shrink-0"
              aria-label="新建 tab"
              title="新建 tab"
              onClick={() => api.openNewTabMenu()}
            >
              <Plus size={14} aria-hidden="true" />
            </Button>
          </div>
        </DndContext>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          aria-label="调试"
          title="调试"
          onClick={() => api.openDebugMenu()}
        >
          <Bug size={14} aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          aria-label={theme === "dark" ? "切换亮色主题" : "切换暗色主题"}
          title={theme === "dark" ? "切换亮色主题" : "切换暗色主题"}
          onClick={() => api.toggleTheme()}
        >
          {theme === "dark" ? (
            <Sun size={14} aria-hidden="true" />
          ) : (
            <Moon size={14} aria-hidden="true" />
          )}
        </Button>
      </header>
      {tabs.length === 0 && (
        <div className="absolute inset-x-0 bottom-0 top-11 flex items-center justify-center bg-background text-muted-foreground/60">
          没有打开的 tab —— 点上方 + 新建
        </div>
      )}
      {loading !== undefined && (
        <div
          className="absolute inset-x-0 bottom-0 top-11 flex flex-col items-center justify-center gap-2.5 bg-background text-muted-foreground"
          aria-live="polite"
        >
          <div className="spinner" />
          <div>正在加载 {loading.title}…</div>
        </div>
      )}
    </>
  );
}

const root = document.getElementById("root");
if (root === null) throw new Error("#root not found");
createRoot(root).render(<App />);
