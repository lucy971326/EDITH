import { apiJSON } from "./client";

export type SkillLevel = "system" | "user" | "project";

export type SkillEntry = {
  name: string;
  description: string;
  level: SkillLevel;
};

// getSkills 返回系统/用户/项目三级累积的技能列表（同名不覆盖，各标层级）。
export async function getSkills(): Promise<SkillEntry[]> {
  const result = await apiJSON<{ skills?: SkillEntry[] }>("/api/skills");
  return result.skills ?? [];
}
