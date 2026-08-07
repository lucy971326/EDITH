# models

`models.Module` 是用户级模型目录，也是模型能力的后端唯一真相来源。

```text
~/.edith/models.yaml
        ↓ Load / Build
models.Module
  ├─ AgentModels()  → Workspace 组装 LLMAgent
  ├─ Catalog()      → Studio / Web 获取 Profile
  ├─ RunOptions()   → Engine 获取本次 Run 的框架选项
  └─ SummaryModel() → Session 摘要使用当前选择的模型
```

## 对外能力

- `Load`：读取用户配置并在启动期创建全部模型实例。
- `Build`：从已解析配置创建模块，便于测试和明确组装边界。
- `AgentModels`：返回已创建的 `model.Model` 实例集合。
- `DefaultModel`：返回后端配置的默认模型实例。
- `Catalog`：返回不含 API Key 的模型 Profile。
- `RunOptions`：校验模型和思考模式，并生成框架 `RunOption`。
- `SummaryModel`：复用已注册模型实例，提供本次摘要所需的模型能力。
- `SupportsVision`：按模型声明判断是否允许图片输入。

## 配置边界

- API Key、Base URL 只在后端使用，不通过 Catalog 返回。
- Context、Vision、Thinking 是模型级能力，由配置声明并由后端校验。
- Provider Variant 只负责供应商协议格式，不代表所有同厂模型能力相同。
- 模型实例启动时创建；运行期间只切换已注册实例。
- 摘要模型也从已注册实例中选择，不为每次 `/compact` 新建供应商连接。

## 思考参数翻译

`RunOptions` 只接收产品级 `ThinkingMode`，供应商字段由模块统一转换：

| Provider | 请求字段 |
| --- | --- |
| DeepSeek / GLM | `thinking.type` +（开启时）`reasoning_effort` |
| MiniMax M3 | `thinking.type` + `reasoning_split: true` |
| MiMo | `thinking.type` |
| Kimi K3 | `reasoning_effort` |

框架 `v1.11.0` 的 OpenAI Variant 负责协议差异；本模块只补充本次 Run 的思考选择和模型 Profile。
