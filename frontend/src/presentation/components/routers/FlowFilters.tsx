import { Radio, Select, Space } from "antd";
import type {
  FlowDirection,
  FlowFilter,
  RouterInterface,
} from "@/domain/entities";
import { OTHER_POP } from "./routerFlows";
import { interfaceLabel } from "./routerTraffic";

const DIRECTIONS = [
  { label: "Download", value: "download" },
  { label: "Upload", value: "upload" },
];

interface Props {
  filter: FlowFilter;
  onChange: (filter: FlowFilter) => void;
  pops: RouterInterface[];
  upstreams: RouterInterface[];
  /** Upstreams by their BGP sessions, as the tables name them. */
  labels: Map<string, string>;
}

export function FlowFilters({
  filter,
  onChange,
  pops,
  upstreams,
  labels,
}: Props) {
  const popOptions = [
    ...pops.map((i) => ({ label: interfaceLabel(i), value: i.id })),
    { label: "Lainnya (di luar blok POP)", value: OTHER_POP },
  ];
  return (
    <Space wrap>
      <Radio.Group
        optionType="button"
        options={DIRECTIONS}
        value={filter.direction}
        onChange={(e) =>
          onChange({ ...filter, direction: e.target.value as FlowDirection })
        }
      />
      <Select
        allowClear
        placeholder="Semua POP"
        style={{ minWidth: 220 }}
        value={filter.pop}
        options={popOptions}
        onChange={(pop: string | undefined) => onChange({ ...filter, pop })}
      />
      <Select
        allowClear
        placeholder="Semua upstream"
        style={{ minWidth: 220 }}
        value={filter.upstream}
        options={upstreams.map((i) => ({
          label: labels.get(i.id) ?? interfaceLabel(i),
          value: i.id,
        }))}
        onChange={(upstream: string | undefined) =>
          onChange({ ...filter, upstream })
        }
      />
    </Space>
  );
}
