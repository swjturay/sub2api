import type { DepartmentAnalytics } from "../lib/types";

export interface DepartmentMetricSpec {
  label: string;
  value: number | null;
  unit?: string;
  detail?: string;
  formula?: string;
}

export function departmentMetricGroups(data: DepartmentAnalytics): {
  title: string;
  primary: DepartmentMetricSpec;
  secondary: DepartmentMetricSpec[];
}[] {
  return [{
    title: "成员与使用",
    primary: { label: "总成员数", value: data.members, formula: "当前所选部门成员数，不随日期或模型变化" },
    secondary: [
      { label: "活跃成员数", value: data.activeMembers },
      { label: "请求次数", value: data.requests },
    ],
  }, {
    title: "Token规模",
    primary: { label: "总Token", value: data.totalTokens },
    secondary: [
      { label: "日均Token", value: data.averageDailyTokens },
      {
        label: "每日人均Token",
        value: data.perMemberDailyTokens,
        formula: "总Token ÷ 自然日数 ÷ 总成员数",
      },
    ],
  }, {
    title: "消耗结构",
    primary: { label: "输出Token", value: data.outputTokens },
    secondary: [
      {
        label: "输出Token占比",
        value: data.outputShare === null ? null : data.outputShare * 100,
        unit: "%",
      },
      {
        label: "缓存命中率",
        value: data.cacheHitRate === null ? null : data.cacheHitRate * 100,
        unit: "%",
      },
    ],
  }];
}
