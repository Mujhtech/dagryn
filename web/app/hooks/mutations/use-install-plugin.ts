import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useInstallPlugin(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (spec: string) => {
      const { data } = await api.installProjectPlugin(projectId, spec);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectPlugins(projectId),
      });
    },
  });
}
