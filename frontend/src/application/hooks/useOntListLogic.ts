import { useState, useEffect } from "react";
import { App } from "antd";
import { useOnts, useCreateOnt } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import axios from "axios";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import type { Ont, CreateOntDto, OntStatus } from "@/domain/entities";

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

export function useOntListLogic() {
  const { message } = App.useApp();
  const [searchText, setSearchText] = useState("");
  const [statusFilter, setStatusFilter] = useState<OntStatus | undefined>();

  // Hierarchy state for OLT topology discovery
  const [selectedOltId, setSelectedOltId] = useState<string | undefined>();
  const [selectedSlotId, setSelectedSlotId] = useState<number | undefined>();
  const [selectedPortId, setSelectedPortId] = useState<number | undefined>();
  const [topologyData, setTopologyData] = useState<GPONSlot[]>([]);
  const [isLoadingTopology, setIsLoadingTopology] = useState(false);

  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [selectedOnt, setSelectedOnt] = useState<Ont | null>(null);

  // Fetch database ONTs
  const { data: ontsData, isLoading, refetch } = useOnts({
    oltId: undefined,
    status: statusFilter,
    limit: 200,
    offset: 0,
  });

  // Update total after data loads
  useEffect(() => {
    if (ontsData && ontsData.total) {
      console.log(`[Data Loaded] Total ONTs: ${ontsData.total}`);
    }
  }, [ontsData?.total]);

  // Only refetch when filter changes
  useEffect(() => {
    if (statusFilter) {
      console.log('[Status Filter Changed] Refetching with status:', statusFilter);
      refetch();
    }
  }, [statusFilter]);

  const { data: oltsData } = useOlts();
  const createMutation = useCreateOnt();

  const filteredOnts = (ontsData?.data || []).filter((ont: Ont) => {
    if (selectedOltId && ont.oltId !== selectedOltId) {
      return false;
    }
    
    if (selectedPortId !== undefined && ont.portId !== selectedPortId) {
      return false;
    }
    
    if (searchText && !ont.serialNumber.toLowerCase().includes(searchText.toLowerCase())) {
      return false;
    }
    
    if (statusFilter && ont.status !== statusFilter) {
      return false;
    }
    
    return true;
  });

  // Debug log
  useEffect(() => {
    console.log('[ONT DEBUG] ontsData:', {
      total_from_api: ontsData?.total,
      data_array_length: ontsData?.data?.length,
      filtered_count: filteredOnts.length,
      searchText: searchText,
      statusFilter: statusFilter,
      selectedOltId: selectedOltId,
      selectedPortId: selectedPortId,
      limit: 200,
      offset: 0,
      sample_ont: ontsData?.data?.[0]
    });
  }, [ontsData, filteredOnts, selectedOltId, selectedPortId]);

  // Fetch topology when OLT is selected
  useEffect(() => {
    if (!selectedOltId) {
      setTopologyData([]);
      setSelectedSlotId(undefined);
      setSelectedPortId(undefined);
      setIsLoadingTopology(false);
      return;
    }

    const fetchTopology = async () => {
      setIsLoadingTopology(true);
      try {
        const response = await axios.get(`${API_ENDPOINTS.OLTS}/${selectedOltId}/topology/cached`);
        console.log('[Topology Response]', response.data);

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
      } finally {
        setIsLoadingTopology(false);
      }
    };

    fetchTopology();
  }, [selectedOltId]);

  const handleViewDetail = (ont: Ont) => {
    setSelectedOnt(ont);
    setIsDetailModalOpen(true);
  };

  const handleCreate = async (values: CreateOntDto) => {
    try {
      await createMutation.mutateAsync(values);
      setIsCreateModalOpen(false);
      message.success("ONT created successfully");
    } catch (error) {
      console.error("Create failed:", error);
    }
  };

  const handleReset = () => {
    setSelectedOltId(undefined);
    setSelectedSlotId(undefined);
    setSelectedPortId(undefined);
    setSearchText('');
    setStatusFilter(undefined);
  };

  return {
    // State
    searchText,
    setSearchText,
    statusFilter,
    setStatusFilter,
    selectedOltId,
    setSelectedOltId,
    selectedSlotId,
    setSelectedSlotId,
    selectedPortId,
    setSelectedPortId,
    topologyData,
    isLoadingTopology,
    isCreateModalOpen,
    setIsCreateModalOpen,
    isDetailModalOpen,
    setIsDetailModalOpen,
    selectedOnt,
    setSelectedOnt,
    
    // Data
    oltsData: oltsData || [],
    filteredOnts,
    isLoading,
    createMutation,
    
    // Actions
    handleViewDetail,
    handleCreate,
    handleReset,
    refetch,
  };
}
