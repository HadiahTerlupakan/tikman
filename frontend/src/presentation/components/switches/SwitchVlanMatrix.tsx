import { useState, type CSSProperties } from "react";
import { theme } from "antd";
import type { SwitchInterface, SwitchVlan } from "@/domain/entities";
import { vlanMembership, type VlanMembership } from "./switchVlans";
import {
  cleanLabel,
  vlanLabel,
  type MatrixColumn,
  type PortGroup,
} from "./vlanMatrix";
import "./vlanMatrix.css";

const MEMBERSHIP_TEXT: Record<VlanMembership, string> = {
  T: "tagged",
  U: "untagged",
  "": "bukan anggota",
};

interface Hot {
  vid: number | null;
  port: string | null;
}
const COLD: Hot = { vid: null, port: null };

type Column = MatrixColumn & { groupStart: boolean };
type OnHover = (hot: Hot) => void;

interface Props {
  vlans: SwitchVlan[];
  groups: PortGroup[];
  /** Every port that switches VLANs, hidden or not, for the member count. */
  countPorts: SwitchInterface[];
}

export function SwitchVlanMatrix({ vlans, groups, countPorts }: Props) {
  const { token } = theme.useToken();
  const [hot, setHot] = useState<Hot>(COLD);
  const columns: Column[] = groups.flatMap((g, gi) =>
    g.ports.map((c, ci) => ({ ...c, groupStart: gi > 0 && ci === 0 })),
  );
  // The theme sets a dark colorBgBase without darkAlgorithm, so the antd fill
  // and preset colours come out of the light palette; derive what this needs
  // from the base text colour and fixed dark shades instead.
  const vars = {
    "--vm-bg": token.colorBgContainer,
    "--vm-border": token.colorBorderSecondary,
    "--vm-divider": token.colorBorder,
    "--vm-text": token.colorText,
    "--vm-muted": token.colorTextTertiary,
    "--vm-tagged": token.blue7,
    "--vm-untagged": token.green8,
    "--vm-up": token.colorSuccess,
    "--vm-down": token.colorTextQuaternary,
  } as CSSProperties;

  return (
    <div style={vars}>
      <div className="vm-scroll" onMouseLeave={() => setHot(COLD)}>
        <table data-testid="vlan-matrix" className="vm-table">
          <MatrixHead
            groups={groups}
            columns={columns}
            hot={hot}
            onHover={setHot}
          />
          <tbody>
            {vlans.map((v) => (
              <MatrixRow
                key={v.vid}
                vlan={v}
                columns={columns}
                members={countPorts.filter((p) => vlanMembership(p, v.vid))}
                hot={hot}
                onHover={setHot}
              />
            ))}
          </tbody>
        </table>
      </div>
      <div className="vm-legend">
        <span className="vm-chip" data-m="T">
          T
        </span>
        tagged
        <span className="vm-chip" data-m="U">
          U
        </span>
        untagged
        <span className="vm-port-dot" data-up="true" />
        port up
      </div>
    </div>
  );
}

// A CE6850 has one slot, so "1/0/" says nothing; the port type is enough and
// the full name is in each port header's label.
const SLOT_PATH = /(\d+\/)+$/;
const groupLabel = (prefix: string) =>
  prefix.replace(SLOT_PATH, "") || "Lainnya";

const groupStartClass = (base: string, start: boolean) =>
  start ? `${base} vm-group-start` : base;

const portTitle = (port: SwitchInterface) =>
  [port.name, cleanLabel(port.alias), port.operStatus === "up" ? "up" : "down"]
    .filter(Boolean)
    .join(" · ");

interface HeadProps {
  groups: PortGroup[];
  columns: Column[];
  hot: Hot;
  onHover: OnHover;
}

function MatrixHead({ groups, columns, hot, onHover }: HeadProps) {
  return (
    <thead onMouseEnter={() => onHover(COLD)}>
      <tr>
        <th scope="col" className="vm-vlan" rowSpan={2}>
          VLAN
        </th>
        {groups.map((g, gi) => (
          <th
            key={g.prefix || "other"}
            scope="colgroup"
            className={groupStartClass("vm-group", gi > 0)}
            colSpan={g.ports.length}
          >
            {groupLabel(g.prefix)}
          </th>
        ))}
        <th scope="col" className="vm-count" rowSpan={2}>
          Port
        </th>
      </tr>
      <tr>
        {columns.map(({ port, short, groupStart }) => (
          <th
            key={port.name}
            scope="col"
            className={groupStartClass("vm-port", groupStart)}
            data-col-hot={hot.port === port.name}
            title={portTitle(port)}
            aria-label={portTitle(port)}
            onMouseEnter={(e) => {
              e.stopPropagation();
              onHover({ vid: null, port: port.name });
            }}
          >
            <span className="vm-port-dot" data-up={port.operStatus === "up"} />
            {short}
          </th>
        ))}
      </tr>
    </thead>
  );
}

interface RowProps {
  vlan: SwitchVlan;
  columns: Column[];
  members: SwitchInterface[];
  hot: Hot;
  onHover: OnHover;
}

function MatrixRow({ vlan, columns, members, hot, onHover }: RowProps) {
  const name = vlanLabel(vlan);
  const vlanText = `VLAN ${vlan.vid}${name ? ` ${name}` : ""}`;
  const rowHover = () => onHover({ vid: vlan.vid, port: null });
  return (
    <tr
      data-hot={hot.vid === vlan.vid}
      className={members.length === 0 ? "vm-empty-row" : undefined}
    >
      <th
        scope="row"
        className="vm-vlan"
        title={name || undefined}
        onMouseEnter={rowHover}
      >
        <span className="vm-vid">{vlan.vid}</span>
        <span className="vm-name">{name}</span>
      </th>
      {columns.map(({ port, groupStart }) => {
        const m = vlanMembership(port, vlan.vid);
        const text = `${vlanText} · ${port.name} · ${MEMBERSHIP_TEXT[m]}`;
        return (
          <td
            key={port.name}
            data-testid={`cell-${vlan.vid}-${port.name}`}
            className={groupStartClass("vm-cell", groupStart)}
            data-col-hot={hot.port === port.name}
            title={text}
            aria-label={text}
            onMouseEnter={() => onHover({ vid: vlan.vid, port: port.name })}
          >
            <span className="vm-chip" data-m={m} aria-hidden="true">
              {m}
            </span>
          </td>
        );
      })}
      <td className="vm-count" onMouseEnter={rowHover}>
        {members.length}
      </td>
    </tr>
  );
}
