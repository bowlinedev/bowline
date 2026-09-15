import type { NextConfig } from "next";

const backend = process.env.LEDGER_URL ?? "http://localhost:8080";

const config: NextConfig = {
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${backend}/api/:path*` }];
  },
};

export default config;
