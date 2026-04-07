# docs +create-markdown

使用 OpenAPI 导入功能从 Markdown 文件创建飞书文档。

## 特点

- ✅ 使用 OpenAPI，私有化部署完全支持
- ✅ 支持完整的 Markdown 格式（标题、列表、代码块、表格等）
- ✅ 自动转换为飞书文档格式
- ✅ 支持指定父文件夹
- ⚠️ 需要上传文件（最大 20MB）

## 与其他命令对比

| 命令 | 协议 | Markdown 支持 | 私有化部署 | 实现方式 |
|------|------|--------------|-----------|---------|
| `docs +create` | MCP | ✅ 完整 | ❌ | MCP 服务端转换 |
| `docs +create-text` | OpenAPI | ❌ 仅纯文本 | ✅ | 客户端构造 blocks |
| `docs +create-markdown` | OpenAPI | ✅ 完整 | ✅ | 飞书导入服务 |

**推荐使用 `docs +create-markdown`**：既支持 Markdown，又兼容私有化部署。

## 用法

```bash
# 基础用法
lark-cli docs +create-markdown --file document.md

# 指定标题
lark-cli docs +create-markdown \
  --file document.md \
  --title "我的文档"

# 指定父文件夹
lark-cli docs +create-markdown \
  --file document.md \
  --title "项目文档" \
  --folder-token "fldcnXXXXXXXXXXXXXXXXXXXXX"

# Dry run（查看 API 调用）
lark-cli docs +create-markdown \
  --file document.md \
  --dry-run
```

## 参数

| 参数 | 必需 | 说明 |
|------|------|------|
| `--file` | 是 | Markdown 文件路径（.md, .markdown, .mark） |
| `--title` | 否 | 文档标题（默认：文件名） |
| `--folder-token` | 否 | 父文件夹 token |
| `--as` | 否 | 身份类型：user（默认）或 bot |
| `--dry-run` | 否 | 仅显示 API 调用，不实际执行 |

## 权限要求

- `drive:drive:readonly` - 读取云空间
- `drive:drive` - 管理云空间文件

## 实现原理

该命令使用飞书的文件导入功能，分三步完成：

### Step 1: 上传 Markdown 文件

```
POST /open-apis/drive/v1/medias/upload_all
{
  "file_name": "document.md",
  "parent_type": "ccm_import_open",  // 特殊类型：用于导入
  "extra": "{\"obj_type\":\"docx\",\"file_extension\":\"md\"}",
  "file": <binary>
}
```

返回：`file_token`

### Step 2: 创建导入任务

```
POST /open-apis/drive/v1/import_tasks
{
  "file_extension": "md",
  "file_token": "boxcnXXX",
  "type": "docx",
  "file_name": "我的文档",
  "point": {
    "mount_type": 1,
    "mount_key": "fldcnXXX"  // 可选：父文件夹
  }
}
```

返回：`ticket`（任务 ID）

### Step 3: 轮询导入结果

```
GET /open-apis/drive/v1/import_tasks/{ticket}
```

返回：
```json
{
  "result": {
    "job_status": 0,  // 0=成功, 1=进行中, 2=失败
    "token": "doxcnXXX"  // 文档 ID
  }
}
```

## 输出

```json
{
  "document_id": "doxcnXXXXXXXXXXXXXXXXXXXXX",
  "title": "我的文档",
  "doc_url": "https://feishu.cn/docx/doxcnXXXXXXXXXXXXXXXXXXXXX"
}
```

## 支持的 Markdown 格式

飞书导入服务支持以下 Markdown 格式：

- ✅ 标题（H1-H6）
- ✅ 段落
- ✅ 列表（有序、无序）
- ✅ 代码块
- ✅ 行内代码
- ✅ 加粗、斜体
- ✅ 链接
- ✅ 图片
- ✅ 引用
- ✅ 表格
- ✅ 分隔线

## 限制

1. **文件大小**：最大 20MB
2. **文件扩展名**：必须是 `.md`、`.markdown` 或 `.mark`
3. **导入时间**：通常 2-10 秒，最长等待 60 秒
4. **临时文件**：上传的源文件会在导入完成后自动删除

## 示例

### 创建简单文档

```bash
# 创建 Markdown 文件
cat > /tmp/hello.md << 'EOF'
# Hello World

这是我的第一个文档。

## 特性

- 简单
- 快速
- 强大
EOF

# 导入为飞书文档
lark-cli docs +create-markdown --file /tmp/hello.md
```

### 从 GitHub README 创建文档

```bash
# 下载 README
curl -o /tmp/README.md https://raw.githubusercontent.com/user/repo/main/README.md

# 导入为飞书文档
lark-cli docs +create-markdown \
  --file /tmp/README.md \
  --title "项目 README"
```

### 批量导入文档

```bash
#!/bin/bash

# 批量导入目录下的所有 Markdown 文件
for file in docs/*.md; do
  title=$(basename "$file" .md)
  echo "Importing: $title"
  lark-cli docs +create-markdown \
    --file "$file" \
    --title "$title" \
    --folder-token "fldcnXXXXXXXXXXXXXXXXXXXXX"
  sleep 2
done
```

## 错误处理

### 导入失败

如果导入失败，会显示详细错误信息：

```
import failed: import failed: 文件格式不支持
```

常见错误：
- `1069910` - 文件扩展名不匹配
- `1069902` - 权限不足
- `timeout` - 导入超时（文件过大或服务繁忙）

### 重试机制

命令会自动轮询导入结果，最多尝试 30 次（约 60 秒）。如果超时，可以手动重试。

## 与 MCP 版本的区别

| 特性 | MCP (`docs +create`) | OpenAPI (`docs +create-markdown`) |
|------|---------------------|----------------------------------|
| 输入方式 | 直接传递 Markdown 字符串 | 上传 Markdown 文件 |
| 文件大小限制 | 无明确限制 | 20MB |
| 私有化部署 | ❌ 需要 MCP 服务 | ✅ 完全支持 |
| 转换质量 | 高（MCP 优化） | 高（飞书官方导入） |
| 速度 | 快（同步） | 较慢（异步，需轮询） |

## 相关命令

- `docs +create` - 使用 MCP 从 Markdown 创建文档（功能更强大，但需要 MCP）
- `docs +create-text` - 使用 OpenAPI 创建纯文本文档（最简单）
- `docs +update` - 更新文档内容
- `docs +fetch` - 获取文档内容

## API 文档

- [上传素材](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/drive-v1/media/upload_all)
- [创建导入任务](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/drive-v1/import_task/create)
- [查询导入结果](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/drive-v1/import_task/get)
- [导入文件概述](https://open.feishu.cn/document/uAjLw4CM/ukTMukTMukTM/reference/drive-v1/import_task/import-user-guide)
