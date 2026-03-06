import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useGitHubAppInstallations() {
  return useQuery({
    queryKey: queryKeys.githubAppInstallations,
    queryFn: async () => {
      const response = await api.listGitHubAppInstallations();
      return response.data;
    },
  });
}

export function useGitHubAppRepos(installationId: string | null) {
  return useQuery({
    queryKey: queryKeys.githubAppRepos(installationId || "none"),
    queryFn: async () => {
      if (!installationId) return [];
      const response = await api.listGitHubAppRepos(installationId);
      return response.data;
    },
    enabled: !!installationId,
  });
}
