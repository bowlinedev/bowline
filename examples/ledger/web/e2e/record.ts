import { fileURLToPath } from "node:url";
import type { Interaction } from "@bowline/client";
import { fileSink } from "@bowline/client/node";
import type { BrowserContext, Page } from "@playwright/test";

export const recording = process.env.RECORD === "1";

const sink = fileSink(
  "ledger-web",
  fileURLToPath(new URL("../../contracts/consumers/ledger-web.json", import.meta.url)),
  { provider: "ledger" },
);

export async function arm(context: BrowserContext): Promise<void> {
  if (!recording) {
    return;
  }
  await context.addInitScript(() => {
    window.localStorage.setItem("bowline.record", "1");
  });
}

export async function collect(...pages: Page[]): Promise<void> {
  if (!recording) {
    return;
  }
  for (const page of pages) {
    const seen = await page.evaluate(
      () =>
        (window as unknown as { __bowlineInteractions?: Interaction[] }).__bowlineInteractions ??
        [],
    );
    for (const interaction of seen) {
      sink.write(interaction);
    }
  }
  await sink.flush();
}
