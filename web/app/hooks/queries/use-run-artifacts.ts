import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRunArtifacts(projectId: string, runId: string) {
  return useQuery({
    queryKey: queryKeys.runArtifacts(projectId, runId),
    queryFn: () => api.listRunArtifacts(projectId, runId),
    enabled: !!projectId && !!runId,
  });
}
