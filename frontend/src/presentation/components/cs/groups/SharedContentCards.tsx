import { Typography } from "antd";
import { EnvironmentOutlined, UserOutlined } from "@ant-design/icons";
import type { WaGroupMessageExtra } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

const card = {
  display: "flex",
  gap: 10,
  alignItems: "flex-start",
  padding: "8px 10px",
  marginBottom: 4,
  borderRadius: 6,
  background: "rgba(255, 255, 255, 0.05)",
} as const;

/** Only an http(s) link is followed: the url comes from whoever shared the
 * place, and must not be able to put a script behind a click. */
function mapsLink(extra: WaGroupMessageExtra): string {
  if (extra.url && /^https?:\/\//i.test(extra.url)) return extra.url;
  return `https://maps.google.com/?q=${extra.latitude},${extra.longitude}`;
}

/** A shared place: its name and address, and the way to open it. */
export function LocationCard({ extra }: { extra: WaGroupMessageExtra }) {
  return (
    <div style={card}>
      <EnvironmentOutlined
        style={{ fontSize: 22, color: colors.danger, marginTop: 2 }}
      />
      <div style={{ minWidth: 0 }}>
        <Text strong style={{ display: "block", fontSize: 13 }}>
          {extra.name || "Lokasi"}
        </Text>
        {extra.address && (
          <Text type="secondary" style={{ display: "block", fontSize: 12 }}>
            {extra.address}
          </Text>
        )}
        <a
          href={mapsLink(extra)}
          target="_blank"
          rel="noopener noreferrer"
          style={{ color: colors.success, fontSize: 12 }}
        >
          Buka di Google Maps
        </a>
      </div>
    </div>
  );
}

/** One card per shared contact, each number copyable: the reason somebody
 * shares a contact in a work group is so the next person can call it. */
export function ContactCards({ extra }: { extra: WaGroupMessageExtra }) {
  return (
    <>
      {(extra.contacts ?? []).map((contact, i) => (
        <div key={`${contact.displayName}-${i}`} style={card}>
          <UserOutlined style={{ fontSize: 20, marginTop: 2 }} />
          <div style={{ minWidth: 0 }}>
            <Text strong style={{ display: "block", fontSize: 13 }}>
              {contact.displayName || "Kontak"}
            </Text>
            {contact.phones.map((phone) => (
              <Text
                key={phone}
                copyable={{ tooltips: ["Salin nomor", "Tersalin"] }}
                style={{ display: "block", fontSize: 12 }}
              >
                {phone}
              </Text>
            ))}
          </div>
        </div>
      ))}
    </>
  );
}
