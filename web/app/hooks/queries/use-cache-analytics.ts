import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useCacheAnalytics(projectId: string, days = 30) {
  return useQuery({
    queryKey: queryKeys.cacheAnalytics(projectId, days),
    queryFn: async () => {
      const { data } = await api.getCacheAnalytics(projectId, days);
      return data;
    },
    enabled: !!projectId,
  });
}
