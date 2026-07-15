<p align="center">
  <img src="build/appicon.png" alt="Legal Extractor Logo" width="120" height="120">
</p>

<h1 align="center">法律文书提取器 (Legal Document Extractor)</h1>

<p align="center">
  <strong>新一代法律文书智能提取工具，基于高性能 OCR 与并行解析引擎</strong>
</p>

<p align="center">
  <a href="README.md">English</a> | 简体中文
</p>

<p align="center">
  <a href="https://github.com/can4hou6joeng4/legal-extractor/releases/latest"><img src="https://img.shields.io/github/v/release/can4hou6joeng4/legal-extractor?style=flat-square&color=blue" alt="Latest Release"></a>
  <a href="https://github.com/can4hou6joeng4/legal-extractor/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/can4hou6joeng4/legal-extractor/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status"></a>
  <a href="https://codecov.io/gh/can4hou6joeng4/legal-extractor"><img src="https://img.shields.io/codecov/c/github/can4hou6joeng4/legal-extractor?style=flat-square&label=Coverage" alt="Coverage"></a>
  <a href="https://goreportcard.com/report/github.com/can4hou6joeng4/legal-extractor"><img src="https://goreportcard.com/badge/github.com/can4hou6joeng4/legal-extractor?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/can4hou6joeng4/legal-extractor/blob/main/LICENSE"><img src="https://img.shields.io/github/license/can4hou6joeng4/legal-extractor?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/Vue-3.x-4FC08D?style=flat-square&logo=vue.js" alt="Vue Version">
  <img src="https://img.shields.io/badge/Platform-macOS%20%7C%20Windows-blue?style=flat-square" alt="Platform">
</p>

---

<p align="center">
  <img src="docs/images/app-screenshot.png" alt="Legal Extractor 界面截图" width="800">
</p>

---

## ✨ 功能特性

- 🗂️ **v4.2 取证台界面** - 真实原文页面与结构化记录左右对照，字段值原位高亮——文本层 PDF、扫描件、照片一致体验，点击高亮与「定位原文」双向回链。
- 🔑 **零配置云端识别** - 扫描件开箱即用，经零密钥中继网关识别，无需申请 Token。
- 📄 **智能解析** - 自动识别 `.docx`、`.pdf` 与图片（`.jpg` / `.png`）法律文书。
- ✏️ **校对即导出** - 提取结果可直接在记录卡上修正，人工修改随导出生效。
- 🎯 **精准提取** - 提取被告、身份证号码、诉讼请求、事实与理由等关键字段，支持字段显隐。
- ⚡ **并行提取** - Go 协程并发处理，本地文本解析速度提升 **300%**。
- 🧩 **超长文档** - 50 页以上 PDF 单作业整体识别，云端真实页级进度。
- 💾 **多格式导出** - 支持 Excel (.xlsx), CSV, JSON 格式导出。

---

## 🚀 快速开始

1. 从 [Releases](https://github.com/can4hou6joeng4/legal-extractor/releases) 下载对应平台的安装包
2. **macOS Intel / Apple Silicon**: 下载通用 `.dmg`，打开后拖入应用程序文件夹
3. **Windows x64**: 运行 `windows_amd64_setup.exe` 安装程序

Windows ARM64 原生构建作为独立发布目标推进，因为 Windows OCR 桥接工具也需要对应 ARM64 产物和真机验证。

📖 **[使用手册](docs/user/USER_GUIDE.md)** —— 面向使用者与实施人员的完整说明（含验收清单与激活流程）。

### 使用步骤

1. **拖入或点击选择**法律文书，应用自动解析并直达取证台
2. 在**取证台**中校对记录：单击字段直接修正、切换字段显隐、点击"定位原文"跳到对应页
3. 选择导出格式（XLSX / CSV / JSON）并点击 **"导出"**，人工修正随文件一并保存

---

## 🛠️ 开发指南

### 环境要求

- Go 1.25+
- Node.js 22+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### 安装依赖

```bash
# 克隆项目
git clone https://github.com/can4hou6joeng4/legal-extractor.git
cd legal-extractor

# 安装依赖
cd frontend && npm install && cd ..
```

```bash
wails dev
```

---

## ⚙️ 配置说明

**开箱即用，无需配置**——扫描件与图片识别默认经官方中继网关完成，无需申请或填写任何 Token。应用**不携带任何云端密钥**：请求携带机器码，由网关按档计量（试用档每日限额，激活后更高额度）。

**可选：自备 AI Studio Token**（优先于网关直连官方接口，也是网关故障时的备用通道）。在 `config/conf.yaml` 中配置：

```yaml
ocr:
  token: "您的AIStudioToken"   # 直连官方接口
```

或环境变量 `LEGAL_EXTRACTOR_OCR_TOKEN`。旧版 `baidu.token` 配置继续有效。

📖 **[点击查看详细配置指南](docs/user/CONFIG_GUIDE.md)**

---

## 📁 项目结构

```
legal-extractor/
├── main.go              # Wails 桌面端入口
├── internal/            # 核心业务逻辑
│   ├── app/             # 桌面端逻辑 (Wails 绑定)
│   ├── config/          # 配置管理
│   ├── extractor/       # 提取引擎 (PDF/DOCX/OCR)
├── frontend/            # Vue 3 桌面端界面
│   ├── src/services/    # Wails API 适配层
├── build/               # 构建资源与安装程序配置
└── README.md
```

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
