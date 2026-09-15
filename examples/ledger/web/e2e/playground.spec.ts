import { expect, test } from "@playwright/test";

test("the playground calls health through the same-origin proxy", async ({ page }) => {
  await page.goto("http://localhost:8080/playground/");
  await page.getByRole("button", { name: "health" }).click();
  await page.getByRole("button", { name: "Send" }).click();
  await expect(page.getByTestId("response")).toContainText('"ok": true');
});
