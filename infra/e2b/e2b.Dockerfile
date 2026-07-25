# EDITH 0.1.0：基于 E2B base，预装 EDITH 的系统 Skills。
FROM e2bdev/base

# 系统 Skills 是 Template 的只读内容，不属于后端运行时代码。
USER root
COPY backend/skills/system/ /home/user/skills/system/
RUN chown -R root:root /home/user/skills/system \
    && chmod -R a-w /home/user/skills/system

USER user
WORKDIR /home/user
