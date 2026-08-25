import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { ConfigTemplateRepository } from "@/infrastructure/repositories/ConfigTemplateRepository";
import type {
  CreateConfigTemplateDto,
  UpdateConfigTemplateDto,
} from "@/domain/entities/ConfigTemplate";

const configTemplateRepository = new ConfigTemplateRepository();

export function useConfigTemplates() {
  return useQuery({
    queryKey: ["config-templates"],
    queryFn: () => configTemplateRepository.getAll(),
  });
}

export function useConfigTemplate(id: string) {
  return useQuery({
    queryKey: ["config-templates", id],
    queryFn: () => configTemplateRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateConfigTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateConfigTemplateDto) =>
      configTemplateRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["config-templates"] });
    },
  });
}

export function useUpdateConfigTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateConfigTemplateDto }) =>
      configTemplateRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["config-templates"] });
      queryClient.invalidateQueries({
        queryKey: ["config-templates", variables.id],
      });
    },
  });
}

export function useDeleteConfigTemplate() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => configTemplateRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["config-templates"] });
    },
  });
}
