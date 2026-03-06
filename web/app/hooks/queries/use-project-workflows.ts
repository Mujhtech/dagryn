import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useProjectWorkflows(projectId: string) {
  return useQuery({
    queryKey: queryKeys.projectWorkflows(projectId),
    queryFn: async () => {
      const { data } = await api.listProjectWorkflows(projectId);
      return data;
    },
    enabled: !!projectId,
  });
}

export function useRunWorkflow(projectId: string, runId: string) {
  return useQuery({
    queryKey: queryKeys.runWorkflow(projectId, runId),
    queryFn: async () => {
      const { data } = await api.getRunWorkflow(projectId, runId);
      return data;
    },
    enabled: !!projectId && !!runId,
  });
}
