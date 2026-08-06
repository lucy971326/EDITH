# project

`project.Module` 是当前 `ProjectRoot` 的唯一文件读取入口。它只持有规范化后的根目录，不缓存仓库内容。

## 对外能力

| 方法 | 输入 | 输出 |
| --- | --- | --- |
| `ListChildren(relativeDir)` | 相对目录；空字符串是根目录 | 该目录的直接子项，目录优先、名称稳定排序 |
| `ReadText(relativeFile)` | 相对普通文本文件 | 文件内容、推断语言与截断标记 |

## 路径边界

- 只接受相对 `ProjectRoot` 的路径；绝对路径和 `..` 逃逸会被拒绝。
- 每次读取都用 `filepath.Rel` 验证边界，不依赖字符串前缀。
- 解析符号链接后仍必须位于根目录；逃出根目录或无效的链接不会被跟随。
- 首版不递归扫描、缓存、监听或写入文件。

```text
Web 展开目录 / 点击文件
          ↓
    project.Module
          ↓  相对路径 + 根目录边界检查
    本地 ProjectRoot 文件系统
          ↓
 FileEntry[] / FileContent
```
