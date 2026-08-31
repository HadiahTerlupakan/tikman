import { Alert, Collapse, Descriptions, Typography, theme } from "antd";
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

/**
 * gatewayFor guesses the OLT's default gateway as the first host of its /24.
 *
 * The address is a convention rather than something this system holds: nothing
 * records the router's address on an OLT's management VLAN. It is right in the
 * installations here and is offered as a filled-in suggestion, labelled as one,
 * because an operator correcting one octet beats an operator decoding a
 * placeholder.
 */
function gatewayFor(ipAddress?: string): string | null {
  const octets = ipAddress?.trim().split(".");
  const valid =
    octets?.length === 4 &&
    octets.every((octet) => /^\d{1,3}$/.test(octet) && Number(octet) <= 255);
  return valid ? `${octets[0]}.${octets[1]}.${octets[2]}.1` : null;
}

function CommandBlock({ testId, text }: { testId: string; text: string }) {
  // Colours come from the theme rather than being fixed: a hardcoded light
  // background left the commands as pale text on a pale block under the dark
  // theme, which is the one this system is actually used in. colorBgBase and
  // colorText are set explicitly by the theme, unlike the derived fill tokens,
  // which antd computes with the light algorithm here.
  const { token } = theme.useToken();

  return (
    <Typography.Paragraph copyable={{ text }} style={{ marginBottom: 0 }}>
      <pre
        data-testid={testId}
        style={{
          margin: 0,
          padding: 12,
          background: token.colorBgBase,
          color: token.colorText,
          border: `1px solid ${token.colorBorderSecondary}`,
          borderRadius: token.borderRadius,
          fontSize: 12,
          lineHeight: 1.6,
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
  const gateway = gatewayFor(ipAddress);

  const oltCommands = [
    `snmp-server host ${destination} version 2c ${community} enable NOTIFICATIONS ` +
      `target-addr-name EMS_${destination} isnmsserver udp-port ${TRAP_PORT} ` +
      `trap-report-compatibility v20`,
    ``,
    `! rute agar OLT bisa menjangkau ${destination}`,
    gateway
      ? `! ${gateway} adalah dugaan: host pertama di subnet OLT ini`
      : `! ganti <gateway-LAN> dengan alamat MikroTik di VLAN manajemen OLT ini`,
    `ip route ${tunnel.network} ${tunnel.mask} ${gateway ?? "<gateway-LAN>"}`,
  ].join("\n");

  // Only a site whose tunnel has never come up needs anything done on its
  // MikroTik. Where the tunnel is already carrying the poller's SNMP, it
  // carries the traps too, and adding a second wireguard interface would not
  // be a harmless repeat — it would be a second tunnel.
  const mikrotikCommands =
    sitePeer && !sitePeer.connected
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
                  <Typography.Text
                    copyable
                    style={{
                      fontSize: 12,
                      fontFamily: "monospace",
                      wordBreak: "break-all",
                    }}
                  >
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
              {mikrotikCommands && (
                <CommandBlock
                  testId="trap-setup-mikrotik"
                  text={mikrotikCommands}
                />
              )}
              {sitePeer?.connected && (
                <Alert
                  type="success"
                  showIcon
                  message="Tunnel site ini sudah aktif — tidak ada perintah MikroTik"
                  description="Trap lewat tunnel yang sama dengan polling SNMP, jadi jalurnya sudah terbuka. Menambah interface WireGuard lagi justru membuat tunnel kedua."
                />
              )}
              {!sitePeer && (
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
                message="Periksa baris ip route sebelum menempel"
                description="Perintah snmp-server dibaca dari OLT yang trap-nya sudah berjalan, jadi terbukti. Baris ip route tidak: sintaksnya disusun dari tabel rute chassis itu, bukan dari perintah aslinya, dan alamat gateway-nya dugaan dari konvensi subnet — bukan sesuatu yang sistem ini simpan."
              />
            </>
          ),
        },
      ]}
    />
  );
}
