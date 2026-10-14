import { Typography } from "antd";
import type { Draft } from "./routerOS";

const SECTIONS: {
  key: keyof Omit<Draft, "note">;
  title: string;
  hint: string;
}[] = [
  {
    key: "precheck",
    title: "1. Cek dulu (read-only)",
    hint: "Tidak mengubah apa pun. Setiap perintah print harus menampilkan tepat satu aturan; kalau tidak, jangan lanjut.",
  },
  { key: "apply", title: "2. Terapkan", hint: "Mengubah konfigurasi router." },
  {
    key: "check",
    title: "3. Cek hasil",
    hint: "Read-only: pastikan perubahan berlaku.",
  },
  {
    key: "undo",
    title: "4. Batalkan",
    hint: "Mengembalikan konfigurasi seperti semula.",
  },
];

/** A draft as separate blocks, each with its own copy button. */
export function FlowDraftBlocks({ draft }: { draft: Draft }) {
  return (
    <div>
      <Typography.Paragraph type="secondary">{draft.note}</Typography.Paragraph>
      {SECTIONS.filter((s) => draft[s.key].length > 0).map((s) => {
        const text = draft[s.key].join("\n");
        return (
          <div key={s.key} style={{ marginBottom: 12 }}>
            <Typography.Text strong>{s.title}</Typography.Text>{" "}
            <Typography.Text type="secondary">{s.hint}</Typography.Text>
            <Typography.Paragraph
              copyable={{ text }}
              style={{ marginTop: 4, marginBottom: 0 }}
            >
              <pre style={{ margin: 0, whiteSpace: "pre-wrap" }}>{text}</pre>
            </Typography.Paragraph>
          </div>
        );
      })}
    </div>
  );
}
