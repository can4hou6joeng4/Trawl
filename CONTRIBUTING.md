# 贡献指南

感谢你对 Legal Extractor 的关注！欢迎通过以下方式参与贡献。

## 如何贡献

### 报告问题

- 使用 [Issue 模板](https://github.com/can4hou6joeng4/legal-extractor/issues/new/choose) 提交 Bug 或功能建议
- 提交前请先搜索是否已有相关 Issue

### 提交代码

1. Fork 本仓库
2. 创建特性分支：`git checkout -b feat/你的功能`
3. 提交变更（遵循下方 Commit 规范）
4. 推送分支：`git push origin feat/你的功能`
5. 创建 Pull Request

### Commit 规范

```
type: 纯中文描述
```

type 类型：`feat` / `fix` / `refactor` / `perf` / `docs` / `test` / `chore` / `ci`

示例：
- `feat: 新增配置管理命令`
- `fix: 修复导出文件编码异常`
- `test: 补齐缓存模块测试覆盖`

### 开发环境

```bash
# 前置条件
# - Go 1.24+
# - Node.js 18+
# - Wails CLI (桌面版开发)
# - golangci-lint (代码检查): brew install golangci-lint

# 克隆并安装依赖
git clone https://github.com/can4hou6joeng4/legal-extractor.git
cd legal-extractor
cd frontend && npm install && cd ..

# 运行测试
make test

# 代码检查
make lint

# 启动开发模式
wails dev
```

### 代码质量要求

- 提交前确保 `make lint` 无报错
- 新增功能必须附带测试用例
- 测试通过：`make test`

## 行为准则

参与本项目即代表你同意遵守 [行为准则](CODE_OF_CONDUCT.md)。
