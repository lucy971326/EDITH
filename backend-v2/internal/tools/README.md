# tools

`tools` 只汇总工具，不实现任何功能工具。

```text
sandbox.Tools ──┐
cronjob.Tools ──┼──► tools.Registry ──► llmagent
未来模块.Tools ─——┘
```

工具的业务代码和数据依赖始终留在所属功能模块。
