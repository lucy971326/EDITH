# session

创建 `%USERPROFILE%/.edith/sessions.db` 的框架 SQLite SessionService。

会话的读取、列表和删除以后也只经由该 Service，不直接查询数据库。
