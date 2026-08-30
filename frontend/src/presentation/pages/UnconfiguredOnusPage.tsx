import { useState } from "react";
import { Card, Select, Button, Space, Alert, Empty, App } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import {
  useOlts,
  useUnconfiguredOnus,
  useZteGPONRegister,
} from "@/application/hooks";
import { ZteProvisionModal } from "@/presentation/components/zte-provisioning";
import type { DetectedOnu, ZteProvisionTarget } from "@/domain/entities";
import { UnconfiguredOnuTable } from "@/presentation/components/UnconfiguredOnuTable";

export default function UnconfiguredOnusPage() {
  const { message } = App.useApp();
  const { data: olts, isLoading: isLoadingOlts } = useOlts();
  const [oltFilter, setOltFilter] = useState<string>();
  const [registerTarget, setRegisterTarget] =
    useState<ZteProvisionTarget | null>(null);
  const registerMutation = useZteGPONRegister();

  const scan = useUnconfiguredOnus(olts);
  const rows = oltFilter
    ? scan.rows.filter((row) => row.oltId === oltFilter)
    : scan.rows;

  const handleRegister = (onu: DetectedOnu) => {
    // The OLT comes from the row, not from the filter: a row always knows which
    // OLT detected it, and a filter can be changed between reading and clicking.
    setRegisterTarget({
      oltId: onu.oltId,
      card: onu.slot,
      pon: onu.port,
      serialNumber: onu.serialNumber,
      onuType: onu.deviceType,
    });
  };

  const handleCopySerial = async (serialNumber: string) => {
    try {
      await navigator.clipboard.writeText(serialNumber);
      message.success(`Copied ${serialNumber}`);
    } catch {
      message.error("Could not copy the serial number");
    }
  };

  return (
    <div style={{ padding: 24 }}>
      <Card
        title="Unconfigured ONU"
        extra={
          <Space>
            <Select
              style={{ minWidth: 220 }}
              placeholder="All OLTs"
              allowClear
              loading={isLoadingOlts}
              value={oltFilter}
              onChange={setOltFilter}
              options={olts?.map((olt) => ({
                label: olt.name,
                value: olt.id,
              }))}
            />
            <Button
              icon={<ReloadOutlined />}
              loading={scan.isFetching}
              disabled={!olts?.length}
              onClick={scan.rescan}
            >
              Scan
            </Button>
          </Space>
        }
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="ONUs detected optically by the OLT that have no provisioning config yet. Every OLT is scanned, so an ONU waiting at any site shows up here. An entry disappears once its serial number is registered on the OLT."
        />

        {scan.failed.length > 0 && (
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 16 }}
            message={`Could not scan ${scan.failed.join(", ")}`}
            // An empty table would otherwise read as "nothing waiting" when the
            // truth is that these OLTs were never asked.
            description="Any ONU waiting at these OLTs is missing from the list below, not absent."
          />
        )}

        {olts?.length ? (
          <>
            <UnconfiguredOnuTable
              dataSource={rows}
              isLoading={scan.isLoading}
              showOlt={!oltFilter}
              onCopySerial={handleCopySerial}
              onRegister={handleRegister}
            />
            {registerTarget && (
              <ZteProvisionModal
                open
                mode="register"
                target={registerTarget}
                onClose={() => setRegisterTarget(null)}
                onSubmit={(request) => {
                  registerMutation.mutate(
                    { oltId: registerTarget.oltId, data: request },
                    {
                      onSuccess: () => {
                        message.success("ONU registration started");
                        setRegisterTarget(null);
                        scan.rescan();
                      },
                      onError: (submitError) =>
                        message.error(submitError.message),
                    },
                  );
                }}
                loading={registerMutation.isPending}
                error={registerMutation.error}
              />
            )}
          </>
        ) : (
          <Empty description="Register an OLT before scanning for ONUs" />
        )}
      </Card>
    </div>
  );
}
