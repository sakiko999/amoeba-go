/**
 * 应用根：海报墙 ⇄ 详情两视图。库根持久化在 localStorage；扫描/筛选状态本地持有，
 * 筛选（genre/搜索）由库数据派生 —— 不硬编码分类。
 */

import { useCallback, useEffect, useMemo, useState } from "react";
import type { LibraryItem, ShowDetail } from "@tabs/media-library";
import { media } from "./lib/api";
import { DetailView } from "./components/DetailView";
import { LibraryGrid } from "./components/LibraryGrid";
import { TopBar } from "./components/TopBar";

const ROOT_KEY = "media-manager:root";

export function App(): React.JSX.Element {
  const [root, setRoot] = useState<string | null>(() =>
    localStorage.getItem(ROOT_KEY),
  );
  const [items, setItems] = useState<LibraryItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<ShowDetail | null>(null);
  const [query, setQuery] = useState("");
  const [genre, setGenre] = useState<string | null>(null);

  const scan = useCallback(async (target: string) => {
    setLoading(true);
    setError(null);
    try {
      setItems(await media().scanLibrary(target));
    } catch (err) {
      setItems([]);
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (root) void scan(root);
  }, [root, scan]);

  const pick = async (): Promise<void> => {
    const dir = await media().pickLibrary();
    if (dir) {
      localStorage.setItem(ROOT_KEY, dir);
      setRoot(dir);
    }
  };

  const openDetail = async (item: LibraryItem): Promise<void> => {
    setDetail(await media().showDetail(item.dir));
  };

  /** 刮削 / 编辑 / 整理写盘后刷新详情与库列表 */
  const refreshCurrent = useCallback(
    async (dir: string) => {
      setDetail(await media().showDetail(dir));
      if (root !== null) void scan(root);
    },
    [root, scan],
  );

  const genres = useMemo(
    () =>
      [...new Set(items.flatMap((i) => i.genres))].sort((a, b) =>
        a.localeCompare(b, "zh-Hans-CN"),
      ),
    [items],
  );
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter(
      (i) =>
        (genre === null || i.genres.includes(genre)) &&
        (q === "" ||
          `${i.title} ${i.originalTitle ?? ""} ${i.dirName}`
            .toLowerCase()
            .includes(q)),
    );
  }, [items, genre, query]);

  return (
    <div className="flex h-full flex-col bg-background text-foreground">
      <TopBar
        root={root}
        count={filtered.length}
        loading={loading}
        query={query}
        onQuery={setQuery}
        onPick={() => void pick()}
        onRefresh={() => root && void scan(root)}
      />
      {detail !== null ? (
        <DetailView
          detail={detail}
          root={root}
          onBack={() => setDetail(null)}
          onChanged={() => void refreshCurrent(detail.item.dir)}
        />
      ) : (
        <LibraryGrid
          items={filtered}
          genres={genres}
          genre={genre}
          onGenre={setGenre}
          loading={loading}
          error={error}
          hasRoot={root !== null}
          onPick={() => void pick()}
          onOpen={(item) => void openDetail(item)}
        />
      )}
    </div>
  );
}
