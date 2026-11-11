import { useMemo } from "react";
import { useLibrenmsGraph } from "@/application/hooks";
import { ImagePreviewModal } from "./ImagePreviewModal";

export interface PreviewTarget {
  deviceId: number;
  ifName: string;
  label: string;
}

interface PortPreviewModalProps {
  target: PreviewTarget | null;
  onClose: () => void;
}

/** One port's section of the report image, titled as it will be sent. */
export function PortPreviewModal({ target, onClose }: PortPreviewModalProps) {
  const { data, isFetching, error } = useLibrenmsGraph(
    target?.deviceId,
    target?.ifName,
    target?.label,
  );
  // A fresh array each render would mint new object URLs and remount the image.
  const images = useMemo(() => (data ? [{ image: data }] : undefined), [data]);

  return (
    <ImagePreviewModal
      open={!!target}
      title={`Pratinjau — ${target?.label ?? ""}`}
      alt={`Grafik trafik ${target?.label ?? ""}`}
      images={images}
      loading={isFetching}
      error={error}
      onClose={onClose}
    />
  );
}
