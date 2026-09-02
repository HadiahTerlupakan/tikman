import { Select } from "antd";
import { UserRole } from "@/domain/entities";
import type { User } from "@/domain/entities";

// Whoever the thread goes to has to be able to open the inbox: a viewer is
// 403'd on every route in the module, so handing them a thread would park a
// customer with someone who cannot even read them.
const CAN_HOLD_A_THREAD = [
  UserRole.ADMIN,
  UserRole.CS,
  UserRole.TECHNICIAN,
] as const;

interface TransferPickerProps {
  users: User[];
  /** Who holds it now — offering them the thread they already have is noise. */
  holderId?: string;
  transferring?: boolean;
  onTransfer: (userId: string) => void;
}

/**
 * "Oper ke CS lain" from the spec. The value is never kept: picking a name is
 * the action itself, not a field being filled in, so the box goes back to its
 * placeholder and the list below reports where the thread actually went.
 */
export function TransferPicker({
  users,
  holderId,
  transferring = false,
  onTransfer,
}: TransferPickerProps) {
  const options = users
    .filter(
      (user) =>
        CAN_HOLD_A_THREAD.includes(
          user.role as (typeof CAN_HOLD_A_THREAD)[number],
        ) && user.id !== holderId,
    )
    .map((user) => ({ value: user.id, label: user.username }));

  return (
    <Select
      showSearch
      optionFilterProp="label"
      style={{ width: 220 }}
      placeholder="Oper ke CS lain"
      loading={transferring}
      value={null}
      options={options}
      onChange={onTransfer}
    />
  );
}
