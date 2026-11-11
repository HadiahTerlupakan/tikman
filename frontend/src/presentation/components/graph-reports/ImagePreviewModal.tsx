import { Fragment, useEffect, useMemo } from "react";
import { Alert, Modal, Spin, Typography } from "antd";

interface ImagePreviewModalProps {
  open: boolean;
  title: string;
  alt: string;
  /** In the order they are sent; a caption is shown under its image the way
   * WhatsApp shows one. */
  images?: { image: Blob; caption?: string }[];
  loading: boolean;
  error: Error | null;
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
  onClose,
}: ImagePreviewModalProps) {
  const shown = useMemo(
    () =>
      (images ?? []).map(({ image, caption }) => ({
        url: URL.createObjectURL(image),
        caption,
      })),
    [images],
  );

  useEffect(
    () => () => {
      shown.forEach(({ url }) => URL.revokeObjectURL(url));
    },
    [shown],
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
        shown.map(({ url, caption }, index) => (
          <Fragment key={url}>
            {shown.length > 1 && (
              <Typography.Text
                type="secondary"
                style={{ display: "block", marginTop: index ? 24 : 0 }}
              >
                Gambar {index + 1} dari {shown.length}
              </Typography.Text>
            )}
            <img
              src={url}
              alt={`${alt} ${index + 1}`}
              style={{ width: "100%", height: "auto" }}
            />
            {caption && (
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
