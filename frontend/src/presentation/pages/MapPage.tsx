import { Alert, Col, Row, Skeleton } from "antd";
import { Link } from "react-router-dom";
import { useGoogleMapsKey, useOlts, useSites } from "@/application/hooks";
import { PageHeader, DarkCard } from "../components/common";
// Imported from their own modules rather than the barrel so a test can mock
// the map without also mocking the panel beside it.
import { SiteMap, mappedSites, unmappedSites } from "../components/map/SiteMap";
import { UnmappedSitesPanel } from "../components/map/UnmappedSitesPanel";

export default function MapPage() {
  const { key, isLoading: keyLoading } = useGoogleMapsKey();
  const { data: sites, isLoading: sitesLoading } = useSites();
  const { data: olts } = useOlts();

  const unmapped = unmappedSites(sites);

  return (
    <div>
      <PageHeader
        title="Site Map"
        description={`${mappedSites(sites).length} sites on the map`}
      />

      {keyLoading || sitesLoading ? (
        <Skeleton active paragraph={{ rows: 8 }} title={false} />
      ) : !key ? (
        <Alert
          type="info"
          showIcon
          message="No Google Maps API key is configured"
          description={
            <span>
              The map needs a key before it can draw anything. Add one under{" "}
              <Link to="/settings">Settings</Link>.
            </span>
          }
        />
      ) : (
        <Row gutter={[16, 16]}>
          <Col xs={24} lg={17}>
            <DarkCard style={{ height: "100%" }}>
              <SiteMap apiKey={key} sites={sites ?? []} olts={olts ?? []} />
            </DarkCard>
          </Col>
          <Col xs={24} lg={7}>
            <UnmappedSitesPanel sites={unmapped} />
          </Col>
        </Row>
      )}
    </div>
  );
}
