import { useState } from "react";
import { App, Button, Space, Typography } from "antd";
import { BellOutlined, PlusOutlined } from "@ant-design/icons";
import {
  useDeleteGraphReport,
  useGraphReports,
  useSaveGraphReport,
  useSendGraphReport,
} from "@/application/hooks";
import type { GraphReport, GraphReportInput } from "@/domain/entities";
import { PageHeader, DarkCard } from "../components/common";
import {
  GraphReportModal,
  GraphReportTable,
  PortAlertDestinationModal,
} from "../components/graph-reports";
import { reportWithEnabled } from "../components/graph-reports/graphReportForm";

export default function GraphReportsPage() {
  const { message } = App.useApp();
  const { data: reports = [], isLoading } = useGraphReports();
  const saveMutation = useSaveGraphReport();
  const toggleMutation = useSaveGraphReport();
  const deleteMutation = useDeleteGraphReport();
  const sendMutation = useSendGraphReport();
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<GraphReport | undefined>();
  const [nocOpen, setNocOpen] = useState(false);

  const openModal = (report?: GraphReport) => {
    setEditing(report);
    setModalOpen(true);
  };

  const handleSubmit = (input: GraphReportInput) =>
    saveMutation.mutate(
      { id: editing?.id, input },
      {
        onSuccess: () => {
          message.success("Laporan disimpan");
          setModalOpen(false);
        },
        onError: (error) => message.error(error.message),
      },
    );

  const handleToggle = (report: GraphReport, enabled: boolean) =>
    toggleMutation.mutate(
      { id: report.id, input: reportWithEnabled(report, enabled) },
      { onError: (error) => message.error(error.message) },
    );

  const handleSend = (report: GraphReport) =>
    sendMutation.mutate(report.id, {
      onSuccess: (text) => message.success(text || "Laporan dikirim"),
      onError: (error) => message.error(error.message),
    });

  const handleDelete = (report: GraphReport) =>
    deleteMutation.mutate(report.id, {
      onSuccess: () => message.success("Laporan dihapus"),
      onError: (error) => message.error(error.message),
    });

  return (
    <div>
      <PageHeader
        title="Laporan Grafik Harian"
        description="Grafik trafik port LibreNMS yang dikirim ke grup WhatsApp dan Telegram"
        extra={
          <Space>
            <Button icon={<BellOutlined />} onClick={() => setNocOpen(true)}>
              Notifikasi NOC
            </Button>
            <Button
              type="primary"
              icon={<PlusOutlined />}
              onClick={() => openModal()}
            >
              Tambah laporan
            </Button>
          </Space>
        }
      />
      <DarkCard>
        <Typography.Paragraph type="secondary">
          Setiap laporan dikirim otomatis tiap hari pada jam kirimnya sendiri
          (WIB), berisi grafik 24 jam terakhir. URL dan token LibreNMS, serta
          token bot Telegram, diatur di Pengaturan.
        </Typography.Paragraph>
        <GraphReportTable
          reports={reports}
          loading={isLoading}
          sendingId={
            sendMutation.isPending ? sendMutation.variables : undefined
          }
          togglingId={
            toggleMutation.isPending ? toggleMutation.variables?.id : undefined
          }
          onToggle={handleToggle}
          onSend={handleSend}
          onEdit={openModal}
          onDelete={handleDelete}
        />
      </DarkCard>
      <GraphReportModal
        open={modalOpen}
        report={editing}
        reports={reports}
        loading={saveMutation.isPending}
        onClose={() => setModalOpen(false)}
        onSubmit={handleSubmit}
      />
      <PortAlertDestinationModal
        open={nocOpen}
        onClose={() => setNocOpen(false)}
      />
    </div>
  );
}
