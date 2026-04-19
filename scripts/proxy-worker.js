/**
 * Cloudflare Worker Proxy for GitHub Private Releases
 * 
 * USE CASE: You have a private repository but want anyone (without a GitHub account/token)
 * to be able to download `themis` releases securely via a standard bash script.
 * 
 * HOW IT WORKS:
 * 1. Deploy this via Cloudflare Workers (`wrangler publish`).
 * 2. Set the `GITHUB_TOKEN` environment variable in your Cloudflare dashboard.
 * 3. Point your `install.sh` to download from `https://your-worker.workers.dev/release/linux`
 */

export default {
    async fetch(request, env) {
      const url = new URL(request.url);
      
      // We only care about /release/linux, /release/windows routes
      if (!url.pathname.startsWith("/release/")) {
        return new Response("Invalid endpoint. Use /release/linux or /release/windows", { status: 400 });
      }
  
      const repo = "syn3rgy2026/UntrainedModels_Syn3rgy_SatyamUttamPandey";
      const token = env.GITHUB_TOKEN; 
  
      if (!token) {
        return new Response("Proxy is missing GITHUB_TOKEN environment variable", { status: 500 });
      }
  
      const platform = url.pathname.replace("/release/", "");
  
      // Step 1: Fetch the latest release metadata
      const releaseRes = await fetch(`https://api.github.com/repos/${repo}/releases/latest`, {
        headers: {
          "Authorization": `token ${token}`,
          "User-Agent": "Cloudflare-Worker"
        }
      });
  
      if (!releaseRes.ok) {
        return new Response("Failed to fetch latest release from GitHub", { status: releaseRes.status });
      }
  
      const releaseJson = await releaseRes.json();
  
      // Step 2: Find the matching asset for the requested platform
      let targetAsset = null;
      for (const asset of releaseJson.assets) {
        const name = asset.name.toLowerCase();
        if (platform === "linux" && name.includes("linux") && (name.includes("amd64") || name.includes("x86_64"))) {
          targetAsset = asset;
          break;
        }
        if (platform === "windows" && (name.includes("windows") || name.includes("win64"))) {
          targetAsset = asset;
          break;
        }
      }
  
      if (!targetAsset) {
        return new Response(`Could not find a matching asset for platform '${platform}' in latest release.`, { status: 404 });
      }
  
      // Step 3: Stream the actual asset binary using the token
      return fetch(targetAsset.url, {
        headers: {
          "Authorization": `token ${token}`,
          "Accept": "application/octet-stream",
          "User-Agent": "Cloudflare-Worker"
        },
        redirect: "follow"
      });
    }
  };
