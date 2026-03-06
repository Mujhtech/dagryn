import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useRegistryPlugins(params?: {
  q?: string;
  type?: string;
  sort?: string;
  page?: number;
  per_page?: number;
}) {
  return useQuery({
    queryKey: queryKeys.registryPlugins(
      params?.q,
      params?.type,
      params?.sort,
      params?.page,
    ),
    queryFn: async () => {
      const { data } = await api.searchRegistryPlugins(params);
      return data;
    },
  });
}

export function useFeaturedPlugins() {
  return useQuery({
    queryKey: queryKeys.featuredPlugins,
    queryFn: async () => {
      const { data } = await api.listFeaturedPlugins();
      return data;
    },
  });
}

export function useTrendingPlugins() {
  return useQuery({
    queryKey: queryKeys.trendingPlugins,
    queryFn: async () => {
      const { data } = await api.listTrendingPlugins();
      return data;
    },
  });
}
