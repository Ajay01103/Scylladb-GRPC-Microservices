import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  transpilePackages: ["tldraw"],
  experimental: {
    useTypeScriptCli: true,
    serverActions: {
      bodySizeLimit: "20mb",
    },
  },
}

export default nextConfig
