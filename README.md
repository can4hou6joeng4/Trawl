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

- 🗂️ **v4.2 Evidence Desk UI** - Real source pages rendered beside structured records; extracted fields are highlighted in place — on text-layer PDFs, scanned PDFs and photos alike — with two-way locate between page and record.
- 🔑 **Zero-config OCR** - Scanned documents work out of the box through the credential-free relay gateway; no token to apply for.
- 📄 **Smart Parsing** - Auto-detect `.docx`, `.pdf`, and image (`.jpg` / `.png`) legal documents.
- ✏️ **Review-then-Export** - Edit extracted values directly on record cards; manual corrections flow into the export.
- 🎯 **Precise Extraction** - Extract key fields (defendant, ID, requests, facts) with per-field visibility toggles.
- ⚡ **Parallel Processing** - 300% faster text extraction using Go Goroutines.
- 🧩 **Long Documents** - 50+ page PDFs handled in a single cloud job with real per-page progress.
- 💾 **Multi-format Export** - Support Excel (.xlsx), CSV, and JSON.

---

## 🚀 Quick Start

1. Download the installer for your platform from [Releases](https://github.com/can4hou6joeng4/legal-extractor/releases)
2. **macOS Intel / Apple Silicon**: Download the universal `.dmg`, open it, and drag the app to Applications
3. **Windows x64**: Run the `windows_amd64_setup.exe` installer

Windows ARM64 native builds are tracked as a separate release target because the Windows OCR bridge also needs a verified ARM64 artifact.

### Usage

1. **Drag in or click to select** a legal document — parsing starts automatically and lands on the Evidence Desk
2. Review records on the desk: click any field to correct it, toggle field visibility, or hit **"Locate"** to jump to the source page
3. Pick an export format (XLSX / CSV / JSON) and click **"Export"** — manual corrections are saved with the file

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

**Works out of the box** — scanned PDFs and images are recognized through the official OCR relay gateway; no token or setup required. The app ships with **zero cloud credentials**: requests carry your machine ID, and the gateway meters usage (trial tier daily quota; licensed tier gets more).

**Optional: bring your own AI Studio token** (takes priority over the gateway; also a fallback if the gateway is unreachable). Create `config/conf.yaml`:

```yaml
ocr:
  token: "your_aistudio_token"   # direct connection to the official API
```

Or via environment variable: `LEGAL_EXTRACTOR_OCR_TOKEN`. Legacy `baidu.token` configs keep working.

📖 **[Read the Full Configuration Guide](docs/user/CONFIG_GUIDE.md)**

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
