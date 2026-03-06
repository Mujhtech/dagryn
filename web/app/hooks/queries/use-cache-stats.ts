import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useCacheStats(projectId: string) {
  return useQuery({
    queryKey: queryKeys.cacheStats(projectId),
    queryFn: async () => {
      const { data } = await api.getCacheStats(projectId);
      return data;
    },
    enabled: !!projectId,
  });
}
