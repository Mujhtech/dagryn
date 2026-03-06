import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRevokeProjectAPIKey(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (keyId: string) => {
      await api.revokeProjectAPIKey(projectId, keyId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectApiKeys(projectId),
      });
    },
  });
}
