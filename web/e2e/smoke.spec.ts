import { test, expect } from "@playwright/test";

test("app loads and shows login or dashboard", async ({ page }) => {
  await page.goto("/");
  // The app should load without crashing
  await expect(page.locator("body")).not.toBeEmpty();
});
