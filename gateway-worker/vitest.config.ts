import { defineConfig } from "vitest/config";
import { cloudflareTest } from "@cloudflare/vitest-pool-workers";

// 可编程假上游：拦截 Worker 的全部出站 fetch。
// 约定式行为（无逐测试脚本）：响应回显收到的鉴权/Content-Type/请求体，
// 供测试断言转发契约；请求体含 TRIGGER-503 时返回 503 模拟上游故障。
async function fakeUpstream(request: Request): Promise<Response> {
  const url = new URL(request.url);
  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    });

  if (url.hostname !== "upstream.test") {
    return json(599, { msg: `unexpected upstream host: ${url.hostname}` });
  }
  if (request.method === "POST" && url.pathname === "/api/v2/ocr/jobs") {
    const body = await request.text();
    if (body.includes("TRIGGER-503")) return json(503, { msg: "upstream down" });
    return json(200, {
      data: { jobId: "job-echo" },
      echo: {
        auth: request.headers.get("Authorization"),
        contentType: request.headers.get("Content-Type"),
        body,
      },
    });
  }
  const poll = url.pathname.match(/^\/api\/v2\/ocr\/jobs\/([^/]+)$/);
  if (request.method === "GET" && poll) {
    return json(200, {
      data: { state: "running", jobId: poll[1] },
      echo: { auth: request.headers.get("Authorization") },
    });
  }
  return json(404, { msg: "not found" });
}

export default defineConfig({
  plugins: [
    cloudflareTest({
      wrangler: { configPath: "./wrangler.toml" },
      miniflare: {
        outboundService: fakeUpstream,
        // 测试专用小配额，便于断言限额行为
        bindings: {
          OCR_GW_TOKEN: "test-upstream-token",
          OCR_GW_LICENSE_SECRET: "test-license-secret",
          TRIAL_DAILY: "2",
          LICENSED_DAILY: "4",
          PER_MINUTE: "50",
          UPSTREAM_JOB_URL: "https://upstream.test/api/v2/ocr/jobs",
        },
      },
    }),
  ],
});
