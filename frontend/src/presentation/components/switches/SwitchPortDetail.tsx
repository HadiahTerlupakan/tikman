import { Descriptions } from "antd";
import type { SwitchInterface } from "@/domain/entities";
import { formatSpeed } from "../trafficFormat";
import { STATE_TEXT } from "../routers/faceplate/portModel";
import { formatBps } from "../routers/routerTraffic";
import { portState, stpText, vlanText } from "./switchPorts";

const dash = (value: string | number | null | undefined) =>
  value === null || value === undefined || value === "" ? "—" : value;

export function SwitchPortDetail({ port }: { port: SwitchInterface }) {
  return (
    <Descriptions size="small" column={2} bordered title={port.name}>
      <Descriptions.Item label="Status">
        {STATE_TEXT[portState(port)]}
      </Descriptions.Item>
      <Descriptions.Item label="Kecepatan">
        {port.speedMbps ? formatSpeed(port.speedMbps) : "—"}
      </Descriptions.Item>
      <Descriptions.Item label="Mode">{dash(port.mode)}</Descriptions.Item>
      <Descriptions.Item label="VLAN">{vlanText(port)}</Descriptions.Item>
      <Descriptions.Item label="STP edge">
        {stpText(port.stpEdge)}
      </Descriptions.Item>
      <Descriptions.Item label="Alias">{dash(port.alias)}</Descriptions.Item>
      <Descriptions.Item label="Masuk (rx)">
        {formatBps(port.lastInBps)}
      </Descriptions.Item>
      <Descriptions.Item label="Keluar (tx)">
        {formatBps(port.lastOutBps)}
      </Descriptions.Item>
      <Descriptions.Item label="Error masuk / keluar">
        {`${dash(port.inErrors)} / ${dash(port.outErrors)}`}
      </Descriptions.Item>
      <Descriptions.Item label="Buang masuk">
        {dash(port.inDiscards)}
      </Descriptions.Item>
    </Descriptions>
  );
}
