import { useEffect, useMemo } from "react";
import { Alert, Modal, Spin, Typography } from "antd";

interface ImagePreviewModalProps {
  open: boolean;
  title: string;
  alt: string;
  image?: Blob;
  loading: boolean;
  error: Error | null;
  /** Shown under the image the way WhatsApp shows a caption. */
  caption?: string;
  onClose: () => void;
}

/** The PNG arrives as a Blob because the endpoints sit behind the session
 * cookie, which a plain <img src> to the API would not carry. */
export function ImagePreviewModal({
  open,
  title,
  alt,
  image,
  loading,
  error,
  caption,
  onClose,
}: ImagePreviewModalProps) {
  const url = useMemo(
    () => (image ? URL.createObjectURL(image) : undefined),
    [image],
  );

  useEffect(
    () => () => {
      if (url) URL.revokeObjectURL(url);
    },
    [url],
  );

  return (
    <Modal
      open={open}
      title={title}
      footer={null}
      width={960}
      onCancel={onClose}
      destroyOnHidden
    >
      {loading && (
        <div style={{ textAlign: "center", padding: 24 }}>
          <Spin />
        </div>
      )}
      {error && <Alert type="error" showIcon message={error.message} />}
      {url && !loading && (
        <>
          <img src={url} alt={alt} style={{ width: "100%", height: "auto" }} />
          {caption && (
            <Typography.Paragraph
              style={{ whiteSpace: "pre-wrap", marginTop: 12, marginBottom: 0 }}
            >
              {caption}
            </Typography.Paragraph>
          )}
        </>
      )}
    </Modal>
  );
}
