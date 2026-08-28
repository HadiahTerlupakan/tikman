import { useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Button,
  Empty,
  Space,
  Spin,
  Table,
  Tabs,
  Typography,
} from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import {
  useOlt,
  useOltOnuTypes,
  useOltSystem,
  useOltTcontProfiles,
  useOltVlanProfiles,
  useOltVlans,
} from "@/application/hooks/useOlts";
import { useOnts } from "@/application/hooks/useOnts";
import { OltChassisTable } from "../components/olts/config/OltChassisTable";
import { OltConfigHeader } from "../components/olts/config/OltConfigHeader";
import { OltPortGrid } from "../components/olts/config/OltPortGrid";
import { OltProfileList } from "../components/olts/config/OltProfileList";

const { Title, Text } = Typography;

export default function OltConfigurationPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();

  const { data: olt } = useOlt(id);
  const { data: snapshot, isLoading } = useOltSystem(id);
  const { data: vlans } = useOltVlans(id);
  const { data: onuTypes } = useOltOnuTypes(id);
  const { data: vlanProfiles } = useOltVlanProfiles(id);
  const { data: tcontProfiles } = useOltTcontProfiles(id);
  // 500 is the list endpoint's ceiling; the header says so when an OLT
  // carries more ONUs than one page holds.
  const { data: ontPage } = useOnts({ oltId: id, limit: 500 });

  const onts = useMemo(() => ontPage?.data ?? [], [ontPage]);
  const ports = snapshot?.ports ?? [];

  const cardType = useMemo(() => {
    const bySlot = new Map(
      (snapshot?.cards ?? []).map((card) => [card.slot, card.type]),
    );
    return (slot: number) => bySlot.get(slot);
  }, [snapshot?.cards]);

  const label = (prefix: string) => (slot: number) => {
    const type = cardType(slot);
    return type ? `${prefix} ${slot} · ${type}` : `${prefix} ${slot}`;
  };

  if (isLoading) {
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <Spin />
      </div>
    );
  }

  return (
    <div>
      <Space style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/olts")}>
          Back
        </Button>
        <Title level={4} style={{ margin: 0 }}>
          {olt?.name ?? "OLT"} configuration
        </Title>
      </Space>

      <Alert
        type="info"
        showIcon
        message="Read-only view"
        description="Everything here is read from the OLT by the discovery poll, over SNMP where the device supports it. This page does not send any command to the OLT."
        style={{ marginBottom: 16 }}
      />

      <OltConfigHeader
        snapshot={snapshot}
        onts={onts}
        totalOnts={ontPage?.total ?? onts.length}
      />

      <Tabs
        items={[
          {
            key: "uplinks",
            label: "Uplinks",
            children: (
              <OltPortGrid
                ports={ports}
                kind="uplink"
                cardLabel={label("Slot")}
                emptyText="No uplink ports reported by the last poll"
              />
            ),
          },
          {
            key: "pon",
            label: "PON cards",
            children: (
              <OltPortGrid
                ports={ports}
                kind="pon"
                cardLabel={label("Card")}
                emptyText="No PON ports reported by the last poll"
              />
            ),
          },
          {
            key: "vlans",
            label: "VLANs",
            children:
              (vlans ?? []).length === 0 ? (
                <Empty description="No VLANs read from the OLT yet" />
              ) : (
                <Table
                  size="small"
                  rowKey="vlanId"
                  dataSource={vlans}
                  pagination={false}
                  columns={[
                    { title: "VLAN ID", dataIndex: "vlanId", width: 120 },
                    { title: "Name", dataIndex: "name" },
                  ]}
                />
              ),
          },
          {
            key: "onu-types",
            label: "ONU types",
            children: (
              <OltProfileList
                title="ONU type"
                names={onuTypes ?? []}
                emptyText="No ONU types read from the OLT yet"
                note="These are the names the OLT accepts in a registration command, not the models ONUs announce over OMCI."
              />
            ),
          },
          {
            key: "wan-ip",
            label: "WAN-IP profiles",
            children: (
              <OltProfileList
                title="VLAN profile"
                names={vlanProfiles ?? []}
                emptyText="No VLAN profiles in use on this OLT"
                note="Recovered from the ONU configurations, because the CLI has no command that lists them."
              />
            ),
          },
          {
            key: "speed",
            label: "Speed profiles",
            children: (
              <OltProfileList
                title="T-CONT profile"
                names={tcontProfiles ?? []}
                emptyText="No T-CONT profiles read from the OLT yet"
              />
            ),
          },
          {
            key: "system",
            label: "System",
            children: (
              <Space
                direction="vertical"
                size="middle"
                style={{ width: "100%" }}
              >
                <Text type="secondary">
                  {snapshot?.system?.description ||
                    "No chassis description read yet"}
                </Text>
                <OltChassisTable entities={snapshot?.system?.entities ?? []} />
                <Text type="secondary">
                  Last read from the OLT:{" "}
                  {snapshot?.updatedAt
                    ? new Date(snapshot.updatedAt).toLocaleString()
                    : "never"}
                </Text>
              </Space>
            ),
          },
        ]}
      />
    </div>
  );
}
