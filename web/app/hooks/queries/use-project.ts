import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useProject(projectId: string) {
  return useQuery({
    queryKey: queryKeys.project(projectId),
    queryFn: async () => {
      const { data } = await api.getProject(projectId);
      return data;
    },
    enabled: !!projectId,
  });
}
