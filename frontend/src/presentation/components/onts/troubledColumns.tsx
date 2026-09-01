import { Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { TroubledOnt } from "@/domain/entities";
import { OutageBar } from "./OutageBar";

/**
 * troubledColumns builds the ranked table.
 *
 * The trap count is coloured against the worst row on the page rather than
 * against a threshold picked out of the air: a ranked list already claims an
 * order, and colouring by rank says the same thing without inventing a number
 * that means nothing to this network.
 */
export function troubledColumns(
  worstTrapCount: number,
  windowMinutes: number,
  errorColor: string,
  warningColor: string,
): ColumnsType<TroubledOnt> {
  const intensity = (count: number) =>
    worstTrapCount > 0 ? count / worstTrapCount : 0;

  return [
    { title: "OLT", dataIndex: "oltName", width: 100 },
    {
      title: "Posisi",
      width: 110,
      render: (_, r) => (
        <span style={{ fontVariantNumeric: "tabular-nums" }}>
          ONU-{r.portId}:{r.ontNumber}
        </span>
      ),
    },
    { title: "Pelanggan", dataIndex: "name", ellipsis: true },
    {
      title: "Serial",
      dataIndex: "serialNumber",
      width: 145,
      render: (serial: string) => (
        <span style={{ fontFamily: "monospace", fontSize: 12 }}>{serial}</span>
      ),
    },
    {
      title: "Trap",
      dataIndex: "trapCount",
      width: 100,
      align: "right",
      defaultSortOrder: "descend",
      sorter: (a, b) => a.trapCount - b.trapCount,
      render: (count: number) => {
        const share = intensity(count);
        return (
          <span
            style={{
              fontVariantNumeric: "tabular-nums",
              fontWeight: share > 0.33 ? 600 : 400,
              color:
                share > 0.66
                  ? errorColor
                  : share > 0.33
                    ? warningColor
                    : undefined,
            }}
          >
            {count.toLocaleString("id-ID")}
          </span>
        );
      },
    },
    {
      title: "Waktu mati",
      dataIndex: "downMinutes",
      width: 130,
      sorter: (a, b) => a.downMinutes - b.downMinutes,
      render: (minutes: number) => (
        <OutageBar minutes={minutes} windowMinutes={windowMinutes} />
      ),
    },
    {
      title: "Status",
      dataIndex: "status",
      width: 105,
      render: (status: string) => (
        <Tag color={status === "online" ? "green" : "red"}>
          {status.toUpperCase()}
        </Tag>
      ),
    },
  ];
}

/**
 * readsFineButIsNot marks a subscriber the status column would clear.
 *
 * Online now and down earlier in the same window: the row the ONT list shows as
 * healthy every time anyone looks, which is the reason this page exists. The
 * test is exact rather than a threshold — it either lost service in the window
 * or it did not.
 */
export function readsFineButIsNot(ont: TroubledOnt): boolean {
  return ont.status === "online" && ont.downMinutes > 0;
}
