import { message } from "antd";
import { useExportMapping } from "@/application/hooks";
import { downloadFile } from "./downloadFile";

const KMZ_FILENAME = "peta-jaringan.kmz";

/**
 * The map's own KMZ download: fetches the archive and hands it to the
 * browser to save, or reports the one way this can fail. Kept out of
 * MapToolbar itself so that component's own function stays about rendering.
 */
export function useKmzExport() {
  const exportMapping = useExportMapping();

  const handleExport = async () => {
    try {
      const kmz = await exportMapping.mutateAsync();
      downloadFile(kmz, KMZ_FILENAME);
    } catch {
      message.error("Gagal mengunduh KMZ");
    }
  };

  return { handleExport, isPending: exportMapping.isPending };
}
