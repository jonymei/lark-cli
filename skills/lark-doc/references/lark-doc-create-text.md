# docs +create-text

使用 OpenAPI 创建飞书文档（纯文本）。

## 与 `docs +create` 的区别

| 特性 | `docs +create` (MCP) | `docs +create-text` (OpenAPI) |
|------|---------------------|------------------------------|
| 协议 | MCP (Model Context Protocol) | OpenAPI (RESTful) |
| 输入格式 | Markdown | 纯文本 |
| 私有化部署 | ❌ 可能不支持 | ✅ 完全支持 |
| 格式支持 | 丰富（标题、列表、代码块等） | 基础（纯文本段落） |

## 用法

```bash
# 基础用法
lark-cli docs +create-text --text "Hello World" --title "My Document"

# 多行文本
lark-cli docs +create-text \
  --text "第一段\n第二段\n第三段" \
  --title "多段落文档"

# 指定父文件夹
lark-cli docs +create-text \
  --text "Content" \
  --title "Doc in Folder" \
  --folder-token "fldcnXXXXXXXXXXXXXXXXXXXXX"

# Dry run（查看将要调用的 API）
lark-cli docs +create-text \
  --text "Test" \
  --title "Test Doc" \
  --dry-run
```

## 参数

| 参数 | 必需 | 说明 |
|------|------|------|
| `--text` | 是 | 文档内容（纯文本，使用 `\n` 分隔段落） |
| `--title` | 否 | 文档标题 |
| `--folder-token` | 否 | 父文件夹 token |
| `--as` | 否 | 身份类型：user（默认）或 bot |
| `--dry-run` | 否 | 仅显示将要调用的 API，不实际执行 |

## 权限要求

- `docx:document:create` - 创建文档
- `docx:document` - 编辑文档内容

## 输出

```json
{
  "document_id": "doxcnXXXXXXXXXXXXXXXXXXXXX",
  "block_id": "doxcnXXXXXXXXXXXXXXXXXXXXX",
  "title": "My Document",
  "doc_url": "https://feishu.cn/docx/doxcnXXXXXXXXXXXXXXXXXXXXX"
}
```

## 实现原理

该命令分两步调用 OpenAPI：

### 1. 创建空文档

```
POST /open-apis/docx/v1/documents
{
  "title": "文档标题",
  "folder_token": "可选"
}
```

返回：
- `document_id` - 文档 ID
- `block_id` - 根 block ID

### 2. 添加文本内容

```
POST /open-apis/docx/v1/documents/{document_id}/blocks/{block_id}/children
{
  "children": [
    {
      "block_type": 2,  // text block
      "text": {
        "elements": [
          {
            "text_run": {
              "content": "段落内容"
            }
          }
        ],
        "style": {}
      }
    }
  ],
  "index": 0
}
```

## 文本格式处理

- 输入文本按 `\n` 分割为多个段落
- 每个非空段落创建一个独立的 text block
- 空行会被自动跳过

## 示例

### 创建单段落文档

```bash
lark-cli docs +create-text \
  --text "这是一个简单的文档" \
  --title "简单文档"
```

### 创建多段落文档

```bash
lark-cli docs +create-text \
  --text "第一段：介绍\n\n第二段：详细说明\n\n第三段：总结" \
  --title "结构化文档"
```

### 从文件读取内容

```bash
lark-cli docs +create-text \
  --text "$(cat content.txt)" \
  --title "从文件创建"
```

## 限制

1. **仅支持纯文本** - 不支持 Markdown 格式（标题、列表、代码块等）
2. **无样式** - 不支持加粗、斜体、颜色等文本样式
3. **段落级别** - 只能创建文本段落，不支持其他 block 类型

## 高级用法

如果需要更丰富的格式，可以：

1. **使用 `docs +create`** (MCP) - 支持 Markdown（需要 MCP 服务）
2. **手动构造 blocks** - 使用 `lark-cli api` 直接调用 OpenAPI，传递完整的 block 结构

## 错误处理

如果文档创建成功但添加内容失败，会返回错误信息：

```
document created but failed to add content: <error details>
```

此时文档已创建，但内容为空，可以手动编辑或使用 `docs +update` 添加内容。

## 相关命令

- `docs +create` - 使用 MCP 从 Markdown 创建文档（功能更强大）
- `docs +update` - 更新文档内容
- `docs +fetch` - 获取文档内容
- `docs +search` - 搜索文档

## API 文档

- [创建文档](https://open.feishu.cn/document/ukTMukTMukTM/uUDN04SN0QjL1QDN/document-docx/docx-v1/document/create)
- [添加 Block](https://open.feishu.cn/document/ukTMukTMukTM/uUDN04SN0QjL1QDN/document-docx/docx-v1/document-block-children/create)
