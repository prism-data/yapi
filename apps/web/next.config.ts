import type { NextConfig } from "next";
import { getYapiPath } from "./app/lib/yapi-path";

// Verify yapi binary exists at build time
const yapiPath = getYapiPath();
console.log(`[next.config] yapi binary found at: ${yapiPath}`);

const nextConfig: NextConfig = {
  turbopack: {

  },
  // Include the yapi binary in serverless function deployment
  // Path relative to apps/web -> monorepo root bin/yapi
  outputFileTracingIncludes: {
    "/api/**/*": ["../../bin/yapi"],
  },
};

export default nextConfig;
