import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRunDashboardSummary(
  projectId: string,
  days = 30,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.runDashboardSummary(projectId, days),
    queryFn: async () => {
      const { data } = await api.getRunDashboardSummary(projectId, days);
      return data;
    },
    enabled: enabled && !!projectId,
  });
}
