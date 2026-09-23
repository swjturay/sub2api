import { BarChart3, Building2, ChevronDown, ExternalLink, LogOut, Moon, Network, RefreshCw, Sun, UserRound } from "lucide-react";
import { NavLink } from "react-router-dom";
import { useEffect, useState, type ReactNode } from "react";
import type { User } from "../../lib/types";
import { Button } from "../ui/Button";
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from "../ui/DropdownMenu";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "../ui/Tooltip";
import { logout } from "../../lib/auth";
import { cn } from "../../lib/cn";
import { REFRESH_INTERVAL_SECONDS } from "../../lib/config";
import { dateTime } from "../../lib/format";
import { useBootstrap } from "../../features/BootstrapContext";

const links = [
  { to: "/personal", label: "个人数据", icon: UserRound, admin: false },
  { to: "/models", label: "模型广场", icon: BarChart3, admin: false },
  { to: "/departments", label: "部门数据", icon: Building2, admin: true },
  { to: "/gateway", label: "网关数据", icon: Network, admin: true },
];

function ModuleTabs({ admin }: { admin: boolean }) {
  return (
    <nav className="module-tabs" aria-label="分析模块">
      {links.filter((link) => !link.admin || admin).map((link) => (
        <NavLink key={link.to} to={link.to} className={({ isActive }) => cn("module-tab", isActive && "module-tab--active")}>
          <link.icon aria-hidden="true" />
          <span>{link.label}</span>
        </NavLink>
      ))}
    </nav>
  );
}

export function Shell({ user, children, autoRefresh, setAutoRefresh, updatedAt, onRefresh, refreshing }: { user: User; children: ReactNode; autoRefresh: boolean; setAutoRefresh: (value: boolean) => void; updatedAt: Date | null; onRefresh: () => void; refreshing: boolean }) {
  const [dark, setDark] = useState(() => localStorage.getItem("theme") === "dark");
  const admin = Boolean(user.is_admin || user.role === "admin");
  const dashboardUrl = admin ? "/admin/dashboard" : "/dashboard";
  const { timezone } = useBootstrap();
  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    localStorage.setItem("theme", dark ? "dark" : "light");
    window.dispatchEvent(new Event("insights-theme-change"));
  }, [dark]);
  const identity = user.username || user.email || "当前用户";
  const initial = identity.slice(0, 1).toUpperCase();
  return (
    <TooltipProvider delayDuration={250}>
      <div className="min-h-screen">
        <a href="#insights-main" className="skip-link">跳到主要内容</a>
        <header className="app-header">
          <div className="app-header__inner">
            <a href={dashboardUrl} className="brand-mark" aria-label="AI基础设施看板">
              <span className="brand-mark__icon"><BarChart3 aria-hidden="true" /></span>
              <span className="brand-mark__name">AI基础设施看板</span>
            </a>
            <ModuleTabs admin={admin} />
            <div className="header-actions">
              <Tooltip>
                <TooltipTrigger asChild><Button type="button" variant="ghost" className="icon-button" onClick={onRefresh} disabled={refreshing} aria-label="刷新数据"><RefreshCw className={cn(refreshing && "animate-spin")} /></Button></TooltipTrigger>
                <TooltipContent>{updatedAt ? `上次更新 ${dateTime(updatedAt, timezone)}` : "刷新当前数据"}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild><Button type="button" variant="ghost" className="icon-button" onClick={() => setDark(!dark)} aria-label={dark ? "切换为浅色模式" : "切换为深色模式"}>{dark ? <Sun /> : <Moon />}</Button></TooltipTrigger>
                <TooltipContent>{dark ? "浅色模式" : "深色模式"}</TooltipContent>
              </Tooltip>
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger asChild>
                  <Button type="button" variant="ghost" className="account-menu-trigger" aria-label="用户菜单">
                    <span className="account-avatar">{initial}</span>
                    <span className="account-name">{identity}</span>
                    <ChevronDown aria-hidden="true" />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-72">
                  <DropdownMenuLabel><span className="block truncate text-sm text-[var(--ink)]">{identity}</span><span className="mt-0.5 block font-normal">{admin ? "管理员" : "用户"}</span></DropdownMenuLabel>
                  <DropdownMenuSeparator />
                  <DropdownMenuCheckboxItem checked={autoRefresh} onCheckedChange={(checked) => setAutoRefresh(checked === true)} onSelect={(event) => event.preventDefault()}>{REFRESH_INTERVAL_SECONDS} 秒自动刷新</DropdownMenuCheckboxItem>
                  <p className="m-0 px-2.5 pb-2 pt-1 text-xs text-[var(--muted)] tabular-nums">{updatedAt ? `更新于 ${dateTime(updatedAt, timezone)}` : "尚未刷新"}</p>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem asChild><a href={dashboardUrl}><ExternalLink aria-hidden="true" />返回 Sub2API</a></DropdownMenuItem>
                  <DropdownMenuItem className="text-[var(--danger)]" onSelect={() => void logout()}><LogOut aria-hidden="true" />退出登录</DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </div>
          </div>
        </header>
        <main id="insights-main" className="app-main">{children}</main>
      </div>
    </TooltipProvider>
  );
}
