import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  /* config options here */
  rewrites() {
    return [
      {
        source: "/api/proxy/route/:path*",
        destination: "http://localhost:8190/:path*",
      },
    ];
  },
};

export default nextConfig;
