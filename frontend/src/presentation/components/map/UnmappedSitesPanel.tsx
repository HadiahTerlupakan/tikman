import { Link } from "react-router-dom";
import type { Site } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { DarkCard } from "../common";

interface UnmappedSitesPanelProps {
  sites: Site[];
}

/**
 * Sites the map cannot draw. Without this the page quietly lies: two pins for
 * three sites reads as complete, and an operator concludes everything is
 * mapped. An empty result and an unknown result must not look alike.
 */
export function UnmappedSitesPanel({ sites }: UnmappedSitesPanelProps) {
  return (
    <DarkCard title="Not on the map" style={{ height: "100%" }}>
      {sites.length === 0 ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          Every site is on the map.
        </div>
      ) : (
        <>
          <div style={{ color: colors.textSecondary, fontSize: 12 }}>
            {sites.length === 1
              ? "1 site has no coordinates"
              : `${sites.length} sites have no coordinates`}
          </div>
          <div style={{ marginTop: 12 }}>
            {sites.map((site, index) => (
              <div
                key={site.id}
                style={{
                  padding: "8px 0",
                  borderTop:
                    index === 0 ? "none" : `1px solid ${colors.border}`,
                }}
              >
                <div style={{ color: colors.textBody, fontSize: 13 }}>
                  {site.name}
                </div>
                <div style={{ color: colors.textMuted, fontSize: 11 }}>
                  {site.location || "no address"}
                </div>
              </div>
            ))}
          </div>
          <div style={{ marginTop: 12, fontSize: 12 }}>
            <Link to="/sites">Add coordinates on the Sites page</Link>
          </div>
        </>
      )}
    </DarkCard>
  );
}
