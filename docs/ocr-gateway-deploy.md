# OCR 中继网关部署指引

网关职责：校验应用的机器码/授权 → 换上真实 AI Studio Token → 转发 v2 作业式接口。
应用安装包不携带任何云端密钥；结果 JSONL 是签名 URL，客户端直连下载不过网关。

两个实现二选一，行为逐项对齐：

| | Cloudflare Workers 版（`gateway-worker/`） | Go 单二进制版（`cmd/ocr-gateway`） |
|---|---|---|
| 适用 | 已有 CF 托管域名、零服务器 | VPS / 国内云函数 |
| 计量存储 | Durable Object（跨实例一致） | 单实例内存 |
| 境内可达性 | 通常可用、偶发波动（走 CF 全球边缘） | 取决于部署位置（境内/港区最稳） |

## 方式 A：Cloudflare Workers（零服务器）

前置：域名已托管在 Cloudflare；`*.workers.dev` 境内不可用，**必须绑定自定义域名**。

```bash
cd gateway-worker
npm install

# 1. 在 wrangler.toml 中取消 routes 注释并填入实际主机名（如 ocr-gw.example.com）
# 2. 注入密钥（交互式输入，不落盘不进 shell 历史）：
CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ACCOUNT_ID=... npx wrangler secret put OCR_GW_TOKEN
CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ACCOUNT_ID=... npx wrangler secret put OCR_GW_LICENSE_SECRET
# 3. 部署（自动创建自定义域名的 DNS 记录与证书）：
CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ACCOUNT_ID=... npx wrangler deploy
```

API Token 权限（自定义 Token）：模板「编辑 Cloudflare Workers」，
Account Resources 选本账号，Zone Resources 选目标域名所在 Zone，
另加 `Zone → DNS → Edit`（自定义域名需创建 DNS 记录）。

配额等参数在 `wrangler.toml` 的 `[vars]` 调整（TRIAL_DAILY/LICENSED_DAILY/PER_MINUTE）。

## 方式 B：Go 单二进制（VPS / 云函数）

### 构建

```bash
make gateway                        # 产出 build/bin/ocr-gateway（当前平台）
GOOS=linux GOARCH=amd64 make gateway  # 交叉编译 Linux 服务器版
```

### 配置（环境变量）

| 变量 | 必填 | 说明 |
|---|---|---|
| `OCR_GW_TOKEN` | ✅ | 真实 AI Studio Token（**建议部署前先在 AI Studio 轮换一次**） |
| `OCR_GW_LICENSE_SECRET` | 推荐 | 与应用发布构建注入的 `LicenseSecret` 一致；缺省则所有调用按试用档 |
| `OCR_GW_UPSTREAM` | | 上游作业接口，默认官方 `https://paddleocr.aistudio-app.com/api/v2/ocr/jobs` |
| `OCR_GW_LISTEN` | | 监听地址，默认 `:8787` |
| `OCR_GW_TRIAL_DAILY` | | 试用档每机器每日提交上限，默认 20 |
| `OCR_GW_LICENSED_DAILY` | | 授权档每机器每日提交上限，默认 200 |

### VPS + systemd

```bash
scp build/bin/ocr-gateway server:/opt/ocr-gateway/
# /etc/systemd/system/ocr-gateway.service：
#   [Service]
#   Environment=OCR_GW_TOKEN=***
#   Environment=OCR_GW_LICENSE_SECRET=***
#   ExecStart=/opt/ocr-gateway/ocr-gateway
#   Restart=always
systemctl enable --now ocr-gateway
```

前置 HTTPS：用 Caddy/Nginx 反代加证书（客户端默认要求 https）。

### 腾讯云函数 / 阿里云 FC

以「自定义运行时/Web 函数」方式部署 Linux 二进制，监听端口按平台约定
（读 `OCR_GW_LISTEN` 注入），环境变量在控制台配置。默认域名免备案。

## 验证（两种方式通用）

```bash
curl https://你的域名/healthz                      # → ok
curl -X POST https://你的域名/api/v2/ocr/jobs      # 无机器码 → 401
```

## 上线收尾

部署完成后把网关地址回填为应用默认值
（`internal/config/config.go` 的 `ocr.gateway_url` 默认值与 conf.yaml 模板），
随 v4.2.0 发布。之后轮换上游 Token 只改网关密钥/环境变量，应用零发版。

## 运维备忘

- 日志每请求一行（机器码/档位/状态码），**永不打印 Token**。
- Go 版配额为单实例内存计量；Workers 版存 Durable Object，天然跨实例。
- 应急：网关故障时，用户可在 `config/conf.yaml` 填自备 Token 直连官方接口（覆盖链）。
