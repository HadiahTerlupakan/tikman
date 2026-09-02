import { Alert, Col, Row, Skeleton } from "antd";
import { Link } from "react-router-dom";
import {
  useGoogleMapsKey,
  useOdcs,
  useOdps,
  useOlts,
} from "@/application/hooks";
import { PageHeader, DarkCard } from "../components/common";
// Imported from their own modules rather than the barrel so a test can mock
// the map without also mocking the panel beside it.
import { OltMap } from "../components/map/OltMap";
import { mappedOlts, unmappedOlts } from "../components/map/oltMapFilters";
import { UnmappedOltsPanel } from "../components/map/UnmappedOltsPanel";

export default function MapPage() {
  const { key, isLoading: keyLoading } = useGoogleMapsKey();
  const { data: olts, isLoading: oltsLoading } = useOlts();
  const { data: odcs } = useOdcs();
  const { data: odps } = useOdps();

  const unmapped = unmappedOlts(olts);

  return (
    <div>
      <PageHeader
        title="OLT Map"
        description={`${mappedOlts(olts).length} OLT · ${odcs?.length ?? 0} ODC · ${odps?.length ?? 0} ODP di peta`}
      />

      {keyLoading || oltsLoading ? (
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
              <OltMap
                apiKey={key}
                olts={olts ?? []}
                odcs={odcs ?? []}
                odps={odps ?? []}
              />
            </DarkCard>
          </Col>
          <Col xs={24} lg={7}>
            <UnmappedOltsPanel olts={unmapped} />
          </Col>
        </Row>
      )}
    </div>
  );
}
