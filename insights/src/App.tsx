import { useEffect, useState } from "react";
import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useInsightsSession } from "./lib/useInsightsSession";
import { userDataScope, canViewInsights } from "./lib/dataScope";
import { BootstrapProvider } from "./features/BootstrapContext";
import { Shell } from "./components/layout/Shell";
import { Loading } from "./components/ui/States";
import { PersonalPage } from "./pages/PersonalPage";
import { ModelsPage } from "./pages/ModelsPage";
import { DepartmentPage } from "./pages/DepartmentPage";
import { GatewayPage } from "./pages/GatewayPage";
import { CostPage } from "./pages/CostPage";
export default function App(){
 const [auto,setAuto]=useState(true),[updated,setUpdated]=useState<Date|null>(null),[refreshing,setRefreshing]=useState(false),location=useLocation();
 const {user,error,checking}=useInsightsSession(location.pathname+location.search);
 useEffect(()=>{const success=(e:Event)=>{setUpdated((e as CustomEvent<{at:Date}>).detail.at);setRefreshing(false)},settled=()=>setRefreshing(false);addEventListener("insights-refresh-success",success);addEventListener("insights-refresh-settled",settled);return()=>{removeEventListener("insights-refresh-success",success);removeEventListener("insights-refresh-settled",settled)}},[]);
 useEffect(()=>{setUpdated(null);setRefreshing(false)},[user?.id,user?.role,user?.is_admin,user?.can_view_insights]);
 if(!user||checking)return <main className="p-6"><Loading label={error||"正在验证会话"}/></main>;
 const admin=Boolean(user.is_admin||user.role==="admin"),canView=canViewInsights(user),dataScope=userDataScope(user);
 return <BootstrapProvider key={dataScope}><Shell user={user} autoRefresh={auto} setAutoRefresh={setAuto} updatedAt={updated} onRefresh={()=>{setRefreshing(true);window.dispatchEvent(new Event("insights-manual-refresh"))}} refreshing={refreshing}><Routes><Route index element={<Navigate to={canView?"/departments":"/personal"} replace/>}/><Route path="/personal" element={<PersonalPage auto={auto}/>}/><Route path="/models" element={<ModelsPage auto={auto}/>}/><Route path="/departments" element={canView?<DepartmentPage auto={auto}/>:<Navigate to="/personal" replace/>}/><Route path="/gateway" element={canView?<GatewayPage auto={auto}/>:<Navigate to="/personal" replace/>}/><Route path="/costs" element={canView?<CostPage auto={auto} canEdit={admin}/>:<Navigate to="/personal" replace/>}/><Route path="*" element={<Navigate to={canView?"/departments":"/personal"} replace/>}/></Routes></Shell></BootstrapProvider>
}
