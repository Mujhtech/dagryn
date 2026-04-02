import { test as base, type Page } from "@playwright/test";

// Mock user data
export const mockUser = {
  id: "user-001",
  email: "test@example.com",
  name: "Test User",
  avatar_url: "",
  created_at: "2025-01-01T00:00:00Z",
};

export const mockAuthProviders = [
  {
    id: "github",
    name: "GitHub",
    display_name: "GitHub",
    auth_url: "https://github.com/login/oauth/authorize?client_id=test",
    enabled: true,
  },
];

export const mockProjects = [
  {
    id: "proj-001",
    name: "My Project",
    slug: "my-project",
    description: "A test project",
    visibility: "private",
    member_count: 2,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-06-01T00:00:00Z",
  },
  {
    id: "proj-002",
    name: "Public App",
    slug: "public-app",
    description: "A public project",
    visibility: "public",
    member_count: 5,
    created_at: "2025-02-01T00:00:00Z",
    updated_at: "2025-05-15T00:00:00Z",
  },
];

export const mockTeams = [
  {
    id: "team-001",
    name: "Engineering",
    slug: "engineering",
    description: "Engineering team",
    member_count: 4,
    created_at: "2025-01-01T00:00:00Z",
    updated_at: "2025-06-01T00:00:00Z",
  },
];

export const mockDashboardOverview = {
  projects: mockProjects,
  recent_runs: [
    {
      id: "run-001",
      project_id: "proj-001",
      project_name: "My Project",
      status: "success",
      started_at: "2025-06-01T10:00:00Z",
      finished_at: "2025-06-01T10:05:00Z",
      duration_ms: 300000,
      task_count: 5,
    },
  ],
};

export const mockLicenseStatus = {
  mode: "cloud",
  valid: true,
  plan: "pro",
};

/**
 * Sets up API route mocking for authenticated sessions.
 * Call this before navigating to any page that requires auth.
 */
export async function setupAuthenticatedSession(page: Page) {
  // Set auth tokens in localStorage before navigation
  await page.addInitScript(() => {
    localStorage.setItem("access_token", "mock-access-token");
    localStorage.setItem("refresh_token", "mock-refresh-token");
  });

  // Mock API responses
  await page.route("**/api/v1/users/me", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: {
          id: "user-001",
          email: "test@example.com",
          name: "Test User",
          avatar_url: "",
          created_at: "2025-01-01T00:00:00Z",
        },
      }),
    });
  });

  await page.route("**/api/v1/license/status", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ mode: "cloud", valid: true, plan: "pro" }),
    });
  });
}

/**
 * Sets up common API mocks for dashboard data.
 */
export async function setupDashboardMocks(page: Page) {
  await page.route("**/api/v1/dashboard/overview", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(mockDashboardOverview),
    });
  });

  await page.route("**/api/v1/projects?*", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: mockProjects,
        meta: { total: mockProjects.length, page: 1, per_page: 20 },
      }),
    });
  });

  await page.route("**/api/v1/projects", (route) => {
    if (route.request().method() === "GET") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: mockProjects,
          meta: { total: mockProjects.length, page: 1, per_page: 20 },
        }),
      });
    } else {
      route.continue();
    }
  });

  await page.route("**/api/v1/teams?*", (route) => {
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        data: mockTeams,
        meta: { total: mockTeams.length, page: 1, per_page: 20 },
      }),
    });
  });

  await page.route("**/api/v1/teams", (route) => {
    if (route.request().method() === "GET") {
      route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          data: mockTeams,
          meta: { total: mockTeams.length, page: 1, per_page: 20 },
        }),
      });
    } else {
      route.continue();
    }
  });
}

// Custom test fixture with authenticated page
export const test = base.extend<{ authenticatedPage: Page }>({
  authenticatedPage: async ({ page }, use) => {
    await setupAuthenticatedSession(page);
    await setupDashboardMocks(page);
    await use(page);
  },
});

export { expect } from "@playwright/test";
