import { expect, test } from "@playwright/test";

test("renders on the server, hydrates the Solid island, creates, and validates", async ({
  page,
}) => {
  const initial = await page.request.get("/");
  const html = await initial.text();
  expect(html).toContain('data-testid="invoice-3"');
  expect(html).toContain('data-testid="invoice-4"');

  await page.goto("/");
  await expect(page.getByTestId("island-invoice-3")).toBeVisible();
  await expect(page.getByTestId("island-invoice-4")).toBeVisible();

  await page.getByLabel("description").fill("");
  await page.getByLabel("quantity").fill("0");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.getByTestId("issues")).toContainText("lines.0.description: is required");
  await expect(page.getByTestId("issues")).toContainText("lines.0.quantity: must be at least 1");

  await page.getByLabel("description").fill("Widgets");
  await page.getByLabel("quantity").fill("3");
  await page.getByRole("button", { name: "create invoice" }).click();
  const created = page.locator("[data-testid^=island-invoice-]").last();
  await expect(created).toContainText("USD 30.00");
  await expect(page.getByTestId("issues")).toHaveCount(0);
});
