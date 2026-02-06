import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useDeleteTeam() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (teamId: string) => {
      await api.deleteTeam(teamId);
    },
    onSuccess: (_, teamId) => {
      queryClient.invalidateQueries({ queryKey: queryKeys.teams });
      queryClient.removeQueries({ queryKey: queryKeys.team(teamId) });
    },
  });
}
