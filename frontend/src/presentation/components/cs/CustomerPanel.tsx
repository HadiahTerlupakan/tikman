import { useState } from "react";
import { App, Button, Select, Space, Tag, Typography } from "antd";
import { LinkOutlined } from "@ant-design/icons";
import { Link } from "react-router-dom";
import type { CsConversation } from "@/domain/entities";
import { useOnt, useOnts } from "@/application/hooks/useOnts";
import {
  useLinkConversationOnt,
  useSetConversationStatus,
} from "@/application/hooks/useCsInbox";
import {
  ontStatusColor,
  ontStatusLabel,
} from "@/presentation/components/ontStatus";

const { Text, Title } = Typography;

// Below this many characters an ONT search would just return "everything",
// which is neither what a CS wants nor cheap to fetch on every keystroke.
const MIN_SEARCH_LENGTH = 3;

interface CustomerPanelProps {
  conversation: CsConversation;
}

/** The subscriber side of a thread: who they are, what ONT they are on (once
 * linked), and the search that makes that link in the first place. */
export function CustomerPanel({ conversation }: CustomerPanelProps) {
  const { message } = App.useApp();
  const [search, setSearch] = useState("");

  const ontQuery = useOnt(conversation.ontId ?? "");
  const searchQuery = useOnts({
    search,
    limit: 10,
    enabled: search.trim().length >= MIN_SEARCH_LENGTH,
  });
  const linkOnt = useLinkConversationOnt();
  const setStatus = useSetConversationStatus();

  const handleLink = (ontId: string) => {
    linkOnt.mutate(
      { conversationId: conversation.id, ontId },
      {
        onSuccess: (result) => {
          if (!result.phoneRecorded) {
            message.warning(
              "Nomor ini sudah tercatat di ONT lain — tautan dibuat, nomornya tidak dipindah.",
            );
          }
        },
      },
    );
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <div>
        <Title level={5} style={{ marginBottom: 4 }}>
          {conversation.customerName || "Tanpa nama"}
        </Title>
        <Text type="secondary">{conversation.customerPhone}</Text>
      </div>

      {conversation.ontId ? (
        <OntLinkSummary ont={ontQuery.data} />
      ) : (
        <Space direction="vertical" style={{ width: "100%" }}>
          <Text type="secondary">Belum tertaut ke ONT</Text>
          <Select
            showSearch
            allowClear
            placeholder="Cari ONT (nama, serial, atau nomor)"
            style={{ width: "100%" }}
            filterOption={false}
            searchValue={search}
            onSearch={setSearch}
            loading={searchQuery.isFetching}
            notFoundContent={
              search.trim().length >= MIN_SEARCH_LENGTH
                ? "Tidak ditemukan"
                : null
            }
            options={(searchQuery.data?.data ?? []).map((ont) => ({
              value: ont.id,
              label: `${ont.name || ont.serialNumber} — ${ont.oltName}`,
            }))}
            onSelect={handleLink}
          />
        </Space>
      )}

      {conversation.status === "open" && (
        <Button
          danger
          onClick={() =>
            setStatus.mutate({
              conversationId: conversation.id,
              status: "closed",
            })
          }
          loading={setStatus.isPending}
        >
          Tutup Percakapan
        </Button>
      )}
    </Space>
  );
}

function OntLinkSummary({ ont }: { ont: ReturnType<typeof useOnt>["data"] }) {
  if (!ont) {
    return <Text type="secondary">Memuat ONT...</Text>;
  }
  return (
    <Space direction="vertical">
      <Tag color={ontStatusColor(ont.status)}>{ontStatusLabel(ont.status)}</Tag>
      <Text>{ont.name || ont.serialNumber}</Text>
      <Link to="/onts">
        <LinkOutlined /> Lihat di daftar ONT
      </Link>
    </Space>
  );
}
