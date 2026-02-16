import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useDashboardOverview(enabled = true) {
  return useQuery({
    queryKey: queryKeys.dashboardOverview,
    queryFn: async () => {
      const { data } = await api.getDashboardOverview();
      return data;
    },
    enabled,
  });
}
