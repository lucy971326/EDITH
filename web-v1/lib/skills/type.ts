// Skills 列表的浏览器契约；页面只接收名称和摘要。

export type SkillListItem = {
  name: string;
  description: string;
};

export type SkillsResponse = {
  system: SkillListItem[];
  custom: SkillListItem[];
};
