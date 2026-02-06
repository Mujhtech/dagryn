import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useTeam(teamId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.team(teamId ?? ""),
    queryFn: async () => {
      const response = await api.getTeam(teamId!);
      return response.data;
    },
    enabled: !!teamId,
  });
}
