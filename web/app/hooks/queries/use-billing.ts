import { useQuery } from "@tanstack/react-query";
import { api } from "~/lib/api";
import { queryKeys } from "~/lib/query-client";

export function useBillingPlans() {
  return useQuery({
    queryKey: queryKeys.billingPlans,
    queryFn: async () => {
      const { data } = await api.listBillingPlans();
      return data;
    },
  });
}

export function useBillingPlan(slug: string) {
  return useQuery({
    queryKey: queryKeys.billingPlan(slug),
    queryFn: async () => {
      const { data } = await api.getBillingPlan(slug);
      return data;
    },
    enabled: !!slug,
  });
}

export function useBillingOverview() {
  return useQuery({
    queryKey: queryKeys.billingOverview,
    queryFn: async () => {
      const { data } = await api.getBillingOverview();
      return data;
    },
  });
}

export function useBillingInvoices(limit = 20, offset = 0) {
  return useQuery({
    queryKey: queryKeys.billingInvoices(limit, offset),
    queryFn: async () => {
      const { data } = await api.listInvoices(limit, offset);
      return data;
    },
  });
}
