export interface ProvisionJob {
  id: string;
  ontId: string;
  templateId?: string;
  status: "pending" | "running" | "success" | "failed" | "rolled_back";
  errorMessage?: string;
  createdAt: string;
  completedAt?: string;
}

export interface ProvisionRequest {
  templateId?: string;
  manualConfig?: Record<string, unknown>;
  confirm: boolean;
}
