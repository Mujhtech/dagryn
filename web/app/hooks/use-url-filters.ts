import {
  parseAsArrayOf,
  parseAsInteger,
  parseAsString,
  useQueryState,
  useQueryStates,
} from "nuqs";
import type { RunStatus } from "~/lib/api";

/**
 * Parser for page number with default of 1
 */
export const parseAsPage = parseAsInteger.withDefault(1);

/**
 * Parser for status filters array
 */
export const parseAsStatusFilters = parseAsArrayOf(parseAsString).withDefault([
  "success",
  "failed",
  "cancelled",
  "running",
]);

/**
 * Parser for event filters array
 */
export const parseAsEventFilters = parseAsArrayOf(parseAsString).withDefault([
  "push",
  "pull_request",
  "schedule",
  "workflow_dispatch",
]);

/**
 * Parser for generic string arrays (users, workflows, branches)
 */
export const parseAsStringArray = parseAsArrayOf(parseAsString).withDefault([]);

/**
 * Hook for managing run filter URL state
 * Returns all filter states and their setters
 */
export function useRunFilters() {
  const [page, setPage] = useQueryState("page", parseAsPage);

  const [filtersState, setFiltersState] = useQueryStates({
    status: parseAsStatusFilters,
    event: parseAsEventFilters,
    users: parseAsStringArray,
    workflows: parseAsStringArray,
    branches: parseAsStringArray,
  });

  // Convert arrays to Sets for easier usage in components
  const statusFilters = new Set(filtersState.status as RunStatus[]);
  const eventFilters = new Set(filtersState.event);
  const selectedUsers = new Set(filtersState.users);
  const selectedWorkflows = new Set(filtersState.workflows);
  const selectedBranches = new Set(filtersState.branches);

  // Helper functions to toggle filters
  const toggleStatusFilter = (status: RunStatus) => {
    const newFilters = new Set(statusFilters);
    if (newFilters.has(status)) {
      newFilters.delete(status);
    } else {
      newFilters.add(status);
    }
    setFiltersState({ status: Array.from(newFilters) });
  };

  const toggleEventFilter = (event: string) => {
    const newFilters = new Set(eventFilters);
    if (newFilters.has(event)) {
      newFilters.delete(event);
    } else {
      newFilters.add(event);
    }
    setFiltersState({ event: Array.from(newFilters) });
  };

  const toggleUser = (userId: string) => {
    const newSet = new Set(selectedUsers);
    if (newSet.has(userId)) {
      newSet.delete(userId);
    } else {
      newSet.add(userId);
    }
    setFiltersState({ users: Array.from(newSet) });
  };

  const toggleWorkflow = (workflow: string) => {
    const newSet = new Set(selectedWorkflows);
    if (newSet.has(workflow)) {
      newSet.delete(workflow);
    } else {
      newSet.add(workflow);
    }
    setFiltersState({ workflows: Array.from(newSet) });
  };

  const toggleBranch = (branch: string) => {
    const newSet = new Set(selectedBranches);
    if (newSet.has(branch)) {
      newSet.delete(branch);
    } else {
      newSet.add(branch);
    }
    setFiltersState({ branches: Array.from(newSet) });
  };

  const setStatusFilters = (filters: Set<RunStatus>) => {
    setFiltersState({ status: Array.from(filters) });
  };

  const setEventFilters = (filters: Set<string>) => {
    setFiltersState({ event: Array.from(filters) });
  };

  const setSelectedUsers = (users: Set<string>) => {
    setFiltersState({ users: Array.from(users) });
  };

  const setSelectedWorkflows = (workflows: Set<string>) => {
    setFiltersState({ workflows: Array.from(workflows) });
  };

  const setSelectedBranches = (branches: Set<string>) => {
    setFiltersState({ branches: Array.from(branches) });
  };

  return {
    // Pagination
    page,
    setPage,

    // Filter Sets (for component usage)
    statusFilters,
    eventFilters,
    selectedUsers,
    selectedWorkflows,
    selectedBranches,

    // Set functions (for bulk updates)
    setStatusFilters,
    setEventFilters,
    setSelectedUsers,
    setSelectedWorkflows,
    setSelectedBranches,

    // Toggle functions (for individual filter changes)
    toggleStatusFilter,
    toggleEventFilter,
    toggleUser,
    toggleWorkflow,
    toggleBranch,
  };
}

/**
 * Hook for managing package manager tab state
 */
export function usePackageManagerTab() {
  const [pm, setPm] = useQueryState("pm", parseAsString.withDefault("npm"));
  return { pm, setPm };
}
