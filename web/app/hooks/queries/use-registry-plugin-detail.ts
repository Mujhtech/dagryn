import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRegistryPluginDetail(publisher: string, name: string) {
  return useQuery({
    queryKey: queryKeys.registryPlugin(publisher, name),
    queryFn: async () => {
      const { data } = await api.getRegistryPlugin(publisher, name);
      return data;
    },
    enabled: !!publisher && !!name,
  });
}

export function useRegistryPluginVersions(publisher: string, name: string) {
  return useQuery({
    queryKey: queryKeys.registryPluginVersions(publisher, name),
    queryFn: async () => {
      const { data } = await api.getRegistryPluginVersions(publisher, name);
      return data;
    },
    enabled: !!publisher && !!name,
  });
}
