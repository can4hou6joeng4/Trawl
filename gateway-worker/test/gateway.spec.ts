import { SELF } from "cloudflare:test";
import { describe, expect, it } from "vitest";
import { licenseFor, HEADER_MACHINE_ID, HEADER_LICENSE } from "../src/index";

const GW = "https://gw.test";
const JOBS = `${GW}/api/v2/ocr/jobs`;

interface EchoResp {
  data?: { jobId?: string; state?: string };
  echo?: { auth: string | null; contentType: string | null; body: string };
  error?: string;
}

async function submit(machine: string, license?: string, body = "fake-multipart") {
  const headers: Record<string, string> = {
    [HEADER_MACHINE_ID]: machine,
    "Content-Type": "multipart/form-data; boundary=x",
  };
  if (license) headers[HEADER_LICENSE] = license;
  return SELF.fetch(JOBS, { method: "POST", headers, body });
}

describe("网关", () => {
  it("healthz 免鉴权", async () => {
    const resp = await SELF.fetch(`${GW}/healthz`);
    expect(resp.status).toBe(200);
    expect(await resp.text()).toBe("ok");
  });

  it("缺少/非法机器码 → 401", async () => {
    for (const machine of ["", "short", "GHIJKLMN", "ABCD1234EXTRA"]) {
      const headers: Record<string, string> = {};
      if (machine) headers[HEADER_MACHINE_ID] = machine;
      const resp = await SELF.fetch(JOBS, { method: "POST", headers, body: "x" });
      expect(resp.status, `machine=${machine}`).toBe(401);
    }
  });

  it("提交换上真实 Token 转发、请求体透传、登记归属", async () => {
    const resp = await submit("AAAA0001");
    expect(resp.status).toBe(200);
    const parsed = (await resp.json()) as EchoResp;
    expect(parsed.data?.jobId).toBe("job-echo");
    // 假上游回显：鉴权头已换为真实 Token，客户端头不透传
    expect(parsed.echo?.auth).toBe("bearer test-upstream-token");
    expect(parsed.echo?.contentType).toContain("boundary=x");
    expect(parsed.echo?.body).toBe("fake-multipart");

    // 归属校验：本机可轮询（响应来自假上游 → 含 echo 字段）
    const poll = await SELF.fetch(`${JOBS}/job-echo`, { headers: { [HEADER_MACHINE_ID]: "AAAA0001" } });
    expect(poll.status).toBe(200);
    const pollParsed = (await poll.json()) as EchoResp;
    expect(pollParsed.data?.state).toBe("running");
    expect(pollParsed.echo?.auth).toBe("bearer test-upstream-token");

    // 跨机器轮询 → 404（响应来自网关而非上游：无 echo 字段）
    const other = await SELF.fetch(`${JOBS}/job-echo`, { headers: { [HEADER_MACHINE_ID]: "BBBB0002" } });
    expect(other.status).toBe(404);
    expect(((await other.json()) as EchoResp).echo).toBeUndefined();
  });

  it("试用档配额（2 次）超额 → 403", async () => {
    expect((await submit("CCCC0003")).status).toBe(200);
    expect((await submit("CCCC0003")).status).toBe(200);
    const third = await submit("CCCC0003");
    expect(third.status).toBe(403);
    expect(((await third.json()) as EchoResp).error).toContain("已达上限");
  });

  it("有效授权码走授权档（4 次），无效授权码按试用档", async () => {
    const lic = await licenseFor("DDDD0004", "test-license-secret");
    for (let i = 0; i < 4; i++) {
      expect((await submit("DDDD0004", lic)).status, `第 ${i + 1} 次`).toBe(200);
    }
    expect((await submit("DDDD0004", lic)).status).toBe(403);

    // 无效授权码：不拒绝，按试用档
    expect((await submit("EEEE0005", "FAKE-FAKE-FAKE-FAKE")).status).toBe(200);
  });

  it("jobId 白名单：路径注入 → 404", async () => {
    for (const bad of ["..", "a%2Fb", "x%3Fy=1", "a".repeat(65)]) {
      const resp = await SELF.fetch(`${JOBS}/${bad}`, { headers: { [HEADER_MACHINE_ID]: "AAAA0001" } });
      expect(resp.status, `jobId=${bad}`).toBe(404);
    }
  });

  it("未知作业轮询 → 404", async () => {
    const resp = await SELF.fetch(`${JOBS}/never-seen`, { headers: { [HEADER_MACHINE_ID]: "AAAA0001" } });
    expect(resp.status).toBe(404);
  });

  it("上游非 200 原样透传", async () => {
    const resp = await submit("FFFF0006", undefined, "TRIGGER-503");
    expect(resp.status).toBe(503);
  });

  it("Content-Length 超限 → 413", async () => {
    const resp = await SELF.fetch(JOBS, {
      method: "POST",
      headers: { [HEADER_MACHINE_ID]: "AAAA0001", "Content-Length": String(61 * 1024 * 1024) },
      body: "x",
    });
    expect(resp.status).toBe(413);
  });

  it("未知路径 → 404，未知方法 → 405", async () => {
    expect((await SELF.fetch(`${GW}/other`)).status).toBe(404);
    expect((await SELF.fetch(JOBS, { method: "DELETE", headers: { [HEADER_MACHINE_ID]: "AAAA0001" } })).status).toBe(405);
  });
});
