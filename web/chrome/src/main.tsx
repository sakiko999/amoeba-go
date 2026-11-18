/**
 * chrome renderer —— tab 条（mygo 版）。
 * 移植自 apps/desktop/src/ui/renderer.tsx：同样的 React 19 + Tailwind 4 + @demo/ui + dnd-kit，
 * 差异：chrome webview 只有 44px 高，DOM 浮层会被视口裁掉，所以“+”与 tab 右键菜单
 * 都交给宿主用原生 Menu 在鼠标位置弹出（Tabs.PopupNewTabMenu/PopupTabMenu）；
 * 状态权威源在 Go 宿主（Tabs 服务 + tabs.changed 事件投影），操作经 window.mygo 桥发回。
 */

import { useEffect, useLayoutEffect, useState } from "react";
import { createRoot } from "react-dom/client";
// 按子路径导入：barrel 在 dev 下没有 tree-shaking（Electron 版同款教训）
import { Button } from "@demo/ui/components/ui/button";
import { DndContext, PointerSensor, closestCenter, useSensor, useSensors, type DragEndEvent } from "@dnd-kit/core";
import { SortableContext, arrayMove, horizontalListSortingStrategy, useSortable } from "@dnd-kit/sortable";
import { restrictToHorizontalAxis, restrictToParentElement } from "@dnd-kit/modifiers";
import { CSS } from "@dnd-kit/utilities";
import { Moon, Plus, Sun, X } from "lucide-react";
import { hostApi, type TabManifest, type TabSummary, type Theme } from "./host";
import "./styles.css";

const api = hostApi();

/** 与 styles.css 的 .tab transition 对应；dnd-kit 只负责 transform 位移，其余过渡保留 */
const TAB_TRANSITION =
  "background-color 200ms ease-in-out, color 200ms ease-in-out, flex 200ms ease, margin 200ms ease, padding 200ms ease, opacity 40ms linear 140ms";

interface TabItemProps {
  tab: TabSummary;
  color: string;
  closing: boolean;
  onClose: () => void;
}

/** 双击（detail=2）与中键（auxclick button=1）都直接关闭，对照 Chrome/cherry-studio */
function TabItem({ tab, color, closing, onClose }: TabItemProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: tab.instanceId,
  });

  return (
    <button
      ref={setNodeRef}
      type="button"
      className={`tab flex h-[30px] min-w-14 max-w-40 flex-1 basis-0 items-center gap-1.5 rounded-[10px] px-2 text-left text-xs${tab.active ? " active" : ""}${closing ? " closing" : ""}${isDragging ? " dragging" : ""}`}
      style={{
        // 只取位移：CSS.Transform 会带 adjustScale 的缩放（按 over 宽度拉伸可变宽 tab）
        transform: CSS.Translate.toString(transform),
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
      <span className="size-2.5 shrink-0 rounded-full" style={{ background: color }} />
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
  const [closingIds, setClosingIds] = useState<ReadonlySet<string>>(() => new Set());
  const [theme, setTheme] = useState<Theme>("dark");

  useEffect(() => {
    const offTabs = api.onTabsChanged((next) => {
      setTabs(next);
      setClosingIds((prev) => {
        if (prev.size === 0) return prev;
        const alive = new Set(next.map((t) => t.instanceId));
        const kept = new Set([...prev].filter((id) => alive.has(id)));
        return kept.size === prev.size ? prev : kept;
      });
    });
    const offState = api.onStateChanged((state) => setTheme(state.theme));
    void api.listKinds().then(setKinds);
    void api.state().then((state) => setTheme(state.theme));
    // 系统窗口按钮区预留（webview 里没有 mygo 注入的 CSS 变量，向宿主取）
    void api.chrome().then((info) => {
      document.documentElement.style.setProperty(
        "--mygo-titlebar-inset-right",
        `${info.insetRight + 8}px`,
      );
    });
    // React 首帧提交后再撤掉内联启动骨架（index.html 的 #boot）
    document.getElementById("boot")?.remove();
    return () => {
      offTabs();
      offState();
    };
  }, []);

  // @demo/ui 主题用 .dark class；layout effect 在首帧 paint 前切换，暗色启动不会闪亮色
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

  // 5px 激活阈值：区分单击（激活 tab）与拖拽排序
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  );

  const onDragEnd = (event: DragEndEvent): void => {
    const { active, over } = event;
    if (over === null || active.id === over.id) return;
    // 乐观更新；宿主按同一规则重排后广播，最终以宿主为准
    setTabs((prev) => {
      const from = prev.findIndex((t) => t.instanceId === active.id);
      const to = prev.findIndex((t) => t.instanceId === over.id);
      return from < 0 || to < 0 ? prev : arrayMove(prev, from, to);
    });
    api.moveTab(String(active.id), String(over.id));
  };

  return (
    <header
      id="tabbar"
      className="flex h-11 items-center gap-1 border-b border-border bg-sidebar px-2"
    >
      <DndContext
        sensors={sensors}
        collisionDetection={closestCenter}
        modifiers={[
          restrictToHorizontalAxis,
          // 钳在 #strip 内：transform 越过右缘会撑大 scrollWidth，dnd-kit 默认
          // autoScroll 借此自激滚动（左侧 tab 被滚出视口、被拖 tab 抖动，#2107）
          restrictToParentElement,
        ]}
        onDragEnd={onDragEnd}
      >
        <div
          id="strip"
          role="tablist"
          aria-label="Tabs"
          className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto pr-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          <SortableContext items={tabs.map((tab) => tab.instanceId)} strategy={horizontalListSortingStrategy}>
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
          {/* “+” 新建菜单：Launchpad 式 sticky 贴最后 tab、不随滚动消失；
              菜单本体由宿主原生弹出（Tabs.PopupNewTabMenu） */}
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
        aria-label={theme === "dark" ? "切换亮色主题" : "切换暗色主题"}
        title={theme === "dark" ? "切换亮色主题" : "切换暗色主题"}
        onClick={() => api.toggleTheme()}
      >
        {theme === "dark" ? <Sun size={14} aria-hidden="true" /> : <Moon size={14} aria-hidden="true" />}
      </Button>
    </header>
  );
}

const root = document.getElementById("root");
if (root === null) throw new Error("#root not found");
createRoot(root).render(<App />);
