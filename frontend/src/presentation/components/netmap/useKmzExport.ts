import { message } from "antd";
import { useExportMapping } from "@/application/hooks";
import { downloadFile } from "./downloadFile";

const KMZ_FILENAME = "peta-jaringan.kmz";

/**
 * The map's own KMZ download: fetches the archive and hands it to the
 * browser to save, or reports the one way this can fail. Kept out of
 * MapToolbar itself so that component's own function stays about rendering.
 *
 * A cable dropped for a missing endpoint (migration 53's dangling-edge
 * state) must not disappear without telling the person downloading the
 * file - the backend already composed that one sentence; this only relays
 * it, and only when it is non-empty, since a clean export must say nothing
 * extra.
 */
export function useKmzExport() {
  const exportMapping = useExportMapping();

  const handleExport = async () => {
    try {
      const { blob, warning } = await exportMapping.mutateAsync();
      downloadFile(blob, KMZ_FILENAME);
      if (warning) {
        message.warning(warning);
      }
    } catch {
      message.error("Gagal mengunduh KMZ");
    }
  };

  return { handleExport, isPending: exportMapping.isPending };
}
