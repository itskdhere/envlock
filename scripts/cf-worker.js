/**
 * Cloudflare Worker script to dynamically serve install scripts for the EnvLock project based on the request context.
 *
 * Routes:
 * - /install.sh, /sh   -> returns install.sh
 * - /install.ps1, /ps1 -> returns install.ps1
 * - "/" and "/install":
 *     If curl / wget -> returns install.sh
 *     If PowerShell (irm / pwsh) -> returns install.ps1
 *     If Browsers -> redirects to GitHub repo
 *
 * Scripts are served from the latest release's assets, so unreleased commits on main never reach users.
 *
 * Caching:
 * - Cloudflare cache is used to cache the upstream scripts for 5 minutes (300 seconds)
 */

const REPO = "itskdhere/envlock";
const SCRIPTS_BASE = `https://github.com/${REPO}/releases/latest/download`;
const DOCS_URL = `https://github.com/${REPO}#readme`;

export default {
  async fetch(request) {
    const url = new URL(request.url);
    const userAgent = (request.headers.get("User-Agent") || "").toLowerCase();

    // Normalize path (e.g. "/install/" -> "/install", empty -> "/")
    const path = url.pathname.replace(/\/+$/, "") || "/";

    // 1. Direct Script Endpoints
    if (path === "/install.sh" || path === "/sh") {
      return fetchScript(
        `${SCRIPTS_BASE}/install.sh`,
        "text/x-shellscript",
      );
    }

    if (path === "/install.ps1" || path === "/ps1") {
      return fetchScript(`${SCRIPTS_BASE}/install.ps1`, "text/plain");
    }

    // 2. Dynamic Routes: Root ("/") and "/install" (curl / irm / browser detection)
    if (path === "/" || path === "/install") {
      // Terminal clients: curl or wget
      if (userAgent.includes("curl") || userAgent.includes("wget")) {
        return fetchScript(
          `${SCRIPTS_BASE}/install.sh`,
          "text/x-shellscript",
        );
      }

      // Windows PowerShell: Invoke-RestMethod (irm) or Invoke-WebRequest (iwr)
      if (
        userAgent.includes("powershell") ||
        userAgent.includes("pwsh") ||
        userAgent.includes("winhttp")
      ) {
        return fetchScript(`${SCRIPTS_BASE}/install.ps1`, "text/plain");
      }

      // Browsers -> redirect to GitHub repo
      return Response.redirect(DOCS_URL, 302);
    }

    return new Response("Not Found", { status: 404 });
  },
};

async function fetchScript(scriptUrl, contentType) {
  const upstream = await fetch(scriptUrl, {
    headers: {
      "User-Agent": "EnvLock-Cloudflare-Worker",
    },
    cf: {
      cacheTtl: 300,
      cacheEverything: true,
    },
  });

  if (!upstream.ok) {
    return new Response(
      `Error fetching install script: HTTP ${upstream.status}`,
      {
        status: upstream.status,
      },
    );
  }

  const content = await upstream.text();

  return new Response(content, {
    headers: {
      "Content-Type": `${contentType}; charset=utf-8`,
      "Cache-Control": "public, max-age=300",
      "Access-Control-Allow-Origin": "*",
    },
  });
}
