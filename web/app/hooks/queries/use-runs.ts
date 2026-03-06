import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRuns(projectId: string, page = 1, perPage = 20) {
  return useQuery({
    queryKey: queryKeys.runs(projectId, page),
    queryFn: async () => {
      const { data } = await api.listRuns(projectId, page, perPage);
      return data;
    },
    enabled: !!projectId,
    // Auto-poll every 3 seconds when there are active (running/pending) runs
    refetchInterval: (query) => {
      const runs = query.state.data?.data ?? [];
      const hasActive = runs.some(
        (r: { status: string }) =>
          r.status === "running" || r.status === "pending",
      );
      return hasActive ? 3000 : false;
    },
  });
}
