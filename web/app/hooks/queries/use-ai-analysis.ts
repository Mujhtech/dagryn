import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRunAIAnalysis(projectId: string, runId: string) {
  return useQuery({
    queryKey: queryKeys.runAIAnalysis(projectId, runId),
    queryFn: async () => {
      const { data } = await api.getRunAIAnalysis(projectId, runId);
      return data;
    },
    enabled: !!projectId && !!runId,
    retry: (failureCount, error) => {
      // Don't retry on 404 (no analysis exists yet)
      if (error && "status" in error && (error as { status: number }).status === 404) {
        return false;
      }
      return failureCount < 2;
    },
  });
}

export function useProjectAIAnalyses(
  projectId: string,
  limit = 20,
  offset = 0,
) {
  return useQuery({
    queryKey: queryKeys.projectAIAnalyses(projectId, offset),
    queryFn: async () => {
      const { data } = await api.listProjectAIAnalyses(projectId, {
        limit,
        offset,
      });
      return data;
    },
    enabled: !!projectId,
  });
}
