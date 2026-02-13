import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export interface PublishPluginInput {
  publisher: string;
  name: string;
  version: string;
  manifest: Record<string, unknown>;
  release_notes?: string;
}

export function usePublishPlugin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: PublishPluginInput) => {
      const { data } = await api.publishPluginVersion(
        input.publisher,
        input.name,
        {
          version: input.version,
          manifest: input.manifest,
          release_notes: input.release_notes,
        },
      );
      return data;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.registryPlugin(variables.publisher, variables.name),
      });
      queryClient.invalidateQueries({
        queryKey: queryKeys.registryPluginVersions(
          variables.publisher,
          variables.name,
        ),
      });
    },
  });
}
