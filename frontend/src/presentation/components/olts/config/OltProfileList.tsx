import { Alert, Empty, Table } from "antd";

interface OltProfileListProps {
  title: string;
  names: string[];
  emptyText: string;
  note?: string;
}

// The profile tabs all show the same thing: a list of names the OLT is using.
// Their contents come from different reads, but none of them is editable here,
// so one table serves all of them.
export function OltProfileList({
  title,
  names,
  emptyText,
  note,
}: OltProfileListProps) {
  return (
    <>
      {note && (
        <Alert
          type="info"
          showIcon
          message={note}
          style={{ marginBottom: 16 }}
        />
      )}
      {names.length === 0 ? (
        <Empty description={emptyText} />
      ) : (
        <Table<{ name: string }>
          size="small"
          rowKey="name"
          dataSource={names.map((name) => ({ name }))}
          pagination={false}
          columns={[{ title, dataIndex: "name" }]}
        />
      )}
    </>
  );
}
