#!/usr/bin/env python3
"""校验一个 EDITH Skill 的目录格式。"""

import re
import sys
from pathlib import Path


MAX_NAME_LENGTH = 64
MAX_DESCRIPTION_LENGTH = 1024
FRONTMATTER_KEYS = {"name", "description"}
EDITH_KEYS = {"display_name", "short_description"}


def parse_simple_yaml(lines, allowed_keys):
    """解析 Skill 元数据需要的简单 YAML 键值。"""
    values = {}
    current_key = None
    for raw_line in lines:
        line = raw_line.rstrip()
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        match = re.match(r"^([A-Za-z][A-Za-z0-9_-]*):(?:\s*(.*))?$", line)
        if match:
            key, value = match.group(1), match.group(2) or ""
            if key not in allowed_keys:
                return None, f"不支持的元数据字段: {key}"
            values[key] = value.strip().strip("'\"")
            current_key = key
            continue
        if current_key and line.startswith((" ", "\t")):
            values[current_key] = (values.get(current_key, "") + " " + line.strip()).strip()
            continue
        return None, f"无法解析元数据行: {raw_line}"
    return values, None


def split_frontmatter(content):
    """分离 SKILL.md 的 YAML 头部。"""
    lines = content.splitlines()
    if not lines or lines[0].strip() != "---":
        return None, None, "SKILL.md 必须以 YAML frontmatter 开始"
    for index in range(1, len(lines)):
        if lines[index].strip() == "---":
            return lines[1:index], lines[index + 1 :], None
    return None, None, "YAML frontmatter 没有结束标记"


def validate_skill(skill_path):
    """校验 Skill 目录，返回 (是否通过, 中文消息)。"""
    skill_path = Path(skill_path)
    skill_md = skill_path / "SKILL.md"
    if not skill_md.is_file():
        return False, "缺少 SKILL.md"

    frontmatter_lines, _, error = split_frontmatter(skill_md.read_text(encoding="utf-8-sig"))
    if error:
        return False, error
    frontmatter, error = parse_simple_yaml(frontmatter_lines, FRONTMATTER_KEYS)
    if error:
        return False, f"SKILL.md 元数据错误: {error}"
    if not frontmatter.get("name"):
        return False, "SKILL.md 缺少 name"
    if not frontmatter.get("description"):
        return False, "SKILL.md 缺少 description"

    name = frontmatter["name"]
    if not re.fullmatch(r"[a-z0-9]+(?:-[a-z0-9]+)*", name):
        return False, "name 只能使用小写字母、数字和单个连字符"
    if len(name) > MAX_NAME_LENGTH:
        return False, f"name 不能超过 {MAX_NAME_LENGTH} 个字符"
    if skill_path.name != name:
        return False, f"目录名 {skill_path.name!r} 必须与 name {name!r} 一致"

    description = frontmatter["description"]
    if len(description) > MAX_DESCRIPTION_LENGTH:
        return False, f"description 不能超过 {MAX_DESCRIPTION_LENGTH} 个字符"
    if "<" in description or ">" in description:
        return False, "description 不能包含 < 或 >"

    edith_path = skill_path / "edith.yaml"
    if edith_path.is_file():
        edith, error = parse_simple_yaml(
            edith_path.read_text(encoding="utf-8-sig").splitlines(), EDITH_KEYS
        )
        if error:
            return False, f"edith.yaml 元数据错误: {error}"
        if not any(edith.get(key) for key in EDITH_KEYS):
            return False, "edith.yaml 至少需要 display_name 或 short_description"

    return True, "Skill 格式正确"


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("用法: python quick_validate.py <skill目录>")
        sys.exit(1)
    valid, message = validate_skill(sys.argv[1])
    print(message)
    sys.exit(0 if valid else 1)
