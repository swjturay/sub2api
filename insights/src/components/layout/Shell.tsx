import { BarChart3, Building2, ChevronLeft, LogOut, Moon, Network, RefreshCw, Sun, UserRound } from "lucide-react";
import { NavLink, useLocation } from "react-router-dom";
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import type { User } from "../../lib/types";
import { Button } from "../ui/Button";
import { Toggle } from "../ui/Controls";
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
  const location = useLocation(), navRef = useRef<HTMLElement>(null), [indicator, setIndicator] = useState({ left: 0, width: 0, visible: false });
  const visibleLinks = useMemo(() => links.filter((link) => !link.admin || admin), [admin]);
  useLayoutEffect(() => {
    const nav = navRef.current, item = nav?.querySelector<HTMLAnchorElement>(".module-tab--active");
    const update = () => { if (!nav || !item) return; setIndicator({ left: item.offsetLeft, width: item.offsetWidth, visible: true }); };
    update(); item?.scrollIntoView({ block: "nearest", inline: "nearest" });
    if (!nav || !item) return;
    const observer = new ResizeObserver(update); observer.observe(nav); observer.observe(item); return () => observer.disconnect();
  }, [location.pathname, visibleLinks]);
  return <nav ref={navRef} className="module-tabs" aria-label="分析页面">
    <span className="module-tab-indicator" aria-hidden style={{ width: indicator.width, transform: `translateX(${indicator.left}px)`, opacity: indicator.visible ? 1 : 0 }} />
    {visibleLinks.map((link) => <NavLink key={link.to} to={link.to} onFocus={(event) => event.currentTarget.scrollIntoView({ block: "nearest", inline: "nearest" })} className={({ isActive }) => cn("module-tab", isActive && "module-tab--active")}><link.icon className="h-4 w-4"/><span>{link.label}</span></NavLink>)}
  </nav>;
}
export function Shell({ user, children, autoRefresh, setAutoRefresh, updatedAt, onRefresh, refreshing }: { user: User; children: ReactNode; autoRefresh: boolean; setAutoRefresh: (v: boolean) => void; updatedAt: Date | null; onRefresh: () => void; refreshing: boolean }) {
  const [dark, setDark] = useState(() => localStorage.getItem("theme") === "dark");
  const admin = Boolean(user.is_admin || user.role === "admin");
  const { timezone } = useBootstrap();
  useEffect(() => { document.documentElement.classList.toggle("dark", dark); localStorage.setItem("theme", dark ? "dark" : "light"); }, [dark]);
  return <div className="min-h-screen">
    <header className="app-header">
      <div className="app-header__top">
        <a href={admin ? "/admin/dashboard" : "/dashboard"} className="brand-mark"><span className="brand-mark__icon"><BarChart3 className="h-4 w-4" /></span><span><strong>AI 基础设施看板</strong><small>Sub2API Insights</small></span></a>
        <div className="header-actions">
          <div className="account-chip"><span className="account-chip__avatar">{(user.username || user.email || "U").slice(0,1).toUpperCase()}</span><span className="hidden sm:block"><strong>{user.username || user.email || "当前用户"}</strong><small>{admin ? "管理员" : "用户"}</small></span></div>
          <Button variant="ghost" className="icon-button" onClick={() => setDark(v=>!v)} aria-label={dark ? "浅色模式" : "深色模式"}>{dark ? <Sun className="h-4 w-4"/> : <Moon className="h-4 w-4"/>}</Button>
          <a href={admin ? "/admin/dashboard" : "/dashboard"} className="header-link"><ChevronLeft className="h-4 w-4"/><span className="hidden md:inline">返回 Sub2API</span></a>
          <Button variant="ghost" className="icon-button text-[var(--danger)]" onClick={()=>void logout()} aria-label="退出登录"><LogOut className="h-4 w-4"/></Button>
        </div>
      </div>
      <div className="app-header__navrow"><ModuleTabs admin={admin}/><div className="refresh-cluster"><Toggle checked={autoRefresh} onChange={setAutoRefresh} label={`${REFRESH_INTERVAL_SECONDS}秒自动刷新`} /><span className="refresh-time">{updatedAt ? `更新于 ${dateTime(updatedAt, timezone)}` : "尚未刷新"}</span><Button variant="secondary" onClick={onRefresh} disabled={refreshing}><RefreshCw className={cn("h-4 w-4",refreshing&&"animate-spin")}/>刷新</Button></div></div>
    </header>
    <main className="app-main">{children}</main>
  </div>;
}
