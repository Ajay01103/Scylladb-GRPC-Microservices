import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  transpilePackages: ["tldraw"],
  experimental: {
    useTypeScriptCli: true
  }
}

export default nextConfig
