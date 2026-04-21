import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, type SetProjectEnvVarInput } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useSetProjectEnvVar(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: SetProjectEnvVarInput) => {
      const { data } = await api.setProjectEnvVar(projectId, input);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectEnvVars(projectId),
      });
    },
  });
}

export function useSeedProjectEnvVars(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (items: SetProjectEnvVarInput[]) => {
      const { data } = await api.seedProjectEnvVars(projectId, items);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectEnvVars(projectId),
      });
    },
  });
}

export function useRotateProjectEnvVar(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: {
      envVarId: string;
      value?: string;
    }) => {
      const { envVarId, ...payload } = input;
      const { data } = await api.rotateProjectEnvVar(projectId, envVarId, payload);
      return data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectEnvVars(projectId),
      });
    },
  });
}

export function useDeleteProjectEnvVar(projectId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (envVarId: string) => {
      await api.deleteProjectEnvVar(projectId, envVarId);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.projectEnvVars(projectId),
      });
    },
  });
}

export function useUpdateProjectEnvVar(projectId: string) {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (input: {
			envVarId: string;
			description?: string;
			required?: boolean;
			enabled?: boolean;
		}) => {
			const { envVarId, ...payload } = input;
			const { data } = await api.updateProjectEnvVar(projectId, envVarId, payload);
			return data;
		},
		onSuccess: () => {
			queryClient.invalidateQueries({
				queryKey: queryKeys.projectEnvVars(projectId),
			});
		},
	});
}
