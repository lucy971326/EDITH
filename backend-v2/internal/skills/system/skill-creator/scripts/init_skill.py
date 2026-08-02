#!/usr/bin/env python3
"""在用户 Skills 目录中创建一个 EDITH Skill 骨架。"""

import argparse
import json
import re
import sys
from pathlib import Path


MAX_NAME_LENGTH = 64
ALLOWED_RESOURCES = {"scripts", "references", "assets"}


def normalize_name(value):
    """把用户输入转换为小写连字符格式。"""
    value = re.sub(r"[^a-zA-Z0-9]+", "-", value.strip().lower()).strip("-")
    return re.sub(r"-{2,}", "-", value)


def parse_resources(value):
    """解析并校验可选资源目录。"""
    if not value:
        return []
    resources = list(dict.fromkeys(item.strip() for item in value.split(",") if item.strip()))
    invalid = sorted(set(resources) - ALLOWED_RESOURCES)
    if invalid:
        raise ValueError(f"不支持的资源目录: {', '.join(invalid)}")
    return resources


def quoted(value):
    """生成安全的 YAML 字符串。"""
    return json.dumps(value, ensure_ascii=False)


def create_skill(name, root, resources, display_name, short_description):
    """创建 Skill 文件和资源目录。"""
    skill_path = Path(root) / name
    if skill_path.exists():
        raise ValueError(f"Skill 目录已存在: {skill_path}")
    skill_path.mkdir(parents=True)

    title = " ".join(part.capitalize() for part in name.split("-"))
    description = short_description or f"用于 {title} 相关任务的工作方法。"
    skill_md = (
        "---\n"
        f"name: {name}\n"
        f"description: {description}\n"
        "---\n\n"
        f"# {title}\n\n"
        "## 执行规则\n\n"
        "在这里写清楚 Agent 执行这类任务时必须遵守的规则、步骤和边界。\n"
    )
    (skill_path / "SKILL.md").write_text(skill_md, encoding="utf-8")

    metadata = display_name or title
    (skill_path / "edith.yaml").write_text(
        f"display_name: {quoted(metadata)}\n"
        f"short_description: {quoted(description)}\n",
        encoding="utf-8",
    )
    for resource in resources:
        (skill_path / resource).mkdir()
    return skill_path


def main():
    parser = argparse.ArgumentParser(description="创建 EDITH 用户 Skill 骨架")
    parser.add_argument("skill_name", help="Skill 名称，会规范化为小写连字符格式")
    parser.add_argument(
        "--path",
        default="/home/user/skills/custom",
        help="用户 Skill 根目录，默认 /home/user/skills/custom",
    )
    parser.add_argument("--resources", default="", help="可选目录: scripts,references,assets")
    parser.add_argument("--display-name", default="", help="edith.yaml 中的展示名称")
    parser.add_argument("--short-description", default="", help="edith.yaml 中的展示说明")
    args = parser.parse_args()

    name = normalize_name(args.skill_name)
    if not name:
        print("错误: Skill 名称不能为空", file=sys.stderr)
        return 1
    if len(name) > MAX_NAME_LENGTH:
        print(f"错误: Skill 名称不能超过 {MAX_NAME_LENGTH} 个字符", file=sys.stderr)
        return 1
    try:
        resources = parse_resources(args.resources)
        skill_path = create_skill(
            name,
            args.path,
            resources,
            args.display_name,
            args.short_description,
        )
    except (OSError, ValueError) as error:
        print(f"错误: {error}", file=sys.stderr)
        return 1

    print(f"已创建 EDITH Skill: {skill_path}")
    print("下一步: 编辑 SKILL.md，完成后运行 quick_validate.py 校验格式。")
    return 0


if __name__ == "__main__":
    sys.exit(main())
