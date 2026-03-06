import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useAISuggestions(projectId: string, runId: string) {
  return useQuery({
    queryKey: queryKeys.aiSuggestions(projectId, runId),
    queryFn: async () => {
      const { data } = await api.getAISuggestions(projectId, runId);
      return data;
    },
    enabled: !!projectId && !!runId,
    retry: false,
  });
}
