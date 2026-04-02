import { useQuery } from "@tanstack/react-query";
import { api, type Worker } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useCluster(clusterId: string) {
  return useQuery({
    queryKey: ["cluster", clusterId],
    queryFn: async () => {
      const { data } = await api.getCluster(clusterId);
      return data;
    },
    enabled: !!clusterId,
  });
}

export function useWorkers(clusterId?: string, status?: Worker["status"]) {
  return useQuery({
    queryKey: queryKeys.workers(clusterId, status),
    queryFn: async () => {
      const { data } = await api.listWorkers(clusterId, status);
      return data;
    },
  });
}
