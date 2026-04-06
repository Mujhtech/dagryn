import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  api,
  type CreateSSOConnectionInput,
  type UpdateSSOConnectionInput,
} from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useCreateSSOConnection(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: CreateSSOConnectionInput) => {
      const { data } = await api.createSSOConnection(teamId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}

export function useUpdateSSOConnection(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (input: UpdateSSOConnectionInput) => {
      const { data } = await api.updateSSOConnection(teamId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}

export function useDeleteSSOConnection(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      await api.deleteSSOConnection(teamId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}

export function useTestSSOConnection(teamId: string) {
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.testSSOConnection(teamId);
      return data;
    },
  });
}

export function useToggleSSOEnforcement(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (enforce: boolean) => {
      const { data } = await api.toggleSSOEnforcement(teamId, enforce);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}

export function useGenerateSCIMToken(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.generateSCIMToken(teamId);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}

export function useRotateSCIMToken(teamId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async () => {
      const { data } = await api.rotateSCIMToken(teamId);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.ssoConnection(teamId),
      });
    },
  });
}
