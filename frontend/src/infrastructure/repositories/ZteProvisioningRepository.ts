import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  ZteGPONRegisterRequest,
  ZteProvisionJob,
  ZteProvisionResponse,
} from "@/domain/entities";

export class ZteProvisioningRepository {
  async register(
    oltId: string,
    data: ZteGPONRegisterRequest,
  ): Promise<ZteProvisionResponse> {
    const response = await apiClient.post<ZteProvisionResponse>(
      API_ENDPOINTS.ZTE_GPON_REGISTER(oltId),
      data,
    );
    return response.data;
  }

  async configureExisting(
    ontId: string,
    data: ZteGPONRegisterRequest,
  ): Promise<ZteProvisionResponse> {
    const response = await apiClient.post<ZteProvisionResponse>(
      API_ENDPOINTS.ZTE_GPON_CONFIGURE(ontId),
      data,
    );
    return response.data;
  }

  async getJob(jobId: string): Promise<ZteProvisionJob> {
    const response = await apiClient.get<{ data: ZteProvisionJob }>(
      API_ENDPOINTS.PROVISION_JOB_BY_ID(jobId),
    );
    return response.data.data;
  }
}
