import { Fragment, useEffect, useMemo } from "react";
import { Alert, Modal, Spin, Typography } from "antd";

interface ImagePreviewModalProps {
  open: boolean;
  title: string;
  alt: string;
  /** In the order they are sent. */
  images?: Blob[];
  loading: boolean;
  error: Error | null;
  /** Shown under the last image, the only one sent with the caption. */
  caption?: string;
  onClose: () => void;
}

/** The PNG arrives as a Blob because the endpoints sit behind the session
 * cookie, which a plain <img src> to the API would not carry. */
export function ImagePreviewModal({
  open,
  title,
  alt,
  images,
  loading,
  error,
  caption,
  onClose,
}: ImagePreviewModalProps) {
  const urls = useMemo(
    () => (images ?? []).map((image) => URL.createObjectURL(image)),
    [images],
  );

  useEffect(
    () => () => {
      urls.forEach((url) => URL.revokeObjectURL(url));
    },
    [urls],
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
      {!loading &&
        urls.map((url, index) => (
          <Fragment key={url}>
            {urls.length > 1 && (
              <Typography.Text
                type="secondary"
                style={{ display: "block", marginTop: index ? 24 : 0 }}
              >
                Gambar {index + 1} dari {urls.length}
              </Typography.Text>
            )}
            <img
              src={url}
              alt={`${alt} ${index + 1}`}
              style={{ width: "100%", height: "auto" }}
            />
            {index === urls.length - 1 && caption && (
              <Typography.Paragraph
                style={{
                  whiteSpace: "pre-wrap",
                  marginTop: 12,
                  marginBottom: 0,
                }}
              >
                {caption}
              </Typography.Paragraph>
            )}
          </Fragment>
        ))}
    </Modal>
  );
}
