# Go 架构心智模型

设计一个模块前，先不急着拆文件或写 `NewXxx`。先问：**这个长期存在的对象，应该拥有什么能力、状态与身份？**

```text
结构体字段
├── 长期能力：Runner、Sandbox、数据库、HTTP Client
├── 长期状态：缓存、Map、Mutex、配置
└── 稳定身份：App ID、基础 Prompt、目录路径

函数参数
└── 本次调用才有的输入：ctx、userID、sessionID、消息、请求选项
```