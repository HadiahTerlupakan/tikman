import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  GraphReport,
  GraphReportInput,
  LibrenmsDevice,
  LibrenmsPort,
} from "@/domain/entities";

const HTTP_OK = 200;

// A send fetches up to ten graphs one after another, each allowed 30s by the
// server, which itself gives up at six minutes. Waiting a little longer than
// that lets the server's reason reach the admin instead of a client timeout.
const SEND_TIMEOUT_MS = 390_000;

// FileReader rather than Blob.text(): jsdom's Blob has no text(), and the
// tests run there.
function readText(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result));
    reader.onerror = () => reject(reader.error);
    reader.readAsText(blob);
  });
}

// A failed graph request answers JSON, but asked for as a blob it arrives as
// one, and mapApiError cannot read a Blob: it would report "UNKNOWN_ERROR"
// where the server said "LibreNMS belum diatur".
async function blobErrorMessage(blob: Blob, status: number): Promise<string> {
  const fallback = `Pratinjau gagal dimuat (HTTP ${status})`;
  try {
    const body = JSON.parse(await readText(blob)) as { error?: string };
    return body.error || fallback;
  } catch {
    return fallback;
  }
}

/** The daily LibreNMS graph reports and the LibreNMS lookups that build one. */
export class GraphReportRepository {
  async list(): Promise<GraphReport[]> {
    const response = await apiClient.get(API_ENDPOINTS.GRAPH_REPORTS);
    return response.data.data ?? [];
  }

  async create(input: GraphReportInput): Promise<void> {
    await apiClient.post(API_ENDPOINTS.GRAPH_REPORTS, input);
  }

  async update(id: string, input: GraphReportInput): Promise<void> {
    await apiClient.put(API_ENDPOINTS.GRAPH_REPORT(id), input);
  }

  async remove(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.GRAPH_REPORT(id));
  }

  /** Resolves once the images are queued; rejects with the server's reason. */
  async send(id: string): Promise<string> {
    const response = await apiClient.post(
      API_ENDPOINTS.GRAPH_REPORT_SEND(id),
      undefined,
      { timeout: SEND_TIMEOUT_MS },
    );
    return response.data.message ?? "";
  }

  async devices(): Promise<LibrenmsDevice[]> {
    const response = await apiClient.get(API_ENDPOINTS.LIBRENMS_DEVICES);
    return response.data.data ?? [];
  }

  async ports(deviceId: number): Promise<LibrenmsPort[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.LIBRENMS_DEVICE_PORTS(deviceId),
    );
    return response.data.data ?? [];
  }

  async graph(deviceId: number, ifName: string): Promise<Blob> {
    const response = await apiClient.get(API_ENDPOINTS.LIBRENMS_GRAPH, {
      params: { device_id: deviceId, if_name: ifName },
      responseType: "blob",
      validateStatus: () => true,
    });
    if (response.status !== HTTP_OK) {
      throw new Error(await blobErrorMessage(response.data, response.status));
    }
    return response.data;
  }
}
