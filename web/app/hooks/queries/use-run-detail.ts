import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRunDetail(
  projectId: string,
  runId: string,
  options?: { refetchInterval?: number | false },
) {
  return useQuery({
    queryKey: queryKeys.runDetail(projectId, runId),
    queryFn: () => api.getRunDetail(projectId, runId),
    enabled: !!projectId && !!runId,
    refetchInterval: options?.refetchInterval,
  });
}
