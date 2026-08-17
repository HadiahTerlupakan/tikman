import { useState, useEffect } from "react";
import {
  Card,
  Table,
  Tag,
  Space,
  Button,
  Input,
  Select,
  Modal,
  Form,
  InputNumber,
  App,
} from "antd";
import {
  PlusOutlined,
  ReloadOutlined,
  SearchOutlined,
  EyeOutlined,
  EditOutlined,
  DeleteOutlined,
} from "@ant-design/icons";
import { useOnts, useCreateOnt, useUpdateOnt, useDeleteOnt } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { OntDetailModal } from "@/presentation/components/OntDetailModal";
import axios from "axios";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import type { Ont, CreateOntDto, UpdateOntDto, OntStatus } from "@/domain/entities";

const { Option } = Select;

// GPONSlot interface matches backend topology response (after camelizeKeys)
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

export default function OntListPage() {
  const { message } = App.useApp();
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState<OntStatus | undefined>();

  // Hierarchy state for OLT topology discovery
  const [selectedOltId, setSelectedOltId] = useState<string | undefined>();
  const [selectedSlotId, setSelectedSlotId] = useState<number | undefined>();
  const [selectedPortId, setSelectedPortId] = useState<number | undefined>();
  const [topologyData, setTopologyData] = useState<GPONSlot[]>([]);

  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [selectedOnt, setSelectedOnt] = useState<Ont | null>(null);
  const [createForm] = Form.useForm();
  const [editForm] = Form.useForm();

  // Fetch database ONTs - only when NOT in topology discovery mode
  const { data: ontsData, isLoading, refetch } = useOnts({
    oltId: undefined,
    status: statusFilter,
  });
  const { data: oltsData } = useOlts();
  const createMutation = useCreateOnt();
  const updateMutation = useUpdateOnt();
  const deleteMutation = useDeleteOnt();

  // Get ONTs from discovered topology based on slot/port selection
  const getDiscoveredOnTsForPort = (): GponPortEntity | undefined => {
    if (!selectedSlotId || !selectedPortId) return undefined;

    const currentSlot = topologyData.find(s => s.slot === selectedSlotId);
    if (!currentSlot) return undefined;

    return currentSlot.ports.find(p => p.portId === selectedPortId);
  };

  // Get currently filtered ONTs (either from discovery or database)
  const currentViewOntData: (any[] | undefined) = (() => {
    console.log('[currentViewOntData] State:', {
      selectedPortId,
      selectedSlotId,
      topologyDataLength: topologyData.length
    });

    // If port is selected, show discovered ONTs only
    if (selectedPortId !== undefined && selectedPortId !== null) {
      const foundPort = getDiscoveredOnTsForPort();

      console.log('[ONT Debug]', {
        selectedSlotId,
        selectedPortId,
        foundPort: foundPort ? {
          portId: foundPort.portId,
          ontsCount: foundPort.onts.length,
          firstOnt: foundPort.onts[0]
        } : null
      });

      if (foundPort) {
        return foundPort.onts.map((ont) => {
          const mappedOnt: Ont & { name?: string; description?: string; distance?: number; rxPower?: number; txPower?: number } = {
            id: `discovered-${ont.portId}-${ont.ontId}`,
            serialNumber: ont.serialNumber || `Unknown`,
            oltName: String(oltsData?.find((o: any) => o.id === selectedOltId)?.name || ''),
            oltId: selectedOltId!,
            portId: ont.portId,
            ontId: ont.ontId,
            status: 'online' as any, // Will be set below
            description: '',
            lastSeenAt: new Date().toISOString(),
            createdAt: new Date().toISOString(),
            updatedAt: new Date().toISOString(),
          };

          // Set status correctly
          if (ont.runState === 3) {
            mappedOnt.status = 'online' as any;
          } else if (ont.runState === 4) {
            mappedOnt.status = 'dying_gasp' as any;
          } else if (ont.runState === 6) {
            mappedOnt.status = 'offline' as any;
          } else if (ont.runState === 1) {
            mappedOnt.status = 'los' as any;
          } else {
            mappedOnt.status = 'unknown' as any;
          }

          // Add optional fields from discovered ONT (after camelizeKeys conversion)
          if (ont.name) {
            mappedOnt.name = ont.name;
          }
          if (ont.description) {
            mappedOnt.description = ont.description;
          }
          if (ont.distance !== undefined && ont.distance > 0) {
            (mappedOnt as any).distance = ont.distance;
          }
          if (ont.rxPower !== undefined && ont.rxPower !== null) {
            (mappedOnt as any).rxPower = ont.rxPower;
          }
          if (ont.txPower !== undefined && ont.txPower !== null) {
            (mappedOnt as any).txPower = ont.txPower;
          }

          return mappedOnt;
        });
      }
      return [];
    }

    // If OLT is selected but no port selected yet, show empty (waiting for port selection)
    if (selectedOltId) {
      return [];
    }

    // Default view - show database ONTs with filters
    if (!ontsData?.data) return [];
    let filtered = [...ontsData.data];

    // Search filter
    if (searchText) {
      filtered = filtered.filter((ont: Ont) =>
        ont.serialNumber.toLowerCase().includes(searchText.toLowerCase())
      );
    }

    // Status filter
    if (statusFilter) {
      filtered = filtered.filter((ont: Ont) => ont.status === statusFilter);
    }

    return filtered;
  })();

  // Fetch topology when OLT is selected for hierarchy view
  useEffect(() => {
    if (!selectedOltId) {
      setTopologyData([]);
      setSelectedSlotId(undefined);
      setSelectedPortId(undefined);
      return;
    }

    const fetchTopology = async () => {
      try {
        const response = await axios.post(`${API_ENDPOINTS.OLTS}/${selectedOltId}/topology`);
        console.log('[Topology Response]', response.data);

        // Manual deep conversion for nested arrays
        const topology = response.data.topology?.map((slot: any) => ({
          slot: slot.slot,
          ports: slot.ports?.map((port: any) => ({
            portId: port.port_id || port.portId,
            onts: port.onts?.map((ont: any) => ({
              portId: ont.port_id ?? ont.portId,
              ontId: ont.ont_id ?? ont.ontId,
              serialNumber: ont.serial_number ?? ont.serialNumber,
              runState: ont.run_state ?? ont.runState,
              name: ont.name,
              description: ont.description,
              rxPower: ont.rx_power !== undefined ? ont.rx_power : ont.rxPower,
              txPower: ont.tx_power !== undefined ? ont.tx_power : ont.txPower,
              distance: ont.distance,
              status: ont.status,
              lastSeenAt: ont.last_seen_at ?? ont.lastSeenAt,
            })) || []
          })) || []
        })) || [];

        setTopologyData(topology);
        message.success(`Discovered ${topology.length} slot(s)`);
      } catch (error) {
        console.error("Failed to fetch topology:", error);
        message.error("Failed to discover ONT topology");
      }
    };

    fetchTopology();
  }, [selectedOltId]);

  // Get active slots and ports from topology
  const activeSlots = topologyData.filter(slot =>
    slot.ports.some(port => port.onts.length > 0)
  );

  // Get all ports from selected slot
  const getPortsForSlot = (slot: GPONSlot): GponPortEntity[] => {
    return slot.ports.filter(port => port.onts.length > 0);
  };

  const getStatusColor = (status: OntStatus) => {
    switch (status) {
      case "online":
        return "success";
      case "offline":
        return "default";
      case "los":
        return "warning";
      case "dying_gasp":
        return "error";
      case "unknown":
        return "default";
      default:
        return "default";
    }
  };

  const columns = [
    {
      title: "Serial Number",
      dataIndex: "serialNumber",
      key: "serialNumber",
      filteredValue: searchText ? [searchText] : null,
      onFilter: (value: unknown, record: Ont) =>
        record.serialNumber.toLowerCase().includes(String(value).toLowerCase()),
    },
    {
      title: "OLT",
      dataIndex: "oltName",
      key: "oltName",
    },
    {
      title: "Port/ONT ID",
      key: "position",
      render: (_: unknown, record: Ont) => `${record.portId}/${record.ontId}`,
    },
    {
      title: "Status",
      dataIndex: "status",
      key: "status",
      render: (status: OntStatus) => (
        <Tag color={getStatusColor(status)}>{status.toUpperCase()}</Tag>
      ),
    },
    {
      title: "Name",
      dataIndex: "name",
      key: "name",
      ellipsis: true,
    },
    {
      title: "Description",
      dataIndex: "description",
      key: "description",
      ellipsis: true,
    },
    {
      title: "Distance (m)",
      key: "distance",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const distance = recordAny.distance;
        return distance > 0 ? distance.toLocaleString() : "-";
      },
    },
    {
      title: "RX Power (dBm)",
      key: "rxPower",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const rxPower = recordAny.rxPower ?? recordAny.metrics?.rxPower;
        return rxPower !== null && rxPower !== undefined ? parseFloat(rxPower).toFixed(2) : "-";
      },
    },
    {
      title: "TX Power (dBm)",
      key: "txPower",
      render: (_: unknown, record: Ont) => {
        const recordAny = record as any;
        const txPower = recordAny.txPower ?? recordAny.metrics?.txPower;
        return txPower !== null && txPower !== undefined ? parseFloat(txPower).toFixed(2) : "-";
      },
    },
    {
      title: "Actions",
      key: "actions",
      render: (_: unknown, record: Ont) => {
        // Check if this is a discovered ONT (has fake ID format) - only allow View
        if (record.id && record.id.startsWith('discovered-')) {
          return (
            <Button
              type="link"
              icon={<EyeOutlined />}
              disabled
              title="Real-time view only"
            >
              View
            </Button>
          );
        }
        // Regular database ONT - allow all actions
        return (
          <Space>
            <Button
              type="link"
              icon={<EyeOutlined />}
              onClick={() => handleViewDetail(record)}
            >
              View
            </Button>
            <Button
              type="link"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record)}
            >
              Edit
            </Button>
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
              onClick={() => handleDelete(record)}
            >
              Delete
            </Button>
          </Space>
        );
      },
    },
  ];

  const handleViewDetail = (ont: Ont) => {
    setSelectedOnt(ont);
    setIsDetailModalOpen(true);
  };

  const handleEdit = (ont: Ont) => {
    setSelectedOnt(ont);
    editForm.setFieldsValue({
      description: ont.description,
      status: ont.status,
    });
    setIsEditModalOpen(true);
  };

  const handleDelete = (ont: Ont) => {
    Modal.confirm({
      title: "Delete ONT",
      content: `Are you sure you want to delete ONT ${ont.serialNumber}?`,
      okText: "Delete",
      okType: "danger",
      onOk: async () => {
        try {
          await deleteMutation.mutateAsync(ont.id);
          message.success("ONT deleted successfully");
        } catch (error) {
          console.error("Delete failed:", error);
        }
      },
    });
  };

  const handleCreate = async (values: CreateOntDto) => {
    try {
      await createMutation.mutateAsync(values);
      setIsCreateModalOpen(false);
      createForm.resetFields();
      message.success("ONT created successfully");
    } catch (error) {
      console.error("Create failed:", error);
    }
  };

  const handleUpdate = async (values: UpdateOntDto) => {
    if (!selectedOnt) return;
    try {
      await updateMutation.mutateAsync({ id: selectedOnt.id, data: values });
      setIsEditModalOpen(false);
      editForm.resetFields();
      setSelectedOnt(null);
      message.success("ONT updated successfully");
    } catch (error) {
      console.error("Update failed:", error);
    }
  };

  return (
    <div style={{ padding: "24px" }}>
      <Card style={{ marginBottom: 16 }}>
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
              disabled={!selectedOltId || activeSlots.length === 0}
            >
              {activeSlots.map((slot: GPONSlot) => {
                const totalOnus = slot.ports.reduce((acc: number, p: GponPortEntity) => acc + p.onts.length, 0);
                return (
                  <Option key={slot.slot} value={slot.slot}>
                    Card {slot.slot} ({totalOnus} ONTs)
                  </Option>
                );
              })}
            </Select>

            <Select
              placeholder="Select PON Port"
              style={{ width: 200 }}
              value={selectedPortId}
              onChange={(value) => {
                console.log('[Port Selected]', value);
                setSelectedPortId(value);
              }}
              allowClear
              disabled={!selectedSlotId}
            >
              {(() => {
                if (!selectedSlotId) return [];
                const currentSlot = topologyData.find(s => s.slot === selectedSlotId);
                if (!currentSlot) return [];
                return getPortsForSlot(currentSlot).map((port: GponPortEntity) => {
                  const onlineCount = port.onts.filter((ont: any) => ont.runState === 3).length;
                  return (
                    <Option key={port.portId} value={port.portId}>
                      Port {port.portId} ({port.onts.length} ONTs, {onlineCount} online)
                    </Option>
                  );
                });
              })()}
            </Select>

            {(selectedOltId || selectedSlotId || selectedPortId) && (
              <Button
                icon={<ReloadOutlined />}
                onClick={() => {
                  setSelectedOltId(undefined);
                  setSelectedSlotId(undefined);
                  setSelectedPortId(undefined);
                  setSearchText('');
                  setStatusFilter(undefined);
                }}
              >
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
              <Option value="online">Online</Option>
              <Option value="offline">Offline</Option>
              <Option value="los">LOS</Option>
              <Option value="dying_gasp">Dying Gasp</Option>
              <Option value="unknown">Unknown</Option>
            </Select>
          </Space>
        </Space>
      </Card>

      <Card
        title="ONT Monitoring"
        extra={
          <Space>
            <Button icon={<ReloadOutlined />} onClick={() => refetch()}>
              Refresh
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => setIsCreateModalOpen(true)}
            >
              Add ONT
            </Button>
          </Space>
        }
      >
        <Table
          columns={columns}
          dataSource={currentViewOntData}
          rowKey="id"
          loading={isLoading || !ontsData && currentViewOntData.length === 0}
          pagination={{
            total: currentViewOntData.length,
            pageSize: 20,
            showTotal: (total) => `Total ${total} ONTs`,
          }}
        />
      </Card>

      {/* Create Modal */}
      <Modal
        title="Add New ONT"
        open={isCreateModalOpen}
        onCancel={() => {
          setIsCreateModalOpen(false);
          createForm.resetFields();
        }}
        onOk={() => createForm.submit()}
        confirmLoading={createMutation.isPending}
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreate}>
          <Form.Item
            name="oltId"
            label="OLT"
            rules={[{ required: true, message: "Please select an OLT" }]}
          >
            <Select placeholder="Select OLT">
              {oltsData?.map((olt) => (
                <Option key={olt.id} value={olt.id} label={olt.name}>
                  {olt.name}
                </Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item
            name="portId"
            label="Port ID"
            rules={[
              { required: true, message: "Please enter port ID" },
              { type: "number", min: 0, max: 15, message: "Port ID must be 0-15" },
            ]}
          >
            <InputNumber style={{ width: "100%" }} min={0} max={15} />
          </Form.Item>
          <Form.Item
            name="ontId"
            label="ONT ID"
            rules={[
              { required: true, message: "Please enter ONT ID" },
              { type: "number", min: 0, max: 127, message: "ONT ID must be 0-127" },
            ]}
          >
            <InputNumber style={{ width: "100%" }} min={0} max={127} />
          </Form.Item>
          <Form.Item
            name="serialNumber"
            label="Serial Number"
            rules={[
              { required: true, message: "Please enter serial number" },
              { max: 20, message: "Serial number max 20 characters" },
            ]}
          >
            <Input placeholder="e.g., ZTEG12345678" />
          </Form.Item>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} placeholder="Optional description" />
          </Form.Item>
        </Form>
      </Modal>

      {/* Edit Modal */}
      <Modal
        title="Edit ONT"
        open={isEditModalOpen}
        onCancel={() => {
          setIsEditModalOpen(false);
          editForm.resetFields();
          setSelectedOnt(null);
        }}
        onOk={() => editForm.submit()}
        confirmLoading={updateMutation.isPending}
      >
        <Form form={editForm} layout="vertical" onFinish={handleUpdate}>
          <Form.Item name="description" label="Description">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="status" label="Status">
            <Select>
              <Option value="online">Online</Option>
              <Option value="offline">Offline</Option>
              <Option value="los">LOS</Option>
              <Option value="dying_gasp">Dying Gasp</Option>
              <Option value="unknown">Unknown</Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>

      {/* Detail Modal with Metrics */}
      {selectedOnt && (
        <OntDetailModal
          ont={selectedOnt}
          visible={isDetailModalOpen}
          onClose={() => {
            setIsDetailModalOpen(false);
            setSelectedOnt(null);
          }}
        />
      )}
    </div>
  );
}
