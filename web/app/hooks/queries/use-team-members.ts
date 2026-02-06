import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useTeamMembers(teamId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.teamMembers(teamId ?? ""),
    queryFn: async () => {
      const response = await api.listTeamMembers(teamId!);
      return response.data;
    },
    enabled: !!teamId,
  });
}
