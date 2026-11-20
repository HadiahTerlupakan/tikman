import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  CreateSwitchDto,
  ManagedSwitch,
  SwitchBackup,
  SwitchBackupSummary,
  SwitchConnectionTestResult,
  SwitchInterface,
  SwitchNeighbor,
  SwitchVlan,
  UpdateSwitchDto,
} from "@/domain/entities";

const HTTP_OK = 200;
const HTTP_CONFLICT = 409;

export class SwitchRepository {
  async list(): Promise<ManagedSwitch[]> {
    return (await apiClient.get(API_ENDPOINTS.SWITCHES)).data;
  }

  async create(data: CreateSwitchDto): Promise<ManagedSwitch> {
    return (await apiClient.post(API_ENDPOINTS.SWITCHES, data)).data;
  }

  async update(id: string, data: UpdateSwitchDto): Promise<ManagedSwitch> {
    return (await apiClient.put(API_ENDPOINTS.SWITCH_BY_ID(id), data)).data;
  }

  async interfaces(id: string): Promise<SwitchInterface[]> {
    return (await apiClient.get(API_ENDPOINTS.SWITCH_INTERFACES(id))).data;
  }

  async vlans(id: string): Promise<SwitchVlan[]> {
    return (await apiClient.get(API_ENDPOINTS.SWITCH_VLANS(id))).data;
  }

  async neighbors(id: string): Promise<SwitchNeighbor[]> {
    return (await apiClient.get(API_ENDPOINTS.SWITCH_NEIGHBORS(id))).data;
  }

  async backups(id: string): Promise<SwitchBackupSummary[]> {
    return (await apiClient.get(API_ENDPOINTS.SWITCH_BACKUPS(id))).data;
  }

  async backup(id: string, backupId: string): Promise<SwitchBackup> {
    return (await apiClient.get(API_ENDPOINTS.SWITCH_BACKUP(id, backupId)))
      .data;
  }

  async backupDiff(id: string, from: string, to: string): Promise<string> {
    const response = await apiClient.get(API_ENDPOINTS.SWITCH_BACKUP_DIFF(id), {
      params: { from, to },
    });
    return response.data.diff;
  }

  async backupNow(id: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.SWITCH_BACKUP_NOW(id));
  }

  async trustHostKey(id: string, fingerprint: string): Promise<ManagedSwitch> {
    return (
      await apiClient.post(API_ENDPOINTS.SWITCH_TRUST_HOST_KEY(id), {
        fingerprint,
      })
    ).data;
  }

  // An unpinned SSH key answers 409 with the fingerprint to show; the error
  // mapper would keep only the message, so the 409 is read as an answer.
  async testConnection(
    data: CreateSwitchDto,
  ): Promise<SwitchConnectionTestResult> {
    const response = await apiClient.post(API_ENDPOINTS.SWITCH_TEST, data, {
      validateStatus: (status) =>
        status === HTTP_OK || status === HTTP_CONFLICT,
    });
    if (response.status === HTTP_CONFLICT) {
      return {
        kind: "unpinned",
        presentedFingerprint: response.data.presentedFingerprint,
      };
    }
    return {
      kind: "identified",
      model: response.data.model,
      version: response.data.version,
    };
  }
}
