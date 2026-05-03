# Changelog

本项目所有重要变更记录。格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)。

## [3.1.0] - 2026-05-03

### 修复
- `wails.json` 中残留的 2.0.0 版本号
- `docker-compose.yml` 引用了应用从未读取的 `BAIDU_API_KEY/SECRET_KEY` 环境变量

### 新增
- 基于 SHA-256 的有界 LRU 缓存（容量 50），替代之前无界增长的内存映射
- `context.Context` 全链路传播：HTTP 请求中止/百度长任务/Windows OCR 子进程都可被取消
- 结构化错误类型（`ErrPDFEncrypted`、`ErrUnsupportedFormat`、`ErrTokenMissing`、`ErrCancelled` 等），消除字符串匹配

## [3.0.0] - 2026-02-03

### 新增
- 全面转向百度 PaddleOCR-VL 视觉大模型
- 并行提取引擎，本地文本层解析支持多核并行加速
- 大文件物理分割，超长 PDF 自动分块处理
- 原子化按页解析，每页独立提取法律实体

### 修复
- 修复特定 PDF 导致的解析挂起问题
- 修复构建缓存导致时间戳失效的问题

### 变更
- 移除腾讯云 OCR 全部逻辑
- CI 自动注入百度 Token 至构建产物

## [2.1.5] - 2026-01-26

### 变更
- OCR 服务从百度云迁移至腾讯云 SmartStructuralOCRV2
- 配置项从 3 个减少到 2 个

## [2.1.0] - 2026-01-22

### 新增
- Web 模式支持，基于 Docker 的浏览器端使用
- REST API 接口
- IO 解耦重构，核心引擎支持内存数据流

## [2.0.0] - 2026-01-21

### 新增
- 接入百度 PaddleOCR-VL 视觉大模型
- 内存缓存机制，同一文件只需识别一次
- 安装包体积缩减 90%

### 移除
- 本地 Python 环境和 OCR 库依赖

## [1.1.0] - 2026-01-20

### 新增
- Python 桥接引擎，电子章干扰清除
- 智能合并算法
- GitHub Actions 自动化构建

## [1.0.0] - 2026-01-17

### 新增
- 首个正式版本
- 支持 docx 和 pdf 法律文书智能解析
- 实时预览与多格式导出
- 暗色玻璃拟态 UI

[3.1.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v3.1.0
[3.0.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v3.0.0
[2.1.5]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v2.1.5
[2.1.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v2.1.0
[2.0.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v2.0.0
[1.1.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v1.1.0
[1.0.0]: https://github.com/can4hou6joeng4/legal-extractor/releases/tag/v1.0.0
