import { useQuery } from "@tanstack/react-query";
import { api } from "../../lib/api";

export function useLicenseStatus() {
  return useQuery({
    queryKey: ["licenseStatus"],
    queryFn: async () => {
      const { data } = await api.getLicenseStatus();
      return data;
    },
    staleTime: 60_000, // rarely changes
  });
}
