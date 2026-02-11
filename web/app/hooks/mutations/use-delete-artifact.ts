import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

interface DeleteArtifactInput {
  projectId: string;
  runId: string;
  artifactId: string;
}

export function useDeleteArtifact() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ projectId, runId, artifactId }: DeleteArtifactInput) =>
      api.deleteArtifact(projectId, runId, artifactId),
    onSuccess: (_, { projectId, runId }) => {
      queryClient.invalidateQueries({
        queryKey: queryKeys.runArtifacts(projectId, runId),
      });
    },
  });
}
