import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useActivateLicense() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (licenseKey: string) => api.activateLicense(licenseKey),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: queryKeys.licenseStatus });
      queryClient.invalidateQueries({ queryKey: queryKeys.capabilities });
    },
  });
}
