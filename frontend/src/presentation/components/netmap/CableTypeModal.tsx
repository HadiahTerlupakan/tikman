import { Modal, Select } from "antd";
import { useState } from "react";
import type { FiberType } from "@/domain/entities";
import { FIBER_LABELS } from "./mappingLabels";

interface CableTypeModalProps {
  open: boolean;
  onCancel: () => void;
  onSubmit: (fiberType: FiberType) => void;
}

const OPTIONS = (Object.keys(FIBER_LABELS) as FiberType[]).map((value) => ({
  value,
  label: FIBER_LABELS[value],
}));

/**
 * Which of the seven fiber types a just-traced cable is. All seven stay
 * choosable here on purpose: odp_to_odp and odc_to_odc carry their own
 * capacity rules, and hardcoding this away from the interface would make
 * those rules unreachable from the map.
 */
export function CableTypeModal({
  open,
  onCancel,
  onSubmit,
}: CableTypeModalProps) {
  const [fiberType, setFiberType] = useState<FiberType>("distribution");

  return (
    <Modal
      open={open}
      title="Jenis kabel"
      onCancel={onCancel}
      onOk={() => onSubmit(fiberType)}
      okText="Simpan"
      cancelText="Batal"
      destroyOnClose
    >
      <Select<FiberType>
        style={{ width: "100%" }}
        value={fiberType}
        onChange={setFiberType}
        options={OPTIONS}
      />
    </Modal>
  );
}
