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
  // Counted over `team`, not over `online`: someone present but holding a role
  // this panel refuses to list would otherwise push the count past the total.
  const hereCount = team.filter((u) => here.has(u.id)).length;

  return (
    <div style={{ padding: 14 }}>
      {/* A count rather than a legend. It answers the question a CS actually
          has — how many of us are here — and in answering it explains the
          dots, which a sentence spelling out the colours did not. */}
      <Text style={{ color: colors.textMuted, fontSize: 11 }}>
        TIM CS · {hereCount} dari {team.length} di inbox
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
                  boxSizing: "border-box",
                  // Filled when present, a hollow ring when not. A filled dot
                  // in the border colour vanished against this background, so
                  // an absent row read as a name with no status at all.
                  background: isHere ? colors.success : "transparent",
                  border: isHere ? "none" : `1px solid ${colors.textMuted}`,
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
