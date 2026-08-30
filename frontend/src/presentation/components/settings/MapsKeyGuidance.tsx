import { Alert } from "antd";

/**
 * Permanent, not dismissible. The Maps key is delivered to every browser that
 * loads the map, so the only thing protecting it is a restriction set in Google
 * Cloud Console — and an operator who believes the key is secret will never go
 * and set one.
 */
export function MapsKeyGuidance() {
  return (
    <Alert
      type="warning"
      showIcon
      style={{ marginTop: 12 }}
      message="This key is visible to anyone who opens the map"
      description={
        <ol style={{ margin: "8px 0 0 18px", padding: 0 }}>
          <li>Google Cloud Console → APIs &amp; Services → Credentials</li>
          <li>Open the key → Application restrictions → Websites</li>
          <li>
            Add <code>https://noc.radpro.id/*</code>
          </li>
          <li>
            API restrictions → restrict to Maps JavaScript API and Places API
          </li>
        </ol>
      }
    />
  );
}
