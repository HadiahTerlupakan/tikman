import { Button, Dropdown, Space, Tooltip } from "antd";
import type { MenuProps } from "antd";
import {
  DeleteOutlined,
  EllipsisOutlined,
  EyeOutlined,
  HistoryOutlined,
  SettingOutlined,
  ThunderboltOutlined,
} from "@ant-design/icons";
import type { Ont } from "@/domain/entities";

interface OntActionsProps {
  ont: Ont;
  onViewDetail: (ont: Ont) => void;
  onDelete: (id: string) => void;
  onProvision?: (ont: Ont) => void;
  onConfigureService?: (ont: Ont) => void;
  onViewHistory?: (ont: Ont) => void;
}

// Five labelled buttons per row pushed the ONT table into horizontal scroll and
// read as a wall of blue text. The two an operator reaches for stay as icons;
// the rest, delete included, sit behind the overflow.
export function OntActions({
  ont,
  onViewDetail,
  onDelete,
  onProvision,
  onConfigureService,
  onViewHistory,
}: OntActionsProps) {
  // The page owns the confirmation, because it fetches the commands a removal
  // would send to the OLT and shows them before anything is done.

  const items: MenuProps["items"] = [
    ...(onProvision
      ? [
          {
            key: "provision",
            icon: <ThunderboltOutlined />,
            label: "Provision",
            onClick: () => onProvision(ont),
          },
        ]
      : []),
    ...(onViewHistory
      ? [
          {
            key: "history",
            icon: <HistoryOutlined />,
            label: "History",
            onClick: () => onViewHistory(ont),
          },
        ]
      : []),
    { type: "divider" as const },
    {
      key: "delete",
      icon: <DeleteOutlined />,
      label: "Delete",
      danger: true,
      onClick: () => onDelete(ont.id),
    },
  ];

  return (
    <Space size={4}>
      <Tooltip title="View details">
        <Button
          type="text"
          aria-label="View details"
          // Queried by test id, not by label: antd injects a stylesheet jsdom's
          // selector engine rejects, and accessibility-tree queries trip on it.
          data-testid="ont-view"
          icon={<EyeOutlined />}
          onClick={() => onViewDetail(ont)}
        />
      </Tooltip>
      {onConfigureService && (
        <Tooltip title="Configure service">
          <Button
            type="text"
            aria-label="Configure service"
            data-testid="ont-configure"
            icon={<SettingOutlined />}
            onClick={() => onConfigureService(ont)}
          />
        </Tooltip>
      )}
      <Dropdown trigger={["click"]} menu={{ items }} placement="bottomRight">
        <Button
          type="text"
          aria-label="More actions"
          data-testid="ont-more"
          icon={<EllipsisOutlined />}
        />
      </Dropdown>
    </Space>
  );
}
