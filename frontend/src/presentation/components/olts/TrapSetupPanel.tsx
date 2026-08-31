import { Alert, Collapse, Descriptions, Typography } from "antd";
import { useWireguardPeers, useWireguardServer } from "@/application/hooks";

const TRAP_PORT = 162;
const MIKROTIK_INTERFACE = "wg-tikman";
const MIKROTIK_LISTEN_PORT = 13231;
const DEFAULT_COMMUNITY = "public";

interface TrapSetupPanelProps {
  siteId?: string;
  ipAddress?: string;
  snmpCommunity?: string;
}

/** netmaskOf renders a CIDR prefix as the dotted mask ZTE's CLI expects. */
function netmaskOf(prefixLength: number): string {
  const bits = 0xffffffff - (2 ** (32 - prefixLength) - 1);
  return [24, 16, 8, 0].map((shift) => (bits >>> shift) & 0xff).join(".");
}

function splitCidr(cidr: string): { network: string; mask: string } {
  const [network, prefix] = cidr.split("/");
  return { network, mask: netmaskOf(Number(prefix)) };
}

function CommandBlock({ testId, text }: { testId: string; text: string }) {
  return (
    <Typography.Paragraph copyable={{ text }} style={{ marginBottom: 0 }}>
      <pre
        data-testid={testId}
        style={{
          margin: 0,
          padding: 12,
          background: "#f5f5f5",
          borderRadius: 4,
          fontSize: 12,
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}
      >
        {text}
      </pre>
    </Typography.Paragraph>
  );
}

/**
 * TrapSetupPanel shows what has to be configured on the OLT and on the site's
 * MikroTik for this OLT's traps to reach TikMan, filled in with the values this
 * installation actually uses rather than placeholders an operator has to edit.
 *
 * A trap goes OLT -> site MikroTik -> WireGuard tunnel -> the receiver, so the
 * destination is the server's tunnel address and never a public one. Every value
 * here is read from the VPN server record and the site's own peer.
 */
export function TrapSetupPanel({
  siteId,
  ipAddress,
  snmpCommunity,
}: TrapSetupPanelProps) {
  const { data: server } = useWireguardServer();
  const { data: peers } = useWireguardPeers();

  if (!server) return null;

  const community = snmpCommunity?.trim() || DEFAULT_COMMUNITY;
  const destination = server.address;
  const sitePeer = peers?.find((peer) => peer.siteId === siteId);
  const tunnel = splitCidr(server.tunnelSubnet);

  const oltCommands = [
    `snmp-server host ${destination} version 2c ${community} enable NOTIFICATIONS ` +
      `target-addr-name EMS_${destination} isnmsserver udp-port ${TRAP_PORT} ` +
      `trap-report-compatibility v20`,
    ``,
    `! rute agar OLT bisa menjangkau ${destination}; ganti <gateway-LAN> dengan`,
    `! alamat MikroTik di VLAN manajemen OLT ini`,
    `ip route ${tunnel.network} ${tunnel.mask} <gateway-LAN>`,
  ].join("\n");

  const mikrotikCommands = sitePeer
    ? [
        `/interface/wireguard/add name=${MIKROTIK_INTERFACE} listen-port=${MIKROTIK_LISTEN_PORT}`,
        `/ip/address/add address=${sitePeer.tunnelAddress}/${server.tunnelSubnet.split("/")[1]} interface=${MIKROTIK_INTERFACE}`,
        `/interface/wireguard/peers/add interface=${MIKROTIK_INTERFACE} \\`,
        `  public-key="${server.publicKey}" \\`,
        `  endpoint-address=${server.endpointHost} endpoint-port=${server.listenPort} \\`,
        `  allowed-address=${server.tunnelSubnet} persistent-keepalive=25s`,
      ].join("\n")
    : null;

  return (
    <Collapse
      style={{ marginBottom: 16 }}
      items={[
        {
          key: "trap",
          forceRender: true,
          label: "Konfigurasi Trap (SNMP) — perintah siap salin",
          children: (
            <>
              <Descriptions
                size="small"
                column={1}
                bordered
                style={{ marginBottom: 12 }}
              >
                <Descriptions.Item label="Tujuan trap">
                  {destination}:{TRAP_PORT}
                </Descriptions.Item>
                <Descriptions.Item label="Community">
                  {community}
                </Descriptions.Item>
                <Descriptions.Item label="Endpoint server">
                  {server.endpointHost}:{server.listenPort}
                </Descriptions.Item>
                <Descriptions.Item label="Public key server">
                  <Typography.Text copyable code style={{ fontSize: 12 }}>
                    {server.publicKey}
                  </Typography.Text>
                </Descriptions.Item>
                <Descriptions.Item label="AllowedIPs di sisi server">
                  {ipAddress ? `${ipAddress}/32` : "isi IP OLT dulu"}
                </Descriptions.Item>
              </Descriptions>

              <Typography.Text strong>Di OLT (ZTE)</Typography.Text>
              <CommandBlock testId="trap-setup-olt" text={oltCommands} />

              <Typography.Text
                strong
                style={{ display: "block", marginTop: 12 }}
              >
                Di MikroTik site
              </Typography.Text>
              {mikrotikCommands ? (
                <CommandBlock
                  testId="trap-setup-mikrotik"
                  text={mikrotikCommands}
                />
              ) : (
                <Alert
                  type="info"
                  showIcon
                  message="Site ini belum punya peer VPN"
                  description={
                    // Each site needs its own address inside the tunnel subnet;
                    // printing a guessed one would collide with whichever site
                    // already holds it.
                    "Buat peer untuk site ini di halaman VPN lebih dulu. Alamat tunnel-nya dipakai di perintah MikroTik."
                  }
                />
              )}

              <Alert
                type="warning"
                showIcon
                style={{ marginTop: 12 }}
                message="Baris ip route belum diverifikasi terhadap firmware"
                description="Perintah snmp-server di atas dibaca dari OLT yang trap-nya sudah berjalan. Baris ip route disusun dari tabel rutenya, bukan dari perintah aslinya — periksa sintaksnya di firmware Anda sebelum menempel."
              />
            </>
          ),
        },
      ]}
    />
  );
}
