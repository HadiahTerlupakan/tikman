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
import type { Olt } from "@/domain/entities/Olt";
import { OntStatus, type Ont } from "@/domain/entities/Ont";
import { useOnts } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { filterOntsByQuery } from "./graphsFilter";

const { Option } = Select;
const { RangePicker } = DatePicker;

type TrafficBucket = "hour" | "day" | "month";

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
  const { data: ontsData, isLoading } = useOnts({
    oltId: selectedOlt,
    status: selectedStatus,
  });

  const filteredOnts = useMemo(() => {
    return filterOntsByQuery(ontsData?.data || [], searchText);
  }, [ontsData, searchText]);
  const totalOnts = filteredOnts.length;

  useEffect(() => {
    setPage(1);
  }, [
    selectedOlt,
    searchText,
    selectedStatus,
    dateRange?.start,
    dateRange?.end,
  ]);

  const paginatedOnts = filteredOnts.slice(
    (page - 1) * pageSize,
    page * pageSize,
  );

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
                style={{ width: 200 }}
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
              style={{ width: 260 }}
            />
            <Select
              style={{ width: 160 }}
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
            <div>
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
          Showing {paginatedOnts.length} of {totalOnts} ONTs
        </div>
      </Card>

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
