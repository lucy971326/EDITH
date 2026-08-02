#!/usr/bin/env python3
"""根据用户 Skills 文件生成 overview.md。"""

import argparse
from pathlib import Path

from quick_validate import FRONTMATTER_KEYS, parse_simple_yaml, split_frontmatter, validate_skill


def skill_summary(skill_path):
    """读取并校验一个 Skill 的 name 和 description。"""
    valid, message = validate_skill(skill_path)
    if not valid:
        raise ValueError(f"{skill_path.name}: {message}")

    content = (skill_path / "SKILL.md").read_text(encoding="utf-8-sig")
    frontmatter_lines, _, error = split_frontmatter(content)
    if error:
        raise ValueError(f"{skill_path.name}: {error}")
    frontmatter, error = parse_simple_yaml(frontmatter_lines, FRONTMATTER_KEYS)
    if error:
        raise ValueError(f"{skill_path.name}: {error}")
    return frontmatter["name"], frontmatter["description"]


def build_overview(root):
    """扫描 custom 目录并生成 overview.md 内容。"""
    skills = []
    for skill_path in sorted(root.iterdir(), key=lambda path: path.name):
        if not skill_path.is_dir() or skill_path.name.startswith("."):
            continue
        skills.append(skill_summary(skill_path))

    lines = [
        "# 用户 Skills 总览",
        "",
        "以下内容由脚本生成，请勿手动修改。",
        "",
    ]
    if not skills:
        lines.append("当前没有用户 Skills。")
    else:
        for name, description in sorted(skills, key=lambda item: item[0]):
            lines.append(f"- `{name}`：{description}")
            lines.append(f"  - 路径：`skills/custom/{name}/SKILL.md`")
    return "\n".join(lines) + "\n"


def main():
    parser = argparse.ArgumentParser(description="生成 EDITH 用户 Skills 总览")
    parser.add_argument(
        "--path",
        default="/home/user/skills/custom",
        help="用户 Skill 根目录，默认 /home/user/skills/custom",
    )
    args = parser.parse_args()

    root = Path(args.path)
    if not root.is_dir():
        print(f"错误: 用户 Skills 目录不存在: {root}")
        return 1

    try:
        overview = build_overview(root)
        (root / "overview.md").write_text(overview, encoding="utf-8")
    except (OSError, ValueError) as error:
        print(f"错误: {error}")
        return 1

    print(f"已生成用户 Skills 总览: {root / 'overview.md'}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
