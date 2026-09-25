import { useEffect, useRef, useState } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { AuthSessionChangedError, sessionSnapshot, verifySession } from "./lib/auth";
import type { User } from "./lib/types";
import { userDataScope } from "./lib/dataScope";
import { BootstrapProvider } from "./features/BootstrapContext";
import { Shell } from "./components/layout/Shell";
import { Loading } from "./components/ui/States";
import { PersonalPage } from "./pages/PersonalPage";
import { ModelsPage } from "./pages/ModelsPage";
import { DepartmentPage } from "./pages/DepartmentPage";
import { GatewayPage } from "./pages/GatewayPage";
import { CostPage } from "./pages/CostPage";
export default function App(){
 const [user,setUser]=useState<User|null>(null),[error,setError]=useState(""),[auto,setAuto]=useState(true),[updated,setUpdated]=useState<Date|null>(null),[refreshing,setRefreshing]=useState(false); const generation=useRef(0),location=useLocation();
 useEffect(()=>{let active=true;const mountedGeneration=generation;const verify=()=>{const current=++generation.current,snapshot=sessionSnapshot();void verifySession(snapshot).then(u=>{if(active&&current===generation.current){setUser(u);setError("")}}).catch(e=>{if(e instanceof AuthSessionChangedError){if(active&&current===generation.current)verify();return}if(active&&current===generation.current){setError((e as Error).message);window.location.assign("/login?redirect="+encodeURIComponent("/insights"+location.pathname+location.search))}})};verify();const storage=(e:StorageEvent)=>{if(["auth_token","refresh_token"].includes(e.key||""))verify()},revalidate=()=>verify();addEventListener("storage",storage);addEventListener("insights-auth-revalidate",revalidate);return()=>{active=false;mountedGeneration.current++;removeEventListener("storage",storage);removeEventListener("insights-auth-revalidate",revalidate)}},[location.pathname,location.search]);
 useEffect(()=>{const success=(e:Event)=>{setUpdated((e as CustomEvent<{at:Date}>).detail.at);setRefreshing(false)},settled=()=>setRefreshing(false);addEventListener("insights-refresh-success",success);addEventListener("insights-refresh-settled",settled);return()=>{removeEventListener("insights-refresh-success",success);removeEventListener("insights-refresh-settled",settled)}},[]);
 useEffect(()=>{setUpdated(null);setRefreshing(false)},[user?.id,user?.role,user?.is_admin]);
 if(!user)return <main className="p-6"><Loading label={error||"正在验证会话"}/></main>;
 const admin=Boolean(user.is_admin||user.role==="admin"),dataScope=userDataScope(user);
 return <BootstrapProvider key={dataScope}><Shell user={user} autoRefresh={auto} setAutoRefresh={setAuto} updatedAt={updated} onRefresh={()=>{setRefreshing(true);window.dispatchEvent(new Event("insights-manual-refresh"))}} refreshing={refreshing}><Routes><Route index element={<Navigate to={admin?"/departments":"/personal"} replace/>}/><Route path="/personal" element={<PersonalPage auto={auto}/>}/><Route path="/models" element={<ModelsPage auto={auto}/>}/><Route path="/departments" element={admin?<DepartmentPage auto={auto}/>:<Navigate to="/personal" replace/>}/><Route path="/gateway" element={admin?<GatewayPage auto={auto}/>:<Navigate to="/personal" replace/>}/><Route path="/costs" element={admin?<CostPage auto={auto}/>:<Navigate to="/personal" replace/>}/><Route path="*" element={<Navigate to={admin?"/departments":"/personal"} replace/>}/></Routes></Shell></BootstrapProvider>
}
