export interface ConfigTemplate {
  id: string;
  name: string;
  description: string;
  vendor: "ZTE" | "HSGQ";
  configFields: Record<string, unknown>;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface CreateConfigTemplateDto {
  name: string;
  description?: string;
  vendor: "ZTE" | "HSGQ";
  configFields: Record<string, unknown>;
  isDefault?: boolean;
}

export interface UpdateConfigTemplateDto {
  name?: string;
  description?: string;
  vendor?: "ZTE" | "HSGQ";
  configFields?: Record<string, unknown>;
  isDefault?: boolean;
}
