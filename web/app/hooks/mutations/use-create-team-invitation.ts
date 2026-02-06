import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface CreateTeamInvitationInput {
  email: string;
  role: string;
}

export function useCreateTeamInvitation(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateTeamInvitationInput) => {
      const { data } = await api.createTeamInvitation(teamId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.teamInvitations(teamId),
      });
    },
  });
}
