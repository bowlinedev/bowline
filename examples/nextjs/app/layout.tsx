import type { ReactNode } from "react";
import { Providers } from "./providers";

export const metadata = { title: "Ledger on Next.js" };

export default function RootLayout({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <body
        style={{ fontFamily: "system-ui", maxWidth: 720, margin: "2rem auto", padding: "0 1rem" }}
      >
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
