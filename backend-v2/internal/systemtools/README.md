# systemtools

`systemtools` 只拥有不依赖用户身份和数据库的系统能力。

```text
currentTimeTool ──► systemtools.Tools ──► tools.Registry
```

需要用户数据的工具必须回到对应功能模块，不能放进这里。
