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
  });
}
