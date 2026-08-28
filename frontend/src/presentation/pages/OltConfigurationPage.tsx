import { useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  App,
  Alert,
  Button,
  Space,
  Spin,
  Tabs,
  Tooltip,
  Typography,
} from "antd";
import { ArrowLeftOutlined, ReloadOutlined } from "@ant-design/icons";
import {
  useOlt,
  useOltSystem,
  useOltVlanProfiles,
  useOltVlans,
  useRefreshOltSystem,
} from "@/application/hooks/useOlts";
import { useOnts } from "@/application/hooks/useOnts";
import { OltChassisTable } from "../components/olts/config/OltChassisTable";
import { OltConfigHeader } from "../components/olts/config/OltConfigHeader";
import { OltOnuTypeTable } from "../components/olts/config/OltOnuTypeTable";
import { OltPortGrid } from "../components/olts/config/OltPortGrid";
import { OltProfileList } from "../components/olts/config/OltProfileList";
import { OltSpeedTable } from "../components/olts/config/OltSpeedTable";
import { OltVlanTable } from "../components/olts/config/OltVlanTable";

const { Title, Text } = Typography;

export default function OltConfigurationPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const { message } = App.useApp();

  const { data: olt } = useOlt(id);
  const { data: snapshot, isLoading } = useOltSystem(id);
  const { data: vlans } = useOltVlans(id);
  const { data: vlanProfiles } = useOltVlanProfiles(id);
  const { data: ontPage } = useOnts({ oltId: id, limit: 500 });
  const refresh = useRefreshOltSystem(id);

  const onts = useMemo(() => ontPage?.data ?? [], [ontPage]);
  const ports = snapshot?.ports ?? [];
  const cardHealth = snapshot?.cardHealth ?? [];

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

  const onRefresh = () => {
    refresh.mutate(undefined, {
      onSuccess: () => message.success("Re-read from the OLT"),
      onError: (error: Error) => message.error(error.message),
    });
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
      <Space style={{ marginBottom: 16 }} wrap>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate("/olts")}>
          Back
        </Button>
        <Title level={4} style={{ margin: 0 }}>
          {olt?.name ?? "OLT"} configuration
        </Title>
        <Tooltip title="Re-reads the chassis, ports, card health and VLANs over SNMP. The profile lists come from a CLI read that runs on its own schedule.">
          <Button
            icon={<ReloadOutlined />}
            onClick={onRefresh}
            loading={refresh.isPending}
          >
            Refresh now
          </Button>
        </Tooltip>
      </Space>

      <Alert
        type="info"
        showIcon
        message="Read-only view"
        description="Everything here is read from the OLT, over SNMP wherever the device supports it. This page does not send any command to the OLT."
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
                cardHealth={cardHealth}
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
                cardHealth={cardHealth}
                cardLabel={label("Card")}
                emptyText="No PON ports reported by the last poll"
              />
            ),
          },
          {
            key: "vlans",
            label: "VLANs",
            children: <OltVlanTable vlans={vlans ?? []} />,
          },
          {
            key: "onu-types",
            label: "ONU types",
            children: <OltOnuTypeTable types={snapshot?.onuTypes ?? []} />,
          },
          {
            key: "wan-ip",
            label: "WAN-IP profiles",
            children: (
              <OltProfileList
                title="VLAN profile"
                names={vlanProfiles ?? []}
                emptyText="No VLAN profiles in use on this OLT"
                note="Recovered from the ONU configurations, because the CLI has no command that lists them. The C300 exposes no CVLAN for these."
              />
            ),
          },
          {
            key: "speed",
            label: "Speed profiles",
            children: (
              <OltSpeedTable profiles={snapshot?.speedProfiles ?? []} />
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
