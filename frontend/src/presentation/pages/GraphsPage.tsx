import { useEffect, useMemo, useState } from "react";
import { Card, Select, Tabs, Pagination, Row, Col, Spin, Input, Space } from "antd";
import { OntTrafficCard } from "@/presentation/components/OntTrafficCard";
import type { Olt } from "@/domain/entities/Olt";
import type { Ont } from "@/domain/entities/Ont";
import { useOnts } from "@/application/hooks/useOnts";
import { useOlts } from "@/application/hooks/useOlts";
import { filterOntsByQuery } from "./graphsFilter";

const { Option } = Select;

export function GraphsPage() {
  const [selectedOlt, setSelectedOlt] = useState<string | undefined>(undefined);
  const [searchText, setSearchText] = useState("");
  const [period, setPeriod] = useState("3h");
  const [page, setPage] = useState(1);
  const pageSize = 9;

  const { data: olts } = useOlts();
  const { data: ontsData, isLoading } = useOnts({ oltId: selectedOlt });

  const filteredOnts = useMemo(
    () => filterOntsByQuery(ontsData?.data || [], searchText),
    [ontsData, searchText]
  );
  const totalOnts = filteredOnts.length;

  useEffect(() => {
    setPage(1);
  }, [selectedOlt, searchText]);

  const paginatedOnts = filteredOnts.slice((page - 1) * pageSize, page * pageSize);

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
            <div>
              <span style={{ marginRight: 8, fontSize: 13 }}>Period:</span>
              <Tabs
                activeKey={period}
                onChange={setPeriod}
                size="small"
                style={{ marginBottom: -16 }}
                items={[
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
        <div style={{ fontSize: 12, color: '#666' }}>
          Showing {paginatedOnts.length} of {totalOnts} ONTs
        </div>
      </Card>

      {isLoading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 60 }}>
          <Spin size="large" />
        </div>
      ) : (
        <>
          <Row gutter={[16, 16]}>
            {paginatedOnts.map((ont: Ont) => (
              <Col key={ont.id} xs={24} sm={24} md={12} lg={8} xl={8}>
                <OntTrafficCard ont={ont} period={period} />
              </Col>
            ))}
          </Row>

          {totalOnts > pageSize && (
            <div style={{ marginTop: 24, textAlign: 'center' }}>
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
