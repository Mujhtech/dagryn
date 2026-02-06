import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface CancelRunInput {
  projectId: string;
  runId: string;
}

export function useCancelRun() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ projectId, runId }: CancelRunInput) =>
      api.cancelRun(projectId, runId),
    onSuccess: (_, { projectId, runId }) => {
      // Invalidate run detail to refetch
      queryClient.invalidateQueries({
        queryKey: queryKeys.runDetail(projectId, runId),
      });
      // Also invalidate runs list
      queryClient.invalidateQueries({
        queryKey: queryKeys.runs(projectId),
      });
    },
  });
}
