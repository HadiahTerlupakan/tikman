import type {
  ConfigTemplate,
  CreateConfigTemplateDto,
  UpdateConfigTemplateDto,
} from "../entities/ConfigTemplate";

export interface IConfigTemplateRepository {
  getAll(): Promise<ConfigTemplate[]>;
  getById(id: string): Promise<ConfigTemplate>;
  create(data: CreateConfigTemplateDto): Promise<ConfigTemplate>;
  update(id: string, data: UpdateConfigTemplateDto): Promise<ConfigTemplate>;
  delete(id: string): Promise<void>;
}
