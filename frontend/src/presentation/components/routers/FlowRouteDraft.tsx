import { useState } from "react";
import { Segmented, Select, Space, Typography } from "antd";
import type { RouteAdvice } from "@/domain/entities";
import { FlowDraftBlocks } from "./FlowDraftBlocks";
import {
  draftTargets,
  preferDraft,
  rangeOptions,
  type DraftScope,
} from "./routeDraft";

/** Builds a draft for one prefix: which peer to prefer, and for how much. */
export function FlowRouteDraft({ advice }: { advice: RouteAdvice }) {
  const targets = draftTargets(advice);
  const [peer, setPeer] = useState(advice.suggested || targets[0]?.peer || "");
  const [scope, setScope] = useState<DraftScope>("exact");
  const ranges = rangeOptions(advice.prefix);
  const [rangeBits, setRangeBits] = useState(ranges[0]);
  const target = targets.find((p) => p.peer === peer);
  if (!target) return null;
  const draft = preferDraft(advice, target, scope, rangeBits);
  return (
    <div style={{ marginTop: 12 }}>
      <Typography.Paragraph strong style={{ marginBottom: 8 }}>
        Draf konfigurasi (tinjau, lalu jalankan sendiri di router)
      </Typography.Paragraph>
      <Space wrap style={{ marginBottom: 8 }}>
        <span>Utamakan lewat</span>
        <Select
          size="small"
          value={peer}
          onChange={setPeer}
          style={{ minWidth: 180 }}
          options={targets.map((p) => ({
            value: p.peer,
            label: `${p.peer} (${p.pathLength} AS${p.transit ? ", transit" : ""})`,
          }))}
        />
        <span>untuk</span>
        <Segmented
          size="small"
          value={scope}
          onChange={(v) => setScope(v as DraftScope)}
          options={[
            { label: "Prefix persis", value: "exact" },
            { label: "Rentang", value: "range" },
            { label: `Semua prefix AS${advice.asn}`, value: "asn" },
          ]}
        />
        {scope === "range" && (
          <Select
            size="small"
            value={rangeBits}
            onChange={setRangeBits}
            options={ranges.map((b) => ({ value: b, label: `/${b}` }))}
          />
        )}
      </Space>
      <FlowDraftBlocks draft={draft} />
    </div>
  );
}
