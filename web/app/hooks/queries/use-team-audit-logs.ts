import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export interface AuditLogParams {
  actor_id?: string;
  actor_email?: string;
  action?: string;
  category?: string;
  since?: string;
  until?: string;
  cursor?: string;
  limit?: number;
}

export function useTeamAuditLogs(
  teamId: string | undefined,
  params?: AuditLogParams,
) {
  return useQuery({
    queryKey: queryKeys.teamAuditLogs(teamId ?? "", params?.cursor),
    queryFn: async () => {
      const response = await api.listTeamAuditLogs(teamId!, params);
      return response.data;
    },
    enabled: !!teamId,
  });
}

export function useAuditRetentionPolicy(teamId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.teamAuditRetention(teamId ?? ""),
    queryFn: async () => {
      const response = await api.getAuditRetentionPolicy(teamId!);
      return response.data;
    },
    enabled: !!teamId,
  });
}

export function useProjectAuditLogs(
  projectId: string | undefined,
  params?: AuditLogParams,
) {
  return useQuery({
    queryKey: queryKeys.projectAuditLogs(projectId ?? "", params?.cursor),
    queryFn: async () => {
      const response = await api.listProjectAuditLogs(projectId!, params);
      return response.data;
    },
    enabled: !!projectId,
  });
}

export function useAuditWebhooks(teamId: string | undefined) {
  return useQuery({
    queryKey: queryKeys.teamAuditWebhooks(teamId ?? ""),
    queryFn: async () => {
      const response = await api.listAuditWebhooks(teamId!);
      return response.data;
    },
    enabled: !!teamId,
  });
}
