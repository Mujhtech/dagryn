import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function usePlugin(pluginName: string) {
  return useQuery({
    queryKey: queryKeys.plugin(pluginName),
    queryFn: async () => {
      const { data } = await api.getPlugin(pluginName);
      return data;
    },
    enabled: !!pluginName,
  });
}
