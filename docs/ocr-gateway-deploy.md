# OCR 中继网关部署指引

网关是一个无状态 Go 单二进制（`cmd/ocr-gateway`），职责：校验应用的
机器码/授权 → 换上真实 AI Studio Token → 转发 v2 作业式接口。
应用安装包不携带任何云端密钥；结果 JSONL 是签名 URL，客户端直连下载不过网关。

## 构建

```bash
make gateway                        # 产出 build/bin/ocr-gateway（当前平台）
GOOS=linux GOARCH=amd64 make gateway  # 交叉编译 Linux 服务器版
```

## 配置（环境变量）

| 变量 | 必填 | 说明 |
|---|---|---|
| `OCR_GW_TOKEN` | ✅ | 真实 AI Studio Token（**建议部署前先在 AI Studio 轮换一次**） |
| `OCR_GW_LICENSE_SECRET` | 推荐 | 与应用发布构建注入的 `LicenseSecret` 一致；缺省则所有调用按试用档 |
| `OCR_GW_UPSTREAM` | | 上游作业接口，默认官方 `https://paddleocr.aistudio-app.com/api/v2/ocr/jobs` |
| `OCR_GW_LISTEN` | | 监听地址，默认 `:8787` |
| `OCR_GW_TRIAL_DAILY` | | 试用档每机器每日提交上限，默认 20 |
| `OCR_GW_LICENSED_DAILY` | | 授权档每机器每日提交上限，默认 200 |

## 部署（任选其一，须境内可达）

### 方式 A：VPS + systemd（最简）

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

### 方式 B：腾讯云函数 / 阿里云 FC（免运维）

以「自定义运行时/Web 函数」方式部署 Linux 二进制，监听端口按平台约定
（读 `OCR_GW_LISTEN` 注入），环境变量在控制台配置。免费额度足以覆盖
（一次识别 ≈ 1 提交 + ~10 轮询）。

## 验证

```bash
curl https://你的域名/healthz                      # → ok
curl -X POST https://你的域名/api/v2/ocr/jobs      # 无机器码 → 401
```

## 上线收尾

部署完成后把网关地址回填为应用默认值
（`internal/config/config.go` 的 `ocr.gateway_url` 默认值与 conf.yaml 模板），
随 v4.2.0 发布。之后轮换上游 Token 只改网关环境变量，应用零发版。

## 运维备忘

- 网关日志每请求一行（机器码/档位/状态码/耗时），**永不打印 Token**。
- 配额为单实例内存计量：多实例部署需外置存储（暂无必要，见 PRD 开放问题 6）。
- 应急：网关故障时，用户可在 `config/conf.yaml` 填自备 Token 直连官方接口（覆盖链）。
