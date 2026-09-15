import { expect, test } from "@playwright/test";

test("creates an invoice in ledger and charges it in billing through one gateway", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByTestId("invoice-3")).toBeVisible();
  await expect(page.getByTestId("invoice-4")).toBeVisible();

  await page.getByRole("button", { name: "create invoice" }).click();
  const created = page.locator("[data-testid^=invoice-]").last();
  await expect(created).toContainText("USD 20.00");

  await page.getByRole("button", { name: "charge 3" }).click();
  await expect(page.getByTestId("charge-1")).toContainText("invoice 3 USD 1500.00 open");
  await expect(page.getByTestId("charge-error")).toHaveCount(0);
});
