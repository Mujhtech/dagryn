import { useQuery } from "@tanstack/react-query";
import { api, type Worker } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useClusters(teamId?: string) {
  return useQuery({
    queryKey: queryKeys.clusters(teamId),
    queryFn: async () => {
      const { data } = await api.listClusters(teamId);
      return data;
    },
  });
}

export function useWorkers(
  clusterId?: string,
  status?: Worker["status"],
  teamId?: string,
  enabled = true,
) {
  return useQuery({
    queryKey: queryKeys.workers(clusterId, status, teamId),
    queryFn: async () => {
      const { data } = await api.listWorkers(clusterId, status, teamId);
      return data;
    },
    enabled,
  });
}

export function useWorkersForAccessibleScopes(teamIds: string[], enabled = true) {
  return useQuery({
    queryKey: ["workers-accessible", [...teamIds].sort()],
    queryFn: async () => {
      const buckets = await Promise.all([
        api.listWorkers(),
        ...teamIds.map((teamId) => api.listWorkers(undefined, undefined, teamId)),
      ]);
      const deduped = new Map<string, Worker>();
      buckets.forEach(({ data }) => {
        data.forEach((worker) => {
          deduped.set(worker.id, worker);
        });
      });
      return Array.from(deduped.values());
    },
    enabled,
  });
}

export function useRunAssignments(runId?: string) {
  return useQuery({
    queryKey: queryKeys.runAssignments(runId ?? ""),
    queryFn: async () => {
      const { data } = await api.listRunAssignments(runId!);
      return data;
    },
    enabled: !!runId,
  });
}

export function useWorkerTokens(enabled = true) {
  return useQuery({
    queryKey: queryKeys.workerTokens,
    queryFn: async () => {
      const { data } = await api.listWorkerTokens();
      return data;
    },
    enabled,
  });
}
