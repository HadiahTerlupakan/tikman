import { useState, useEffect } from "react";
import { App } from "antd";
import {
  useOnts,
  useCreateOnt,
  useDeleteOnt,
} from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { OntRepository } from "@/infrastructure/repositories/OntRepository";
import type { Ont, CreateOntDto, OntStatus } from "@/domain/entities";

const ontRepository = new OntRepository();

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
  const {
    data: ontsData,
    isLoading,
    refetch,
  } = useOnts({
    // Filter by OLT on the server. This was hardcoded to undefined while the
    // OLT filter was applied client-side, so every OLT competed for one window
    // of rows: with 444 ONTs across two OLTs, whichever sorted later was absent
    // from the page entirely no matter which OLT you selected.
    oltId: selectedOltId,
    status: statusFilter,
    // ponytail: fixed ceiling of 1000 rows per fetch, with pagination and search
    // still done client-side. Enough for one OLT (largest here is 246) but an OLT
    // with more than 1000 ONTs would silently lose the remainder. Upgrade path is
    // server-side pagination: pass offset from the table's page and drive the
    // total from the response instead of from the fetched array.
    limit: 1000,
    offset: 0,
  });

  // Only refetch when filter changes
  useEffect(() => {
    if (statusFilter) {
      refetch();
    }
  }, [statusFilter, refetch]);

  const { data: oltsData } = useOlts();
  const createMutation = useCreateOnt();
  const deleteMutation = useDeleteOnt();

  const filteredOnts = (ontsData?.data || []).filter((ont: Ont) => {
    if (selectedOltId && ont.oltId !== selectedOltId) {
      return false;
    }

    if (selectedPortId !== undefined && ont.portId !== selectedPortId) {
      return false;
    }

    if (
      searchText &&
      !ont.serialNumber.toLowerCase().includes(searchText.toLowerCase())
    ) {
      return false;
    }

    if (statusFilter && ont.status !== statusFilter) {
      return false;
    }

    return true;
  });

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
        const topology = await ontRepository.getTopology(selectedOltId);

        const mappedTopology = (topology || []).map((slot) => ({
          slot: slot.slot,
          ports:
            slot.ports?.map((port) => ({
              portId: port.portId || 0,
              onts:
                port.onts?.map((ont) => ({
                  portId: ont.portId || 0,
                  ontId: ont.ontId || 0,
                  serialNumber: ont.serialNumber || "",
                  runState: ont.runState || 0,
                  name: ont.name,
                  description: ont.description,
                  rxPower: ont.rxPower,
                  txPower: ont.txPower,
                  distance: ont.distance,
                  status: ont.status,
                  lastSeenAt: ont.lastSeenAt,
                })) || [],
            })) || [],
        }));

        setTopologyData(mappedTopology);
        message.success(`Discovered ${mappedTopology.length} slot(s)`);
      } catch {
        message.error("Failed to discover ONT topology");
      } finally {
        setIsLoadingTopology(false);
      }
    };

    fetchTopology();
  }, [selectedOltId, message]);

  const handleViewDetail = (ont: Ont) => {
    setSelectedOnt(ont);
    setIsDetailModalOpen(true);
  };

  const handleCreate = async (values: CreateOntDto) => {
    try {
      await createMutation.mutateAsync(values);
      setIsCreateModalOpen(false);
      message.success("ONT created successfully");
    } catch {
      message.error("Failed to create ONT");
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await deleteMutation.mutateAsync(id);
      if (selectedOnt?.id === id) {
        setIsDetailModalOpen(false);
        setSelectedOnt(null);
      }
      message.success("ONT deleted successfully");
    } catch {
      message.error("Failed to delete ONT");
    }
  };

  const handleReset = () => {
    setSelectedOltId(undefined);
    setSelectedSlotId(undefined);
    setSelectedPortId(undefined);
    setSearchText("");
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
    deleteMutation,

    // Actions
    handleViewDetail,
    handleCreate,
    handleDelete,
    handleReset,
    refetch,
  };
}
