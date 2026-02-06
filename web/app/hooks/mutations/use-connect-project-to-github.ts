import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useConnectProjectToGitHub(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (data: {
      github_installation_id: string;
      github_repo_id: number;
      repo_url: string;
    }) => {
      return api.connectProjectToGitHub(projectId, data);
    },
    onSuccess: () => {
      // Invalidate project to refetch with new GitHub connection
      queryClient.invalidateQueries({ queryKey: queryKeys.project(projectId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.projects });
    },
  });
}
