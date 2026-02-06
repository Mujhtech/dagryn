import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface UpdateProjectInput {
  name?: string;
  description?: string;
  visibility?: string;
}

export function useUpdateProject(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateProjectInput) => {
      const { data } = await api.updateProject(projectId, input);
      return data;
    },
    onSuccess: () => {
      // Invalidate both the project detail and projects list
      queryClient.invalidateQueries({ queryKey: queryKeys.project(projectId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}
