import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useProjectAPIKeys(projectId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.projectApiKeys(projectId ?? ""),
    queryFn: async () => {
      const response = await api.listProjectAPIKeys(projectId!);
      return response.data;
    },
    enabled: !!projectId,
  });
}
