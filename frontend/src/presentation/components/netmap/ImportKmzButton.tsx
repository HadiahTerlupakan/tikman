import { useState } from "react";
import { ImportOutlined } from "@ant-design/icons";
import { Button } from "antd";
import { ImportKmzModal } from "./ImportKmzModal";

/**
 * The map toolbar's own trigger for importing a KMZ - self-contained (owns
 * its own open/closed state) so MapToolbar needs no new prop for it, the
 * same reason useKmzExport's download button lives entirely behind one
 * prop-free call.
 */
export function ImportKmzButton() {
  const [open, setOpen] = useState(false);
  return (
    <>
      <Button icon={<ImportOutlined />} onClick={() => setOpen(true)}>
        Impor KMZ
      </Button>
      <ImportKmzModal open={open} onClose={() => setOpen(false)} />
    </>
  );
}
