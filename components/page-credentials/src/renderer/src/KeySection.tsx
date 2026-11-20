/** API 密钥区：只展示脱敏值，明文仅上行；保存/清除后重拉状态。 */

import { useCallback, useEffect, useState } from "react";
import {
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  Input,
} from "@demo/ui";
import { KeyRound, Trash2 } from "lucide-react";
import { errText, keys, setKey, type KeyStatus } from "./host";

export function KeySection(): React.ReactElement {
  const [items, setItems] = useState<KeyStatus[]>([]);
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState<string | null>(null);
  const [note, setNote] = useState<string | null>(null);

  const load = useCallback(async () => setItems(await keys()), []);
  useEffect(() => {
    load().catch((e: unknown) => setNote(errText(e)));
  }, [load]);

  const run = async (name: string, value: string, done: string) => {
    setBusy(name);
    setNote(null);
    try {
      await setKey(name, value);
      setDrafts((d) => ({ ...d, [name]: "" }));
      await load();
      setNote(done);
    } catch (e) {
      setNote(errText(e));
    } finally {
      setBusy(null);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-sm">
          <KeyRound className="size-4" />
          API 密钥
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {items.map((k) => {
          const draft = drafts[k.name] ?? "";
          return (
            <div key={k.name} className="flex flex-col gap-2">
              <div className="flex items-center gap-2 text-sm">
                <span className="font-medium">{k.label}</span>
                <Badge variant={k.set ? "secondary" : "outline"}>
                  {k.set ? `已设置 ${k.masked ?? ""}` : "未设置"}
                </Badge>
              </div>
              <div className="flex gap-2">
                <Input
                  type="password"
                  autoComplete="off"
                  aria-label={k.label}
                  placeholder={k.set ? "输入新值以替换" : "粘贴 token"}
                  value={draft}
                  onChange={(e) =>
                    setDrafts((d) => ({ ...d, [k.name]: e.target.value }))
                  }
                />
                <Button
                  onClick={() => void run(k.name, draft.trim(), "已保存")}
                  disabled={busy === k.name || draft.trim() === ""}
                >
                  保存
                </Button>
                <Button
                  variant="outline"
                  aria-label={`清除 ${k.label}`}
                  onClick={() => void run(k.name, "", "已清除")}
                  disabled={busy === k.name || !k.set}
                >
                  <Trash2 className="size-4" />
                </Button>
              </div>
            </div>
          );
        })}
        {note !== null && <p className="text-xs text-muted-foreground">{note}</p>}
        <p className="text-xs leading-relaxed text-muted-foreground">
          TMDB Token 也可用 TMDB_TOKEN 环境变量兜底；Bangumi Token 可选，仅提升请求配额。
        </p>
      </CardContent>
    </Card>
  );
}
