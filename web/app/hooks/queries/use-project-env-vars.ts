import { useQuery } from "@tanstack/react-query";
import { api, type ListProjectEnvVarsInput } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useProjectEnvVars(
  projectId: string | undefined,
  filters?: ListProjectEnvVarsInput,
) {
  return useQuery({
    queryKey: queryKeys.projectEnvVars(projectId ?? "", filters),
    queryFn: async () => {
      const response = await api.listProjectEnvVars(projectId!, filters);
      return response.data;
    },
    enabled: !!projectId,
  });
}
