import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface RetryAIAnalysisInput {
  projectId: string;
  runId: string;
}

export function useRetryAIAnalysis() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ projectId, runId }: RetryAIAnalysisInput) =>
      api.retryAIAnalysis(projectId, runId),
    onSuccess: (_, { projectId, runId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.runAIAnalysis(projectId, runId),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectAIAnalyses(projectId),
      });
    },
  });
}

export function usePostAISuggestions() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ projectId, runId }: RetryAIAnalysisInput) =>
      api.postAISuggestions(projectId, runId),
    onSuccess: (_, { projectId, runId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.aiSuggestions(projectId, runId),
      });
    },
  });
}
