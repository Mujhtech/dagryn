import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useUninstallPlugin(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (pluginName: string) => {
      const { data } = await api.uninstallProjectPlugin(projectId, pluginName);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectPlugins(projectId),
      });
    },
  });
}
