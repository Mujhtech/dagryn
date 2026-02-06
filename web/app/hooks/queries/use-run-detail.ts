import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRunDetail(projectId: string, runId: string) {
  return useQuery({
    queryKey: queryKeys.runDetail(projectId, runId),
    queryFn: () => api.getRunDetail(projectId, runId),
    enabled: !!projectId && !!runId,
  });
}
