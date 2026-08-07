# Prompt Log

记录框架在 `BeforeModel` 阶段准备好的模型请求，供本地调试 Prompt 使用。

## 作用

```text
Runner
  → PromptLogPlugin.BeforeModel
  → 读取 model.Request
  → 写入 ~/.edith/logs/llm-prompts.log
  → 放行模型调用
```

日志包含：

- 消息历史；
- 生成参数和供应商扩展字段；
- 当前模型与会话；
- 本次请求的完整工具定义。

插件只观察，不修改请求，也不阻断模型调用。
