import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useTeamAnalytics(
  teamId: string | undefined,
  days = 30,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.teamAnalytics(teamId ?? "", days),
    queryFn: async () => {
      const response = await api.getTeamAnalytics(teamId!, days);
      return response.data;
    },
    enabled: enabled && !!teamId,
  });
}

export function useUserAnalytics(days = 30, enabled = true) {
  return useQuery({
    queryKey: queryKeys.userAnalytics(days),
    queryFn: async () => {
      const response = await api.getUserAnalytics(days);
      return response.data;
    },
    enabled,
  });
}
