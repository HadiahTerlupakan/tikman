import { useEffect, useMemo, useState } from "react";
import {
  Card,
  Select,
  Tabs,
  Pagination,
  Row,
  Col,
  Spin,
  Input,
  Space,
  DatePicker,
} from "antd";
import { OntTrafficCard } from "@/presentation/components/OntTrafficCard";
import { OltAggregateCard } from "@/presentation/components/OltAggregateCard";
import type { Olt } from "@/domain/entities/Olt";
import { OntStatus, type Ont } from "@/domain/entities/Ont";
import { useOnts } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { SEARCH_DEBOUNCE_MS } from "@/shared/config/limits";
import { useDebouncedValue } from "@/application/hooks/useDebouncedValue";
import { CONTROL_MAX_WIDTH } from "@/presentation/components/controlWidth";

const { Option } = Select;
const { RangePicker } = DatePicker;

type TrafficBucket = "hour" | "day" | "month";

// The list endpoint's ceiling. Without asking for it the page took the default
// of 20, so an OLT with 200 ONTs graphed the first 20 and the search box could
// not find any of the rest.

function getTrafficBucket(): TrafficBucket {
  // Always use "hour" (5-minute buckets) for custom ranges to match period views
  // and preserve traffic detail. Frontend filters empty buckets when calculating stats.
  return "hour";
}

export function GraphsPage() {
  const [selectedOlt, setSelectedOlt] = useState<string | undefined>(undefined);
  const [searchText, setSearchText] = useState("");
  const [selectedStatus, setSelectedStatus] = useState<OntStatus | undefined>(
    undefined,
  );
  const [period, setPeriod] = useState("3h");
  const [dateRange, setDateRange] = useState<
    { start: string; end: string; bucket: TrafficBucket } | undefined
  >(undefined);
  const [page, setPage] = useState(1);
  const pageSize = 9;

  const { data: olts } = useOlts();
  const search = useDebouncedValue(searchText.trim(), SEARCH_DEBOUNCE_MS);

  // The database filters and pages. Searching the rows one request had returned
  // meant an OLT larger than the fetch limit had ONTs the search could not
  // reach, which the page had to apologise for in its own footer.
  const { data: ontsData, isLoading } = useOnts({
    oltId: selectedOlt,
    status: selectedStatus,
    search: search || undefined,
    limit: pageSize,
    offset: (page - 1) * pageSize,
  });

  const paginatedOnts = useMemo(() => ontsData?.data || [], [ontsData]);
  const totalOnts = ontsData?.total ?? 0;

  useEffect(() => {
    setPage(1);
  }, [selectedOlt, search, selectedStatus, dateRange?.start, dateRange?.end]);

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="Traffic Graphs"
        style={{ marginBottom: 24 }}
        extra={
          <Space wrap size={16} align="center">
            <div>
              <span style={{ marginRight: 8, fontSize: 13 }}>OLT:</span>
              <Select
                style={{ width: 200, maxWidth: CONTROL_MAX_WIDTH }}
                placeholder="Select OLT"
                allowClear
                value={selectedOlt}
                onChange={setSelectedOlt}
              >
                {olts?.map((olt: Olt) => (
                  <Option key={olt.id} value={olt.id}>
                    {olt.name}
                  </Option>
                ))}
              </Select>
            </div>
            <Input
              placeholder="Search serial number or ONT name"
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              allowClear
              style={{ width: 260, maxWidth: CONTROL_MAX_WIDTH }}
            />
            <Select
              style={{ width: 160, maxWidth: CONTROL_MAX_WIDTH }}
              placeholder="Select status"
              allowClear
              value={selectedStatus}
              onChange={setSelectedStatus}
            >
              {Object.values(OntStatus).map((status) => (
                <Option key={status} value={status}>
                  {status}
                </Option>
              ))}
            </Select>
            <RangePicker
              style={{ maxWidth: CONTROL_MAX_WIDTH }}
              placeholder={["Start date", "End date"]}
              onChange={(values) => {
                if (values?.[0] && values[1]) {
                  const start = values[0].startOf("day").toISOString();
                  const end = values[1].endOf("day").toISOString();
                  setDateRange({
                    start,
                    end,
                    bucket: getTrafficBucket(),
                  });
                } else {
                  setDateRange(undefined);
                }
              }}
              allowEmpty={[false, false]}
            />
            {/* Bounded so the tab bar collapses its overflow into Ant's own
                "more" menu instead of pushing six periods past the screen. */}
            <div style={{ maxWidth: CONTROL_MAX_WIDTH }}>
              <span style={{ marginRight: 8, fontSize: 13 }}>Period:</span>
              <Tabs
                activeKey={dateRange ? "custom" : period}
                onChange={(key) => {
                  setDateRange(undefined);
                  setPeriod(key);
                }}
                size="small"
                style={{ marginBottom: -16 }}
                items={[
                  ...(dateRange ? [{ key: "custom", label: "Custom" }] : []),
                  { key: "3h", label: "3H" },
                  { key: "6h", label: "6H" },
                  { key: "1d", label: "1D" },
                  { key: "3d", label: "3D" },
                  { key: "7d", label: "7D" },
                  { key: "30d", label: "30D" },
                ]}
              />
            </div>
          </Space>
        }
      >
        <div style={{ fontSize: 12, color: "#666" }}>
          {`Showing ${paginatedOnts.length} of ${totalOnts} ONTs`}
        </div>
      </Card>

      {selectedOlt && !dateRange && (
        <OltAggregateCard
          oltId={selectedOlt}
          oltName={
            olts?.find((olt: Olt) => olt.id === selectedOlt)?.name ?? "OLT"
          }
          period={period}
        />
      )}

      {isLoading ? (
        <div style={{ display: "flex", justifyContent: "center", padding: 60 }}>
          <Spin size="large" />
        </div>
      ) : (
        <>
          <Row gutter={[16, 16]}>
            {paginatedOnts.map((ont: Ont) => (
              <Col key={ont.id} xs={24} sm={24} md={12} lg={8} xl={8}>
                <OntTrafficCard ont={ont} period={period} range={dateRange} />
              </Col>
            ))}
          </Row>

          {totalOnts > pageSize && (
            <div style={{ marginTop: 24, textAlign: "center" }}>
              <Pagination
                current={page}
                pageSize={pageSize}
                total={totalOnts}
                onChange={setPage}
                showSizeChanger={false}
                showTotal={(total) => `Total ${total} ONTs`}
              />
            </div>
          )}
        </>
      )}
    </div>
  );
}
