import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";
import { queryKeys } from "../../lib/query-client";

export function useLicenseStatus() {
  return useQuery({
    queryKey: queryKeys.licenseStatus,
    queryFn: async () => {
      const { data } = await api.getLicenseStatus();
      return data;
    },
    staleTime: 60_000, // rarely changes
  });
}
