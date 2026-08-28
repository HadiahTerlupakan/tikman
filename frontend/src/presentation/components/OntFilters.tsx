import { Space, Select, Button, Input } from "antd";
import { ReloadOutlined, SearchOutlined } from "@ant-design/icons";
import type { OntStatus } from "@/domain/entities";
import { ONT_STATUSES } from "./ontStatus";
import { ontPositionLabel } from "./ontAddress";

const { Option } = Select;

interface GponPortEntity {
  portId: number;
  onts: Array<{
    portId: number;
    ontId: number;
    serialNumber: string;
    runState: number;
    name?: string;
    description?: string;
    rxPower?: number | null;
    txPower?: number | null;
    distance?: number;
  }>;
}

interface GPONSlot {
  slot: number;
  ports: GponPortEntity[];
}

interface OntFiltersProps {
  oltsData: Array<{ id: string; name: string; ipAddress: string }>;
  selectedOltId: string | undefined;
  setSelectedOltId: (id: string | undefined) => void;
  selectedSlotId: number | undefined;
  setSelectedSlotId: (id: number | undefined) => void;
  selectedPortId: number | undefined;
  setSelectedPortId: (id: number | undefined) => void;
  topologyData: GPONSlot[];
  isLoadingTopology?: boolean;
  searchText: string;
  setSearchText: (text: string) => void;
  statusFilter: OntStatus | undefined;
  setStatusFilter: (status: OntStatus | undefined) => void;
  onReset: () => void;
}

export function OntFilters({
  oltsData,
  selectedOltId,
  setSelectedOltId,
  selectedSlotId,
  setSelectedSlotId,
  selectedPortId,
  setSelectedPortId,
  topologyData,
  isLoadingTopology,
  searchText,
  setSearchText,
  statusFilter,
  setStatusFilter,
  onReset,
}: OntFiltersProps) {
  // Every fitted card is offered, including one carrying no ONU yet. Hiding
  // those made a card that is installed and detected look absent.
  const cards = topologyData;

  const getPortsForSlot = (slot: GPONSlot): GponPortEntity[] => slot.ports;

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      {/* Clean 3-dropdown hierarchy in one row */}
      <Space wrap>
        <Select
          placeholder="Select OLT"
          style={{ width: 240 }}
          value={selectedOltId}
          onChange={(value) => {
            setSelectedOltId(value);
            setSelectedSlotId(undefined);
            setSelectedPortId(undefined);
          }}
          allowClear
          showSearch
          optionFilterProp="label"
        >
          {oltsData?.map((olt) => (
            <Option key={olt.id} value={olt.id} label={`${olt.name}`}>
              {olt.name} ({olt.ipAddress})
            </Option>
          ))}
        </Select>

        <Select
          placeholder="Select Card/Slot"
          style={{ width: 200 }}
          value={selectedSlotId}
          onChange={(value) => {
            setSelectedSlotId(value);
            setSelectedPortId(undefined);
          }}
          allowClear
          disabled={!selectedOltId || cards.length === 0}
          loading={isLoadingTopology}
          notFoundContent={
            isLoadingTopology ? "Loading topology..." : "No slots found"
          }
        >
          {cards.map((slot: GPONSlot) => {
            const totalOnus = slot.ports.reduce(
              (acc: number, p: GponPortEntity) => acc + p.onts.length,
              0,
            );
            return (
              <Option key={slot.slot} value={slot.slot}>
                Card {slot.slot} · {ontPositionLabel(slot.slot)} ({totalOnus}{" "}
                {totalOnus === 1 ? "ONT" : "ONTs"})
              </Option>
            );
          })}
        </Select>

        <Select
          placeholder="Select PON Port"
          style={{ width: 200 }}
          value={selectedPortId}
          onChange={(value) => {
            setSelectedPortId(value);
          }}
          allowClear
          disabled={!selectedSlotId}
        >
          {(() => {
            if (!selectedSlotId) return [];
            const currentSlot = topologyData.find(
              (s) => s.slot === selectedSlotId,
            );
            if (!currentSlot) return [];
            return getPortsForSlot(currentSlot).map((port: GponPortEntity) => {
              const onlineCount = port.onts.filter(
                (ont) => ont.runState === 3,
              ).length;
              return (
                <Option key={port.portId} value={port.portId}>
                  PON {port.portId} ·{" "}
                  {ontPositionLabel(selectedSlotId, port.portId)} (
                  {port.onts.length} ONTs, {onlineCount} online)
                </Option>
              );
            });
          })()}
        </Select>

        {(selectedOltId || selectedSlotId || selectedPortId) && (
          <Button icon={<ReloadOutlined />} onClick={onReset}>
            Reset
          </Button>
        )}
      </Space>

      {/* Search & Status filters */}
      <Space wrap>
        <Input
          placeholder="Search serial number"
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          style={{ width: 200 }}
          allowClear
        />
        <Select
          placeholder="Filter status"
          style={{ width: 150 }}
          value={statusFilter}
          onChange={setStatusFilter}
          allowClear
        >
          {ONT_STATUSES.map((status) => (
            <Option key={status.value} value={status.value}>
              {status.label}
            </Option>
          ))}
        </Select>
      </Space>
    </Space>
  );
}
