# 安全政策

## 支持的版本

| 版本 | 支持状态 |
| --- | --- |
| 3.0.x | :white_check_mark: 当前支持 |
| < 3.0 | :x: 不再支持 |

## 报告安全漏洞

如果你发现了安全漏洞，**请勿通过公开 Issue 提交**。

请通过以下方式私密报告：

1. 发送邮件至仓库所有者（可通过 GitHub 个人页面获取联系方式）
2. 或使用 [GitHub Security Advisories](https://github.com/can4hou6joeng4/legal-extractor/security/advisories/new) 私密报告

我们会在收到报告后 72 小时内确认，并在 7 个工作日内提供修复计划。

## 安全最佳实践

- 百度 OCR Token 应通过环境变量注入，不要提交到代码仓库
- 生产环境部署时请确保 Docker 镜像来自官方 Release
- 定期更新至最新版本以获取安全补丁
