import { useState } from "react";
import { Alert, Button, Col, Row, Skeleton, Space, Tag } from "antd";
import { Link } from "react-router-dom";
import {
  useGoogleMapsKey,
  useOdcFeeds,
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
import {
  PlantFormModal,
  type PlantKind,
} from "../components/map/PlantFormModal";
import type { Coordinates } from "../components/map/plantForm";
import { cableSegments } from "../components/map/cableSegments";
import { CableEditor } from "../components/map/CableEditor";
import { useCableEditing } from "../components/map/useCableEditing";

export default function MapPage() {
  const { key, isLoading: keyLoading } = useGoogleMapsKey();
  const { data: olts, isLoading: oltsLoading } = useOlts();
  const { data: odcs } = useOdcs();
  const { data: odps } = useOdps();
  const { data: feeds } = useOdcFeeds();
  const [placing, setPlacing] = useState<PlantKind | null>(null);
  const [placed, setPlaced] = useState<Coordinates>();
  const cable = useCableEditing();
  const cables = cableSegments(olts, odcs, odps, feeds);

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
              {cable.selected ? (
                <CableEditor
                  segment={cable.selected}
                  drafting={cable.drafting}
                  draftCount={cable.drawn.length}
                  saving={cable.saving}
                  onStartDraw={cable.startDraw}
                  onSave={cable.saveDraft}
                  onCancel={cable.close}
                  onStraighten={cable.straighten}
                />
              ) : (
                <Space wrap style={{ marginBottom: 12 }}>
                  <Button onClick={() => setPlacing("odc")}>Tambah ODC</Button>
                  <Button onClick={() => setPlacing("odp")}>Tambah ODP</Button>
                  {placing && (
                    <Tag color="green">
                      Klik di peta untuk menaruh {placing.toUpperCase()}
                    </Tag>
                  )}
                </Space>
              )}
              <OltMap
                apiKey={key}
                olts={olts ?? []}
                odcs={odcs ?? []}
                odps={odps ?? []}
                cables={cables}
                selectedCableId={cable.selected?.id}
                onSelectCable={cable.select}
                draft={cable.draftSegment()}
                // One click, three possible meanings, in the order that keeps
                // each mode from stealing the other's clicks.
                onPlace={(coordinates) => {
                  if (cable.drafting) {
                    cable.addPoint({
                      lat: coordinates.latitude,
                      lng: coordinates.longitude,
                    });
                    return;
                  }
                  if (placing) {
                    setPlaced(coordinates);
                  }
                }}
              />
            </DarkCard>
          </Col>
          <Col xs={24} lg={7}>
            <UnmappedOltsPanel olts={unmapped} />
          </Col>
        </Row>
      )}

      <PlantFormModal
        open={!!placing && !!placed}
        kind={placing ?? "odc"}
        coordinates={placed}
        onClose={() => {
          setPlacing(null);
          setPlaced(undefined);
        }}
      />
    </div>
  );
}
