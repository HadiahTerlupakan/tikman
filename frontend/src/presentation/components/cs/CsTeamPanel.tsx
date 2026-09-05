import { Typography } from "antd";
import { UserRole, type User } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

/** The roles the API lets into /api/v1/cs — anyone else can never take a
 * thread, so offering them here would be an invitation that goes nowhere. */
const INBOX_ROLES: UserRole[] = [
  UserRole.ADMIN,
  UserRole.CS,
  UserRole.TECHNICIAN,
];

interface CsTeamPanelProps {
  users: User[];
  /** Ids of whoever currently has the inbox open. */
  online: string[];
  currentUserId?: string;
}

/**
 * Who else is staffing the inbox.
 *
 * "Online" here is narrower than logged in: a browser claims presence only
 * while the CS Inbox route itself is open, so a technician reading the OLT map
 * is absent by design. The wording says "di inbox" rather than "online" so the
 * panel does not promise more than it knows.
 *
 * Ordered by name, never by status. Presence changes on its own whenever a
 * browser sleeps, and a list sorted by it would reshuffle under the reader.
 */
export function CsTeamPanel({
  users,
  online,
  currentUserId,
}: CsTeamPanelProps) {
  const here = new Set(online);
  const team = users
    .filter((u) => INBOX_ROLES.includes(u.role))
    .sort((a, b) => a.username.localeCompare(b.username, "id"));

  return (
    <div style={{ padding: 14 }}>
      <Text style={{ color: colors.textMuted, fontSize: 11 }}>
        TIM CS · titik hijau berarti sedang membuka inbox
      </Text>

      <ul
        style={{
          listStyle: "none",
          margin: "10px 0 0",
          padding: 0,
          display: "flex",
          flexDirection: "column",
          gap: 8,
        }}
      >
        {team.map((u) => {
          const isHere = here.has(u.id);
          return (
            <li
              key={u.id}
              aria-label={`${u.username} — ${
                isHere ? "sedang di inbox" : "sedang tidak di inbox"
              }`}
              style={{ display: "flex", alignItems: "center", gap: 8 }}
            >
              <span
                aria-hidden
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  flexShrink: 0,
                  background: isHere ? colors.success : colors.border,
                }}
              />
              <Text
                style={{
                  color: isHere ? colors.textPrimary : colors.textMuted,
                  fontSize: 13,
                }}
              >
                {u.username}
              </Text>
              {u.id === currentUserId && (
                <Text style={{ color: colors.textMuted, fontSize: 11 }}>
                  (Anda)
                </Text>
              )}
            </li>
          );
        })}
      </ul>
    </div>
  );
}
