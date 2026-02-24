import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface CreateAuditWebhookInput {
  url: string;
  description?: string;
  event_filter?: string[];
}

interface UpdateAuditWebhookInput {
  webhookId: string;
  data: Partial<{
    url: string;
    description: string;
    event_filter: string[];
    is_active: boolean;
  }>;
}

export function useCreateAuditWebhook(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: CreateAuditWebhookInput) => {
      const { data } = await api.createAuditWebhook(teamId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.teamAuditWebhooks(teamId),
      });
    },
  });
}

export function useUpdateAuditWebhook(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateAuditWebhookInput) => {
      const { data } = await api.updateAuditWebhook(
        teamId,
        input.webhookId,
        input.data,
      );
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.teamAuditWebhooks(teamId),
      });
    },
  });
}

export function useDeleteAuditWebhook(teamId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (webhookId: string) => {
      await api.deleteAuditWebhook(teamId, webhookId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.teamAuditWebhooks(teamId),
      });
    },
  });
}
