import { Link } from "react-router-dom";
import type { Olt } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { DarkCard } from "../common";

interface UnmappedOltsPanelProps {
  olts: Olt[];
}

/**
 * OLTs the map cannot draw. Without this the page quietly lies: two pins for
 * three OLTs reads as complete, and an operator concludes everything is
 * mapped. An empty result and an unknown result must not look alike.
 */
export function UnmappedOltsPanel({ olts }: UnmappedOltsPanelProps) {
  return (
    <DarkCard title="Not on the map" style={{ height: "100%" }}>
      {olts.length === 0 ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          Every OLT is on the map.
        </div>
      ) : (
        <>
          <div style={{ color: colors.textSecondary, fontSize: 12 }}>
            {olts.length === 1
              ? "1 OLT has no coordinates"
              : `${olts.length} OLTs have no coordinates`}
          </div>
          <div style={{ marginTop: 12 }}>
            {olts.map((olt, index) => (
              <div
                key={olt.id}
                style={{
                  padding: "8px 0",
                  borderTop:
                    index === 0 ? "none" : `1px solid ${colors.border}`,
                }}
              >
                <div style={{ color: colors.textBody, fontSize: 13 }}>
                  {olt.name}
                </div>
                <div style={{ color: colors.textMuted, fontSize: 11 }}>
                  {olt.siteName} · {olt.ipAddress}
                </div>
              </div>
            ))}
          </div>
          <div style={{ marginTop: 12, fontSize: 12 }}>
            <Link to="/olts">Add coordinates on the OLTs page</Link>
          </div>
        </>
      )}
    </DarkCard>
  );
}
