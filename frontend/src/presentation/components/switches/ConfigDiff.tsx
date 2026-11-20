import { theme, Typography } from "antd";
import type { CSSProperties } from "react";

type LineKind = "header" | "hunk" | "add" | "del" | "context";

function lineKind(line: string): LineKind {
  if (line.startsWith("+++") || line.startsWith("---")) return "header";
  if (line.startsWith("@@")) return "hunk";
  if (line.startsWith("+")) return "add";
  if (line.startsWith("-")) return "del";
  return "context";
}

/** A unified diff of two masked configs: added lines green, removed red. */
export function ConfigDiff({ diff }: { diff: string }) {
  const { token } = theme.useToken();
  const styles: Record<LineKind, CSSProperties> = {
    add: { background: token.colorSuccessBg, color: token.colorSuccessText },
    del: { background: token.colorErrorBg, color: token.colorErrorText },
    hunk: { color: token.colorTextSecondary },
    header: { fontWeight: 600 },
    context: {},
  };
  if (diff === "") {
    return (
      <Typography.Text type="secondary">Tidak ada perbedaan</Typography.Text>
    );
  }
  return (
    <pre style={{ margin: 0, overflowX: "auto", fontSize: 12 }}>
      {diff
        .replace(/\n$/, "")
        .split("\n")
        .map((line, i) => {
          const kind = lineKind(line);
          return (
            <div key={i} data-kind={kind} style={styles[kind]}>
              {line}
            </div>
          );
        })}
    </pre>
  );
}
