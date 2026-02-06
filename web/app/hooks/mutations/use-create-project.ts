import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export interface CreateProjectInput {
  name: string;
  slug: string;
  description?: string;
  visibility?: string;
  team_id?: string | null;
  repo_url?: string;
  github_installation_id?: string;
  github_repo_id?: number;
}

export function useCreateProject() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateProjectInput) => {
      const { data } = await api.createProject(input);
      return data;
    },
    onSuccess: () => {
      // Invalidate projects list to refetch
      queryClient.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}
