import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface UpdateUserInput {
  name: string;
}

export function useUpdateUser() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (input: UpdateUserInput) => api.updateCurrentUser(input),
    onSuccess: () => {
      // Invalidate current user query to refetch
      queryClient.invalidateQueries({ queryKey: queryKeys.currentUser });
    },
  });
}
