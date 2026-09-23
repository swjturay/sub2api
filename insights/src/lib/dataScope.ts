import type { User } from "./types";
export const userDataScope = (user: User) => `${user.id}:${user.role || ""}:${user.is_admin ? 1 : 0}`;
