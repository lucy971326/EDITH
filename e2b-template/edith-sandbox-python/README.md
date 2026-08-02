# edith-sandbox-python - E2B Sandbox Template

这是一个 E2B sandbox template，允许你在受控环境中运行代码。

## 前置条件

开始之前，请确保你具备：
- 一个 E2B 账号（在 [e2b.dev](https://e2b.dev) 注册）
- 你的 E2B API key（从 [E2B dashboard](https://e2b.dev/dashboard) 获取）
- 已安装 Python

## 配置

1. 在项目根目录创建 `.env` 文件，或设置环境变量：
   ```
   E2B_API_KEY=your_api_key_here
   ```

## 安装依赖

```bash
pip install e2b
```

## 构建 Template

```bash
# 开发环境
make e2b:build:dev

# 生产环境
make e2b:build:prod
```

## 在 Sandbox 中使用 Template

Template 构建完成后，即可在你的 E2B sandbox 中使用：

```python
from e2b import Sandbox

# 创建一个新的 sandbox 实例
sandbox = Sandbox.create('edith-sandbox-python')

# 你的 sandbox 已就绪！
print('Sandbox created successfully')
```

## Template 结构

- `template.py` - 定义 sandbox template 配置
- `build_dev.py` - 构建开发环境 template
- `build_prod.py` - 构建生产环境 template

Template 创建的 Sandbox 会预置：

```text
/home/user/
├─ uploads/
├─ work/
└─ artifacts/

/home/user/skills/
├─ system/    ← 从 backend-v2/internal/skills/system 复制的公共 Skills（root 所有）
└─ custom/    ← 用户级 Skills Volume 的挂载点
```

`EDITH_E2B_TEMPLATE` 同时控制 `.env` 中的生产版本和开发版本前缀。当前版本为 `edith-v0-1-5`，开发版本会构建为 `edith-v0-1-5-dev`。

## 下一步

1. 在 `template.py` 中按需定制 template
2. 使用上述方法之一构建 template
3. 在你的 E2B sandbox 代码中使用该 template
4. 查阅 [E2B 文档](https://e2b.dev/docs) 了解更多高级用法
