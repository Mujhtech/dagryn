import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export interface CreateProjectAPIKeyInput {
  name: string;
  expires_in?: string;
}

export function useCreateProjectAPIKey(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateProjectAPIKeyInput) => {
      const { data } = await api.createProjectAPIKey(projectId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectApiKeys(projectId),
      });
    },
  });
}
