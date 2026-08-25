import { Alert, Typography } from "antd";
import type { ZteGPONRegisterRequest } from "@/domain/entities";

interface ZteCommandPreviewProps {
  request: Partial<ZteGPONRegisterRequest>;
  onuId: number;
}

export function ZteCommandPreview({ request, onuId }: ZteCommandPreviewProps) {
  const serial = request.serialNumber?.trim().toUpperCase() || "<serial>";
  const profile = request.downloadProfile || "<profile>";
  const vlan = request.vlanId || "<vlan>";
  const username = request.pppoeUsername || "<username>";
  const commands = [
    "configure terminal",
    `interface gpon-olt_1/${request.card || "<card>"}/${request.pon || "<pon>"}`,
    `onu ${onuId} type ${request.onuType || "<onu-type>"} sn ${serial}`,
    "exit",
    `interface gpon-onu_1/${request.card || "<card>"}/${request.pon || "<pon>"}:${onuId}`,
    ...(request.name ? [`name ${request.name}`] : []),
    `tcont 1 name internet profile-name ${profile}`,
    "gemport 1 name internet tcont 1",
    `service-port 1 vport 1 user-vlan ${vlan} vlan ${vlan}`,
    `wan-ip 1 mode pppoe username ${username} password <redacted> vlan-profile ${request.vlanProfile || "<profile>"}`,
    "exit",
    "commit",
  ];

  return (
    <Alert
      type="info"
      showIcon
      message="Command preview (password redacted)"
      description={
        <Typography.Paragraph copyable={{ text: commands.join("\n") }}>
          <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>
            {commands.join("\n")}
          </pre>
        </Typography.Paragraph>
      }
    />
  );
}
