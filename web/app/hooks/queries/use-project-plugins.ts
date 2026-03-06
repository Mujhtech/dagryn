import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useProjectPlugins(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectPlugins(projectId),
    queryFn: async () => {
      const { data } = await api.listProjectPlugins(projectId);
      return data;
    },
    enabled: !!projectId,
  });
}
