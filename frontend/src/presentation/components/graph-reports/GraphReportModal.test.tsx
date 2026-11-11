import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { GraphReport } from "@/domain/entities";
import { GraphReportModal } from "./GraphReportModal";

const idle = { data: [], isLoading: false, error: null };

vi.mock("@/application/hooks", () => ({
  useGraphReportPreview: () => ({ mutate: vi.fn(), isPending: false }),
  useLibrenmsDevices: () => idle,
  useLibrenmsPorts: () => idle,
  useLibrenmsGraph: () => ({ data: undefined, isLoading: false, error: null }),
  useTelegramChats: () => idle,
  useWaAccounts: () => idle,
  useWaGroups: () => idle,
}));

const whatsappReport: GraphReport = {
  id: "r1",
  waGroupId: "g1",
  groupName: "TEST MRTG",
  waGroupJid: "g1@g.us",
  waAccountId: "a1",
  waAccountLabel: "Kantor",
  telegramChatId: null,
  telegramChatTitle: "",
  telegramThreadId: null,
  telegramTopicName: "",
  customerName: "PT WhatsApp",
  message: "",
  enabled: true,
  mentionAll: true,
  notifyPortStatus: false,
  sendTime: "07:00",
  lastSentOn: null,
  lastAttemptAt: null,
  lastError: "",
  ports: [],
};

const telegramReport: GraphReport = {
  ...whatsappReport,
  id: "r2",
  waGroupId: null,
  waAccountId: null,
  telegramChatId: "t1",
  telegramChatTitle: "NOC",
  customerName: "PT Telegram",
};

function modal(open: boolean, report?: GraphReport) {
  return (
    <GraphReportModal
      open={open}
      report={report}
      reports={[]}
      loading={false}
      onClose={vi.fn()}
      onSubmit={vi.fn()}
    />
  );
}

describe("GraphReportModal", () => {
  // The page keeps the modal mounted between opens, so its form store
  // outlives each open; without clearing it, Simpan on the second report
  // wrote the first report's customer and groups over it.
  it("opens each report on its own values, not the previously opened one", () => {
    const { rerender } = render(modal(true, whatsappReport));
    expect(screen.getByLabelText("Nama pelanggan")).toHaveValue("PT WhatsApp");

    rerender(modal(false, whatsappReport));
    rerender(modal(true, telegramReport));

    expect(screen.getByLabelText("Nama pelanggan")).toHaveValue("PT Telegram");
    expect(screen.getByRole("radio", { name: "Telegram saja" })).toBeChecked();
    expect(screen.queryByLabelText("Grup WhatsApp")).not.toBeInTheDocument();
  });
});
