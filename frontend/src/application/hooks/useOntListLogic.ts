import { useState, useEffect } from "react";
import { App } from "antd";
import {
  useOnts,
  useCreateOnt,
  useDeleteOnt,
} from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { OntRepository } from "@/infrastructure/repositories/OntRepository";
import { SEARCH_DEBOUNCE_MS } from "@/shared/config/limits";
import { DEFAULT_ONT_PAGE_SIZE } from "@/presentation/components/ontPageSize";
import { useDebouncedValue } from "@/application/hooks/useDebouncedValue";
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

  // The table's page, held here because the server returns one page at a time
  // now. It resets whenever a filter changes: staying on page 9 of a result
  // that just became one page long shows an empty table over a full network.
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(DEFAULT_ONT_PAGE_SIZE);

  const search = useDebouncedValue(searchText.trim(), SEARCH_DEBOUNCE_MS);

  useEffect(() => {
    setPage(1);
  }, [
    selectedOltId,
    selectedSlotId,
    selectedPortId,
    statusFilter,
    search,
    pageSize,
  ]);

  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isDetailModalOpen, setIsDetailModalOpen] = useState(false);
  const [selectedOnt, setSelectedOnt] = useState<Ont | null>(null);

  // Fetch database ONTs
  const {
    data: ontsData,
    isLoading,
    refetch,
  } = useOnts({
    // Every filter is applied by the database. Filtering in the browser could
    // only ever narrow the rows one request had returned, so on a network larger
    // than a page it answered from a slice of itself: selecting card 9 searched
    // the first page for card 9, not the chassis.
    oltId: selectedOltId,
    status: statusFilter,
    slot: selectedSlotId,
    portId: selectedPortId,
    search: search || undefined,
    limit: pageSize,
    offset: (page - 1) * pageSize,
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

  const onts = ontsData?.data || [];
  // What the server says matches, not what arrived. This number drives the
  // pager, and taking it from the page's own length would tell an operator the
  // network ends where their screen does.
  const total = ontsData?.total ?? 0;

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

  const handleDelete = async (id: string, removeFromOlt = false) => {
    try {
      await deleteMutation.mutateAsync({ id, removeFromOlt });
      if (selectedOnt?.id === id) {
        setIsDetailModalOpen(false);
        setSelectedOnt(null);
      }
      message.success(
        removeFromOlt
          ? "Removed from the OLT and from TikMan"
          : "Removed from TikMan",
      );
    } catch (error) {
      // The OLT's own refusal is the useful part: it names the command it
      // rejected, which a generic failure message would hide.
      message.error(
        error instanceof Error ? error.message : "Failed to remove the ONT",
      );
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
    filteredOnts: onts,
    total,
    page,
    setPage,
    pageSize,
    setPageSize,
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
