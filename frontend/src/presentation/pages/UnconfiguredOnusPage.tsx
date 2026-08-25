import { useEffect, useState } from "react";
import { Card, Select, Button, Space, Alert, Empty, App } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import {
  useOlts,
  useUnconfiguredOnus,
  useZteGPONRegister,
} from "@/application/hooks";
import { ZteProvisionModal } from "@/presentation/components/zte-provisioning";
import type { UnconfiguredOnu, ZteProvisionTarget } from "@/domain/entities";
import { UnconfiguredOnuTable } from "@/presentation/components/UnconfiguredOnuTable";

export default function UnconfiguredOnusPage() {
  const { message } = App.useApp();
  const { data: olts, isLoading: isLoadingOlts } = useOlts();
  const [selectedOltId, setSelectedOltId] = useState<string>();
  const [registerTarget, setRegisterTarget] =
    useState<ZteProvisionTarget | null>(null);
  const registerMutation = useZteGPONRegister();

  useEffect(() => {
    if (!selectedOltId && olts?.length) setSelectedOltId(olts[0].id);
  }, [olts, selectedOltId]);

  const { data, isLoading, isFetching, error, refetch } =
    useUnconfiguredOnus(selectedOltId);

  const handleRegister = (onu: UnconfiguredOnu) => {
    if (!selectedOltId) return;
    setRegisterTarget({
      oltId: selectedOltId,
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
              placeholder="Select OLT"
              loading={isLoadingOlts}
              value={selectedOltId}
              onChange={setSelectedOltId}
              options={olts?.map((olt) => ({ label: olt.name, value: olt.id }))}
            />
            <Button
              icon={<ReloadOutlined />}
              loading={isFetching}
              disabled={!selectedOltId}
              onClick={() => refetch()}
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
          message="ONUs detected optically by the OLT that have no provisioning config yet. An entry disappears once its serial number is registered on the OLT."
        />
        {error && (
          <Alert
            type="error"
            showIcon
            style={{ marginBottom: 16 }}
            message="Scan failed"
            description={
              error instanceof Error ? error.message : "SNMP scan failed"
            }
          />
        )}
        {selectedOltId ? (
          <>
            <UnconfiguredOnuTable
              dataSource={data ?? []}
              isLoading={isLoading}
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
                        refetch();
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
          <Empty description="Select an OLT to scan" />
        )}
      </Card>
    </div>
  );
}
