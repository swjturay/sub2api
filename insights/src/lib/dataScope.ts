import type { User } from "./types";
export const canViewInsights = (user: User) => Boolean(user.is_admin || user.role === "admin" || user.can_view_insights);
export const userDataScope = (user: User) => `${user.id}:${user.role || ""}:${user.is_admin ? 1 : 0}:${canViewInsights(user) ? 1 : 0}`;
