import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function usePluginAnalytics(
  publisher: string,
  name: string,
  days = 30,
) {
  return useQuery({
    queryKey: queryKeys.registryPluginAnalytics(publisher, name, days),
    queryFn: async () => {
      const { data } = await api.getRegistryPluginAnalytics(
        publisher,
        name,
        days,
      );
      return data;
    },
    enabled: !!publisher && !!name,
  });
}
