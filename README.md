<p align="center">
  <img src="build/appicon.png" alt="Legal Extractor Logo" width="120" height="120">
</p>

<h1 align="center">Legal Document Extractor</h1>

<p align="center">
  <strong>Next-Gen intelligent information extraction from legal documents with high-performance OCR</strong>
</p>

<p align="center">
  English | <a href="README.zh-CN.md">简体中文</a>
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
  <img src="docs/images/app-screenshot.png" alt="Legal Extractor Screenshot" width="800">
</p>

---

## ✨ Features

- 🚀 **v3.0 Next-Gen Engine** - Powered by Baidu AI Studio for high-precision legal document analysis.
- 📄 **Smart Parsing** - Auto-detect structure of `.docx` and `.pdf` legal documents.
- ⚡ **Parallel Processing** - 300% faster text extraction using Go Goroutines.
- 🎯 **Precise Extraction** - Extract key fields like defendant, ID, requests, and facts.
- 🧩 **Physical Slicing** - Support for 50+ pages long PDF documents.
- 👁️ **Live Preview** - Preview data before extraction to ensure accuracy.
- 💾 **Multi-format Export** - Support Excel (.xlsx), CSV, and JSON.

---

## 🚀 Quick Start

1. Download the installer for your platform from [Releases](https://github.com/can4hou6joeng4/legal-extractor/releases)
2. **macOS Intel / Apple Silicon**: Download the universal `.dmg`, open it, and drag the app to Applications
3. **Windows x64**: Run the `windows_amd64_setup.exe` installer

Windows ARM64 native builds are tracked as a separate release target because the Windows OCR bridge also needs a verified ARM64 artifact.

### Usage

1. Click **"Select Files"** to choose legal documents
2. Click **"Preview"** to verify extracted data (Optional)
3. Click **"Extract & Save"** to export structured data

---

## 🛠️ Development

### Prerequisites

- Go 1.25+
- Node.js 22+
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### Setup

```bash
# Clone project
git clone https://github.com/can4hou6joeng4/legal-extractor.git
cd legal-extractor

# Install dependencies
cd frontend && npm install && cd ..
```

```bash
wails dev
```

---

## ⚙️ Configuration

### Baidu OCR (Required for PDF)

The project uses Baidu AI Studio (PaddleOCR-VL) for high-precision document analysis.

📖 **[Read the Full Configuration Guide](docs/user/CONFIG_GUIDE.md)**

**Option 1: Environment Variables**
- `LEGAL_EXTRACTOR_BAIDU_TOKEN` (Access Token for Baidu Cloud)

**Option 2: Configuration File**
Create `config/conf.yaml`:
```yaml
baidu:
  token: "your_baidu_token"
```

---

## 📁 Project Structure

```
legal-extractor/
├── main.go              # Wails desktop entrypoint
├── internal/            # Core logic
│   ├── app/             # Desktop App Logic (Wails bindings)
│   ├── config/          # Configuration management
│   ├── extractor/       # Extraction Engine (PDF/DOCX/OCR)
├── frontend/            # Vue 3 Desktop UI
│   ├── src/services/    # Wails API adapter
├── build/               # Build assets & installer config
└── README.md
```

---

## 📝 Extraction Fields

| Field         | Rule                                           |
| :------------ | :--------------------------------------------- |
| **Defendant** | Extracted from text after "被告:" (Defendant:) |
| **ID Number** | 18-digit ID number patterns                    |
| **Requests**  | Content between "诉讼请求" and "事实与理由"    |
| **Facts**     | Content between "事实与理由" and "此致"        |

---

## 📄 License

[MIT License](LICENSE) © 2026

---

<p align="center">
  <sub>Made with ❤️ using <a href="https://wails.io">Wails</a> & <a href="https://vuejs.org/">Vue 3</a></sub>
</p>
