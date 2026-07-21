<p align="center">
  <img src="build/appicon.png" alt="Trawl Logo" width="120" height="120">
</p>

<h1 align="center">Trawl</h1>

<p align="center">
  <em>把成摞的法律文书，一网打捞成结构化数据</em>
</p>

<p align="center">
  简体中文 | <a href="README.en.md">English</a>
</p>

<p align="center">
  <a href="https://github.com/can4hou6joeng4/Trawl/releases/latest"><img src="https://img.shields.io/github/v/release/can4hou6joeng4/Trawl?style=flat-square&color=blue" alt="Latest Release"></a>
  <a href="https://github.com/can4hou6joeng4/Trawl/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/can4hou6joeng4/Trawl/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status"></a>
  <a href="https://github.com/can4hou6joeng4/Trawl/stargazers"><img src="https://img.shields.io/github/stars/can4hou6joeng4/Trawl?style=flat-square&color=yellow" alt="Stars"></a>
  <a href="https://github.com/can4hou6joeng4/Trawl/graphs/commit-activity"><img src="https://img.shields.io/github/commit-activity/m/can4hou6joeng4/Trawl?style=flat-square" alt="Commit Activity"></a>
  <a href="https://github.com/can4hou6joeng4/Trawl/blob/main/LICENSE"><img src="https://img.shields.io/github/license/can4hou6joeng4/Trawl?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat-square&logo=vue.js" alt="Vue Version">
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Windows-blue?style=flat-square" alt="Platform">
</p>

---

<p align="center">
  <img src="docs/images/app-screenshot.png" alt="Trawl 取证台界面截图" width="800">
</p>

---

## 🌊 为什么叫 Trawl

**Trawl**（拖网）。英语里 *trawl through documents* 正是「在成堆文件里翻找信息」——这恰好是本项目做的事：把散落在起诉状里的被告、身份证号、诉讼请求、事实与理由，像拖网捕捞一样，一次性打捞成整齐的结构化记录。

它是航海家族的一员：[Harbor](https://github.com/can4hou6joeng4/Harbor) 让知识停泊，[Beacon](https://github.com/can4hou6joeng4/Beacon) 在证件过期前发出预警，[Atlas](https://github.com/can4hou6joeng4/Atlas) 丈量每一段 AI 编码航程，而 **Trawl** 负责在文书的海里捕捞价值。

---

## ✨ 功能特性

- 🗂️ **取证台界面** - 真实原文页面与结构化记录左右对照，字段值原位高亮——文本层 PDF、扫描件、照片一致体验，点击高亮与「定位原文」双向回链。
- 🔑 **零配置云端识别** - 扫描件开箱即用，经零密钥中继网关识别，无需申请 Token。
- 📄 **智能解析** - 自动识别 `.docx`、`.pdf` 与图片（`.jpg` / `.png`）法律文书。
- ✏️ **校对即导出** - 提取结果可直接在记录卡上修正，人工修改随导出生效。
- 🎯 **精准提取** - 提取被告、身份证号码、诉讼请求、事实与理由等关键字段，支持字段显隐；坐标解析宁缺毋错。
- ⚡ **并行提取** - Go 协程并发处理，本地文本解析速度提升 **300%**。
- 🧩 **超长文档** - 50 页以上 PDF 单作业整体识别，云端真实页级进度。
- 💾 **多格式导出** - 支持 Excel (.xlsx), CSV, JSON 格式导出。

---

## ⚙️ 工作原理

```
 .docx ─────────────────────► 本地解析 ──────────┐
                                                 │
 .pdf ──► 文本层探测 ──┬─ 有文本 ► 并行本地解析 ─┤    ┌──────────────┐
                       └─ 扫描件 ► 云端 OCR ─────┼───►│  字段提取     │
                                  (零密钥网关)   │    │  被告/身份证  │──► 取证台校对 ──► XLSX / CSV / JSON
 图片 ──────────────────────► 云端 OCR ──────────┘    │  请求/事实    │      (原位高亮·双向定位)
                                                      └──────────────┘
```

应用自身**不携带任何云端密钥**：识别请求携带机器码，由官方中继网关按档计量（试用档每日限额，激活后更高额度）。

---

## 🚀 快速开始

1. 从 [Releases](https://github.com/can4hou6joeng4/Trawl/releases) 下载对应平台的安装包
2. **macOS Intel / Apple Silicon**: 下载通用 `.dmg`，打开后拖入应用程序文件夹
3. **Windows x64**: 运行 `windows_amd64_setup.exe` 安装程序

Windows ARM64 原生构建作为独立发布目标推进，因为 Windows OCR 桥接工具也需要对应 ARM64 产物和真机验证。

📖 **[使用手册](docs/user/USER_GUIDE.md)** —— 面向使用者与实施人员的完整说明（含验收清单与激活流程）。

### 使用步骤

1. **拖入或点击选择**法律文书，应用自动解析并直达取证台
2. 在**取证台**中校对记录：单击字段直接修正、切换字段显隐、点击「定位原文」跳到对应页
3. 选择导出格式（XLSX / CSV / JSON）并点击 **「导出」**，人工修正随文件一并保存

---

## 🔧 配置说明

**开箱即用，无需配置**。可选：自备 AI Studio Token（优先于网关直连官方接口，也是网关故障时的备用通道），在 `config/conf.yaml` 中配置：

```yaml
ocr:
  token: "您的AIStudioToken"   # 直连官方接口
```

或环境变量 `LEGAL_EXTRACTOR_OCR_TOKEN`。旧版 `baidu.token` 配置继续有效。

📖 **[详细配置指南](docs/user/CONFIG_GUIDE.md)**

---

## 🧰 技术栈

| 层           | 技术                                                         |
| :----------- | :----------------------------------------------------------- |
| 桌面框架     | [Wails v2](https://wails.io)（Go 后端 + 系统 WebView）        |
| 后端         | Go 1.25+ · 并行解析 · SHA256 内容缓存 · Viper 配置            |
| 前端         | Vue 3 + TypeScript + Vite · pdf.js 渲染与字符级高亮           |
| OCR          | PaddleOCR-VL 云端异步作业 · Cloudflare Workers 零密钥中继网关 |

---

## 🛠️ 本地开发

- Go 1.25+ / Node.js 22+ / [Wails CLI](https://wails.io/docs/gettingstarted/installation)

```bash
git clone https://github.com/can4hou6joeng4/Trawl.git
cd Trawl
cd frontend && npm install && cd ..
wails dev
```

常用命令：`make test`（Go 测试）、`make lint`、`make check`；前端在 `frontend/` 下 `npm test`。

---

## 📝 提取字段说明

| 字段           | 匹配规则                                   |
| :------------- | :----------------------------------------- |
| **被告**       | 从 "被告:" 关键词后提取姓名                |
| **身份证号码** | 自动识别 18 位身份证号码模式               |
| **诉讼请求**   | 提取 "诉讼请求" 与 "事实与理由" 之间的内容 |
| **事实与理由** | 提取 "事实与理由" 与 "此致" 之间的内容     |

---

## 📄 开源协议

[MIT License](LICENSE) © 2026

---

<p align="center">
  <sub>Made with ❤️ using <a href="https://wails.io">Wails</a> & <a href="https://vuejs.org/">Vue 3</a></sub>
</p>
