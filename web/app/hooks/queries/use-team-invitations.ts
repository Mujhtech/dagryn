import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useTeamInvitations(teamId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.teamInvitations(teamId ?? ""),
    queryFn: async () => {
      const response = await api.listTeamInvitations(teamId!);
      return response.data;
    },
    enabled: !!teamId,
  });
}
