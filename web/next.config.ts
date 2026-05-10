import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  /* config options here */
  rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: "http://hub-dev:8084/:path*",
      },
    ];
  },
};

export default nextConfig;
