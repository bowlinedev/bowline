import { expect, test } from "@playwright/test";

test("the app still lists and creates with CSRF enabled", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByTestId("invoice-3")).toBeVisible();

  await page.getByLabel("description").fill("Widgets");
  await page.getByLabel("quantity").fill("2");
  await page.getByRole("button", { name: "create invoice" }).click();
  await expect(page.locator("[data-testid^=invoice-]").last()).toContainText("USD 20.00");
});

test("a mutation forged from another origin is rejected", async ({ request }) => {
  const response = await request.post("http://localhost:8080/api/invoices.void", {
    headers: {
      "Content-Type": "application/json",
      Origin: "https://evil.example",
      "Sec-Fetch-Site": "cross-site",
    },
    data: { id: 3 },
    failOnStatusCode: false,
  });
  expect(response.status()).toBe(403);
  expect(await response.json()).toMatchObject({
    error: { code: "PERMISSION_DENIED", message: "cross-origin request rejected" },
  });
});

test("error responses carry the security headers", async ({ request }) => {
  const response = await request.get("http://localhost:8080/api/invoices.nope", {
    failOnStatusCode: false,
  });
  expect(response.headers()["x-content-type-options"]).toBe("nosniff");
  expect(response.headers()["cache-control"]).toBe("no-store");
});
