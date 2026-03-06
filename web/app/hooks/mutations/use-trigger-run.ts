import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, TriggerRunRequest } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useTriggerRun(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (request?: TriggerRunRequest) => {
      const { data } = await api.triggerRun(projectId, request);
      return data;
    },
    onSuccess: () => {
      // Invalidate runs list to refetch
      queryClient.invalidateQueries({ queryKey: queryKeys.runs(projectId) });
    },
  });
}
