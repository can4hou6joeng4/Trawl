# 配置指南

> 适用版本：v4.2.0 及以上 ｜ 上手与功能说明见 [使用手册](USER_GUIDE.md)

## 默认无需任何配置

v4.2.0 起，扫描件与照片的云端识别默认经**官方中继网关**完成：
应用不携带、也不要求用户申请任何云服务密钥。安装即用。

网关按机器每日计量（试用 20 次/日、激活 200 次/日）。以下配置全部为**可选**。

## 配置文件位置

应用可执行文件同级的 `config/conf.yaml`，首次运行自动生成模板：

- **macOS**：`legal-extractor.app/Contents/MacOS/config/conf.yaml`
  （访达中对应用右键 → 显示包内容）
- **Windows**：安装目录下 `config\conf.yaml`

```yaml
ocr:
  token: ""        # 可选：自备 AI Studio Token 时直连官方接口（优先于网关）
  gateway_url: "https://ocr-gw.bobochang.cn/api/v2/ocr/jobs"  # OCR 中继网关
  model: "PaddleOCR-VL-1.6"
```

**生效时机**：修改 `conf.yaml` 后需重启应用；环境变量修改即刻生效。

## 可选项一：自备 Token 直连

适合希望使用自己云端额度、或网关临时不可用时自救的用户：

1. 登录 [百度 AI Studio](https://aistudio.baidu.com/)，在「访问令牌」页获取 Token
2. 填入 `ocr.token`（或设置环境变量），重启应用

自备 Token 存在时**优先于网关**，请求直连官方接口，不占网关额度。

## 可选项二：环境变量

所有配置项都可用环境变量覆盖，前缀 `LEGAL_EXTRACTOR_`，层级点号换下划线：

| 环境变量 | 对应配置 |
|---|---|
| `LEGAL_EXTRACTOR_OCR_TOKEN` | `ocr.token` |
| `LEGAL_EXTRACTOR_OCR_GATEWAY_URL` | `ocr.gateway_url` |
| `LEGAL_EXTRACTOR_OCR_MODEL` | `ocr.model` |

## 旧版本配置兼容

v4.1 及之前的 `baidu.token` 配置继续有效（自动等价为 `ocr.token`），
建议迁移到新键名。旧的 `baidu.api_url`（同步接口）已废弃，不再读取。

## 常见问题

**Q：不配置任何东西，扫描件识别用的是谁的额度？**
A：软件提供方在网关侧统一承担，按你的机器每日限额（见上）。

**Q：公司内网能用吗？**
A：只需放行对 `ocr-gw.bobochang.cn` 与百度云对象存储（`*.bcebos.com`）的
HTTPS 出站。Word 与文字版 PDF 全程本地解析，可完全离线。

**Q：如何确认当前走的是网关还是自备 Token？**
A：`ocr.token` 非空即直连官方接口；为空则走网关。
