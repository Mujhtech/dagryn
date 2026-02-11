import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useGitHubWorkflowTranslation(
  repoFullName: string | null,
  installationId?: string | null,
) {
  return useQuery({
    queryKey: queryKeys.githubWorkflowTranslation(
      repoFullName || "none",
      installationId || undefined,
    ),
    queryFn: async () => {
      if (!repoFullName) {
        return null;
      }
      const response = await api.translateGitHubWorkflows({
        repo_full_name: repoFullName,
        github_installation_id: installationId || undefined,
      });
      return response.data;
    },
    enabled: !!repoFullName,
  });
}
