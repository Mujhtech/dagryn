import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRemoveTeamMember(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (userId: string) => {
      await api.removeTeamMember(teamId, userId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.teamMembers(teamId),
      });
      queryClient.invalidateQueries({ queryKey: queryKeys.team(teamId) });
      queryClient.invalidateQueries({ queryKey: queryKeys.teams });
    },
  });
}
