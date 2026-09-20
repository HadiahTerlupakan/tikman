import { useState } from "react";
import { UploadOutlined } from "@ant-design/icons";
import { Alert, Button, Modal, Space, Typography, Upload } from "antd";
import type {
  ImportedEdge,
  ImportedNode,
  ImportPreview,
} from "@/domain/entities";
import { ImportEdgesTable } from "./ImportEdgesTable";
import { ImportNodesTable } from "./ImportNodesTable";
import { useKmzImport } from "./useKmzImport";

interface ImportKmzModalProps {
  open: boolean;
  onClose: () => void;
}

function summaryLine(preview: ImportPreview): string {
  return `${preview.totalPlacemarks} placemark ditemukan: ${preview.nodes.length} node, ${preview.edges.length} kabel, ${preview.issues.length} tidak dikenali.`;
}

function hasIncludedRow(preview: ImportPreview): boolean {
  return (
    preview.nodes.some((n) => n.include) || preview.edges.some((e) => e.include)
  );
}

function IssuesAlert({ issues }: { issues: ImportPreview["issues"] }) {
  if (issues.length === 0) {
    return null;
  }
  return (
    <Alert
      type="warning"
      showIcon
      message={`${issues.length} placemark tidak dikenali dan tidak akan diimpor`}
      description={
        <ul style={{ margin: 0, paddingLeft: 20 }}>
          {issues.map((issue) => (
            <li key={issue.row}>
              {issue.name || "(tanpa nama)"} &mdash; {issue.reason}
            </li>
          ))}
        </ul>
      }
    />
  );
}

// The upload step before anything has been parsed: choosing a file is not
// yet asking for a preview, so a second, explicit tap is what actually
// sends it - mirroring BroadcastModal/MessageComposer's own beforeUpload
// (capture the File, return false, do nothing network-bound yet).
function UploadStep({
  onPreview,
  isPreviewing,
}: {
  onPreview: (file: File) => void;
  isPreviewing: boolean;
}) {
  const [file, setFile] = useState<File>();
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Typography.Paragraph>
        Unggah berkas .kmz. Tidak ada yang disimpan ke peta sampai Anda menekan
        &quot;Simpan ke Peta&quot; pada langkah berikutnya.
      </Typography.Paragraph>
      <Upload
        maxCount={1}
        accept=".kmz"
        beforeUpload={(chosen) => {
          setFile(chosen);
          return false;
        }}
        onRemove={() => setFile(undefined)}
      >
        <Button icon={<UploadOutlined />}>Pilih Berkas KMZ</Button>
      </Upload>
      <Button
        type="primary"
        disabled={!file}
        loading={isPreviewing}
        onClick={() => file && onPreview(file)}
      >
        Pratinjau
      </Button>
    </Space>
  );
}

interface PreviewStepProps {
  preview: ImportPreview;
  onChangeNode: (row: number, patch: Partial<ImportedNode>) => void;
  onChangeEdge: (row: number, patch: Partial<ImportedEdge>) => void;
}

// What was found, laid out for correction - the screen this whole design
// exists for. Split from ImportKmzModal itself only to keep that function
// under the project's line guideline; it owns no state of its own.
function PreviewStep({
  preview,
  onChangeNode,
  onChangeEdge,
}: PreviewStepProps) {
  return (
    <Space direction="vertical" style={{ width: "100%" }} size="middle">
      <Alert type="info" showIcon message={summaryLine(preview)} />
      <ImportNodesTable nodes={preview.nodes} onChange={onChangeNode} />
      <ImportEdgesTable edges={preview.edges} onChange={onChangeEdge} />
      <IssuesAlert issues={preview.issues} />
    </Space>
  );
}

interface ModalFooterProps {
  preview?: ImportPreview;
  onCancel: () => void;
  onCommit: () => void;
  isCommitting: boolean;
}

// No footer at all during the upload step: there is nothing yet to cancel
// out of that a plain [X] close does not already cover, and no commit to
// offer before a preview exists.
function ModalFooter({
  preview,
  onCancel,
  onCommit,
  isCommitting,
}: ModalFooterProps) {
  if (!preview) {
    return null;
  }
  return (
    <>
      <Button key="cancel" onClick={onCancel}>
        Batal
      </Button>
      <Button
        key="commit"
        type="primary"
        disabled={!hasIncludedRow(preview)}
        loading={isCommitting}
        onClick={onCommit}
      >
        Simpan ke Peta
      </Button>
    </>
  );
}

/**
 * The whole point of this design: nothing from a .kmz is written until this
 * screen's own "Simpan ke Peta" is pressed. What arrives already correct
 * (our own export) is just confirmed here; a field survey or a vendor's
 * file gets the same review, because a wrong guess about type or fibre
 * would otherwise write bad data into a network that is already running.
 */
export function ImportKmzModal({ open, onClose }: ImportKmzModalProps) {
  const {
    preview,
    runPreview,
    updateNode,
    updateEdge,
    commit,
    reset,
    isPreviewing,
    isCommitting,
  } = useKmzImport();

  const close = () => {
    reset();
    onClose();
  };
  const handleCommit = async () => (await commit()) && onClose();

  return (
    <Modal
      open={open}
      title="Impor Peta dari KMZ"
      onCancel={close}
      width={900}
      destroyOnClose
      footer={
        <ModalFooter
          preview={preview}
          onCancel={close}
          onCommit={handleCommit}
          isCommitting={isCommitting}
        />
      }
    >
      {!preview && (
        <UploadStep onPreview={runPreview} isPreviewing={isPreviewing} />
      )}
      {preview && (
        <PreviewStep
          preview={preview}
          onChangeNode={updateNode}
          onChangeEdge={updateEdge}
        />
      )}
    </Modal>
  );
}
