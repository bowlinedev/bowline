import { expect, test } from "@playwright/test";

test("calls invoices.get through the proxy against the mock", async ({ page }) => {
  await page.goto("/_playground/");
  await page.getByRole("button", { name: "invoices.get" }).click();
  await page.getByLabel("id").fill("3");
  await page.getByRole("button", { name: "Send" }).click();
  await expect(page.getByTestId("response")).toContainText('"id": 3');
});
