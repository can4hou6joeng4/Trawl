/**
 * Legal Extractor OCR 中继网关 —— Cloudflare Workers 版。
 *
 * 与 Go 版（internal/gateway）行为逐项对齐：
 *   - 校验 X-LE-Machine-ID（8 位十六进制）与可选 X-LE-License
 *     （HMAC-SHA256(machineID, LICENSE_SECRET) 前 16 位，4-4-4-4 格式）
 *   - 试用/授权双档每日提交配额 + 每分钟请求限频 + 作业归属校验
 *     （计量状态存于单实例 Durable Object，SQLite 存储，闹钟定期清理）
 *   - 换上真实 Token（Worker Secret，永不下发）转发 AI Studio v2 作业接口
 *   - 结果 JSONL 为免鉴权签名 URL，客户端直连下载，不过网关
 *
 * 部署见 docs/ocr-gateway-deploy.md 的 Cloudflare Workers 一节。
 */

export interface Env {
  METER: DurableObjectNamespace;
  /** Worker Secret：真实 AI Studio Token（wrangler secret put OCR_GW_TOKEN） */
  OCR_GW_TOKEN: string;
  /** Worker Secret：授权校验密钥，与应用发布构建注入的 LicenseSecret 一致；缺省全部按试用档 */
  OCR_GW_LICENSE_SECRET?: string;
  /** 以下为 wrangler.toml [vars]，均有默认值 */
  UPSTREAM_JOB_URL?: string;
  TRIAL_DAILY?: string;
  LICENSED_DAILY?: string;
  PER_MINUTE?: string;
}

const DEFAULT_UPSTREAM = "https://paddleocr.aistudio-app.com/api/v2/ocr/jobs";
const JOBS_PREFIX = "/api/v2/ocr/jobs";
const MACHINE_RE = /^[0-9A-F]{8}$/;
const JOB_ID_RE = /^[A-Za-z0-9_-]{1,64}$/; // 白名单：杜绝路径/查询注入
const MAX_UPLOAD_BYTES = 60 * 1024 * 1024;

export const HEADER_MACHINE_ID = "X-LE-Machine-ID";
export const HEADER_LICENSE = "X-LE-License";

function jsonError(status: number, msg: string): Response {
  return new Response(JSON.stringify({ error: msg }), {
    status,
    headers: { "Content-Type": "application/json; charset=utf-8" },
  });
}

/** 与应用端 config.GenerateLicense 同构 */
export async function licenseFor(machineID: string, secret: string): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    "raw", enc.encode(secret), { name: "HMAC", hash: "SHA-256" }, false, ["sign"],
  );
  const sig = await crypto.subtle.sign("HMAC", key, enc.encode(machineID));
  const hex = [...new Uint8Array(sig)].map((b) => b.toString(16).padStart(2, "0")).join("");
  const code = hex.slice(0, 16).toUpperCase();
  return `${code.slice(0, 4)}-${code.slice(4, 8)}-${code.slice(8, 12)}-${code.slice(12, 16)}`;
}

/** 常数时间比较，防时序侧信道 */
function constantTimeEqual(a: string, b: string): boolean {
  const enc = new TextEncoder();
  const ab = enc.encode(a);
  const bb = enc.encode(b);
  if (ab.length !== bb.length) return false;
  let diff = 0;
  for (let i = 0; i < ab.length; i++) diff |= ab[i] ^ bb[i];
  return diff === 0;
}

async function verifyLicense(machineID: string, license: string, secret?: string): Promise<boolean> {
  if (!secret) return false;
  const expected = await licenseFor(machineID, secret);
  return constantTimeEqual(license.trim().toUpperCase(), expected);
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    const url = new URL(request.url);

    if (url.pathname === "/healthz") {
      return new Response("ok");
    }
    if (!url.pathname.startsWith(JOBS_PREFIX)) {
      return jsonError(404, "未知路径");
    }

    // 认证：机器码必填，授权码可选（有效则授权档）
    const machineID = (request.headers.get(HEADER_MACHINE_ID) ?? "").trim().toUpperCase();
    if (!MACHINE_RE.test(machineID)) {
      return jsonError(401, "缺少或非法的机器标识");
    }
    const license = request.headers.get(HEADER_LICENSE) ?? "";
    const licensed = license !== "" && (await verifyLicense(machineID, license, env.OCR_GW_LICENSE_SECRET));
    const tier = licensed ? "licensed" : "trial";

    const meter = env.METER.get(env.METER.idFromName("global"));
    const upstream = env.UPSTREAM_JOB_URL || DEFAULT_UPSTREAM;
    const rest = url.pathname.slice(JOBS_PREFIX.length);

    // POST /api/v2/ocr/jobs —— 提交
    if (request.method === "POST" && (rest === "" || rest === "/")) {
      const contentLength = Number(request.headers.get("Content-Length") ?? "0");
      if (contentLength > MAX_UPLOAD_BYTES) {
        return jsonError(413, "文件过大，超出网关限制");
      }

      const gate = await meterCall(meter, { op: "submit", machine: machineID, tier });
      if (!gate.ok) return jsonError(gate.status, gate.error);

      let resp: Response;
      try {
        resp = await fetch(upstream, {
          method: "POST",
          headers: forwardHeaders(request, env),
          body: request.body,
        });
      } catch {
        return jsonError(502, "云端服务暂不可用，请稍后再试");
      }

      // 提交响应是小 JSON：读全以登记作业归属，再原样回写
      const bodyText = await resp.text();
      if (resp.ok) {
        const jobId = extractJobID(bodyText);
        if (jobId) await meterCall(meter, { op: "record", machine: machineID, jobId });
      }
      return passthrough(resp, bodyText);
    }

    // GET /api/v2/ocr/jobs/{id} —— 轮询
    if (request.method === "GET" && rest.startsWith("/")) {
      const jobId = rest.slice(1);
      if (!JOB_ID_RE.test(jobId)) {
        return jsonError(404, "作业不存在");
      }
      const gate = await meterCall(meter, { op: "poll", machine: machineID, jobId });
      if (!gate.ok) return jsonError(gate.status, gate.error);

      let resp: Response;
      try {
        resp = await fetch(`${upstream}/${jobId}`, {
          method: "GET",
          headers: { Authorization: `bearer ${env.OCR_GW_TOKEN}` },
        });
      } catch {
        return jsonError(502, "云端服务暂不可用，请稍后再试");
      }
      return passthrough(resp, await resp.text());
    }

    return jsonError(405, "不支持的方法");
  },
} satisfies ExportedHandler<Env>;

/** 只带必要头转发：客户端头一律不透传（Content-Type 携带 multipart 边界必须保留） */
function forwardHeaders(request: Request, env: Env): HeadersInit {
  const h: Record<string, string> = { Authorization: `bearer ${env.OCR_GW_TOKEN}` };
  const ct = request.headers.get("Content-Type");
  if (ct) h["Content-Type"] = ct;
  return h;
}

function passthrough(resp: Response, bodyText: string): Response {
  return new Response(bodyText, {
    status: resp.status,
    headers: { "Content-Type": resp.headers.get("Content-Type") ?? "application/json" },
  });
}

function extractJobID(body: string): string {
  try {
    const parsed = JSON.parse(body) as { data?: { jobId?: string } };
    return parsed.data?.jobId ?? "";
  } catch {
    return "";
  }
}

// ─── Meter Durable Object：单实例全局计量 ───

interface MeterRequest {
  op: "submit" | "poll" | "record";
  machine: string;
  tier?: string;
  jobId?: string;
}

interface MeterReply {
  ok: boolean;
  status: number;
  error: string;
}

async function meterCall(stub: DurableObjectStub, req: MeterRequest): Promise<MeterReply> {
  const resp = await stub.fetch("https://meter.internal/", {
    method: "POST",
    body: JSON.stringify(req),
  });
  return (await resp.json()) as MeterReply;
}

const JOB_TTL_MS = 24 * 60 * 60 * 1000;
const SWEEP_INTERVAL_MS = 60 * 60 * 1000;

export class Meter implements DurableObject {
  private readonly storage: DurableObjectStorage;
  private readonly env: Env;

  constructor(state: DurableObjectState, env: Env) {
    this.storage = state.storage;
    this.env = env;
  }

  async fetch(request: Request): Promise<Response> {
    const req = (await request.json()) as MeterRequest;
    const reply = await this.handle(req);
    return new Response(JSON.stringify(reply), {
      headers: { "Content-Type": "application/json" },
    });
  }

  private async handle(req: MeterRequest): Promise<MeterReply> {
    const now = new Date();
    const minute = now.toISOString().slice(0, 16); // 2026-07-15T10:00
    const day = now.toISOString().slice(0, 10); // 2026-07-15

    // 分钟级限频（提交+轮询共用）
    if (req.op === "submit" || req.op === "poll") {
      const perMinute = intEnv(this.env.PER_MINUTE, 120);
      const reqKey = `req:${minute}:${req.machine}`;
      const count = ((await this.storage.get<number>(reqKey)) ?? 0) + 1;
      if (count > perMinute) {
        return { ok: false, status: 429, error: "请求过于频繁，请稍后再试" };
      }
      await this.storage.put(reqKey, count);
    }

    switch (req.op) {
      case "submit": {
        const limit = req.tier === "licensed"
          ? intEnv(this.env.LICENSED_DAILY, 200)
          : intEnv(this.env.TRIAL_DAILY, 20);
        const subKey = `sub:${day}:${req.machine}`;
        const count = ((await this.storage.get<number>(subKey)) ?? 0) + 1;
        if (count > limit) {
          return { ok: false, status: 403, error: "今日识别次数已达上限，请明日再试或联系开发者" };
        }
        await this.storage.put(subKey, count);
        await this.ensureAlarm();
        return { ok: true, status: 200, error: "" };
      }
      case "record": {
        if (req.jobId) {
          await this.storage.put(`job:${req.jobId}`, { machine: req.machine, at: Date.now() });
        }
        return { ok: true, status: 200, error: "" };
      }
      case "poll": {
        const owner = await this.storage.get<{ machine: string; at: number }>(`job:${req.jobId}`);
        if (!owner || owner.machine !== req.machine) {
          return { ok: false, status: 404, error: "作业不存在或无权访问" };
        }
        return { ok: true, status: 200, error: "" };
      }
      default:
        return { ok: false, status: 400, error: "未知操作" };
    }
  }

  /** 闹钟每小时清理：过期窗口计数与超过 24h 的作业归属 */
  async alarm(): Promise<void> {
    const now = new Date();
    const minute = now.toISOString().slice(0, 16);
    const day = now.toISOString().slice(0, 10);

    const entries = await this.storage.list();
    const stale: string[] = [];
    for (const [key, value] of entries) {
      if (key.startsWith("req:") && !key.startsWith(`req:${minute}:`)) stale.push(key);
      else if (key.startsWith("sub:") && !key.startsWith(`sub:${day}:`)) stale.push(key);
      else if (key.startsWith("job:")) {
        const at = (value as { at?: number }).at ?? 0;
        if (Date.now() - at > JOB_TTL_MS) stale.push(key);
      }
    }
    // storage.delete 单次上限 128 键，分批
    for (let i = 0; i < stale.length; i += 128) {
      await this.storage.delete(stale.slice(i, i + 128));
    }
    if ((await this.storage.list({ limit: 1 })).size > 0) {
      await this.storage.setAlarm(Date.now() + SWEEP_INTERVAL_MS);
    }
  }

  private async ensureAlarm(): Promise<void> {
    if ((await this.storage.getAlarm()) === null) {
      await this.storage.setAlarm(Date.now() + SWEEP_INTERVAL_MS);
    }
  }
}

function intEnv(v: string | undefined, def: number): number {
  const n = Number(v);
  return Number.isFinite(n) && n > 0 ? n : def;
}
