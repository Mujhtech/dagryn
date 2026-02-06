import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface AddTeamMemberInput {
  user_id: string;
  role: string;
}

export function useAddTeamMember(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: AddTeamMemberInput) => {
      const { data } = await api.addTeamMember(teamId, input);
      return data;
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
