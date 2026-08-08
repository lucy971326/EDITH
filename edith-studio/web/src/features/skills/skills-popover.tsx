"use client";

import { Fragment, useEffect, useMemo, useRef, useState } from "react";
import type { SkillEntry, SkillLevel } from "../../api/skills";

const LEVEL_ORDER: SkillLevel[] = ["system", "user", "project"];

const LEVEL_LABELS: Record<SkillLevel, string> = {
  system: "系统级",
  user: "用户级",
  project: "项目级",
};

type SkillsPopoverProps = {
  skills: SkillEntry[];
  onSelect: (name: string) => void;
  onClose: () => void;
};

// SkillsPopover 是侧栏的 Skills 弹层：按系统/用户/项目分组累积展示，
// 支持上下键移动高亮、ENTER 选中技能名、ESC 关闭。
export function SkillsPopover({ skills, onSelect, onClose }: SkillsPopoverProps) {
  // items 是按层级分组排序的扁平列表，键盘高亮基于它（header 不作为可选项）。
  const items = useMemo(() => {
    const list: SkillEntry[] = [];
    for (const level of LEVEL_ORDER) {
      const group = skills
        .filter((entry) => entry.level === level)
        .sort((a, b) => a.name.localeCompare(b.name));
      list.push(...group);
    }
    return list;
  }, [skills]);

  const [activeIndex, setActiveIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const activeRef = useRef<HTMLButtonElement>(null);
  // 技能列表可能刷新变短，渲染和选择一律用 clamp 后的安全索引，不直接改写 state。
  const safeActiveIndex = items.length === 0 ? -1 : Math.min(activeIndex, items.length - 1);

  useEffect(() => {
    containerRef.current?.focus();
  }, []);

  // 高亮项进入视野。
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: "nearest" });
  }, [safeActiveIndex]);

  function handleKeyDown(event: React.KeyboardEvent<HTMLDivElement>) {
    if (items.length === 0) {
      return;
    }
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((index) => Math.min(index + 1, items.length - 1));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((index) => Math.max(index - 1, 0));
    } else if (event.key === "Enter") {
      event.preventDefault();
      const target = items[safeActiveIndex];
      if (target) {
        onSelect(target.name);
      }
    } else if (event.key === "Escape") {
      event.preventDefault();
      onClose();
    }
  }

  return (
    <div className="skills-popover" ref={containerRef} onKeyDown={handleKeyDown} role="listbox" tabIndex={-1}>
      {items.length === 0 ? (
        <p className="empty-skills">暂无技能，可在 ~/.edith/skills 或项目 .edith/skills 添加。</p>
      ) : (
        items.map((entry, index) => {
          const header = index === 0 || items[index - 1].level !== entry.level;
          return (
            <Fragment key={`${entry.level}:${entry.name}`}>
              {header && <div className="skills-group-label">{LEVEL_LABELS[entry.level]}</div>}
              <button
                aria-selected={index === safeActiveIndex}
                className={`skills-item ${index === safeActiveIndex ? "active" : ""}`}
                onClick={() => onSelect(entry.name)}
                ref={index === safeActiveIndex ? activeRef : undefined}
                role="option"
                type="button"
              >
                <b>{entry.name}</b>
                <span>{entry.description}</span>
              </button>
            </Fragment>
          );
        })
      )}
    </div>
  );
}
