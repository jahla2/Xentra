import {NavLink,Outlet,useLocation} from 'react-router-dom';
import {useWorkspace} from '../app/WorkspaceProvider';

const navigation=[
 {to:'/',label:'Overview',end:true},
 {to:'/projects',label:'Projects'},
 {to:'/environments',label:'Environments'},
 {to:'/incidents',label:'Incidents'},
 {to:'/ask',label:'Ask AI'},
 {to:'/integrations',label:'Integrations'},
 {to:'/approvals',label:'Approvals'},
 {to:'/audit',label:'Audit'},
 {to:'/settings',label:'Settings'},
];

const titles:Record<string,string>={
 '/':'Infrastructure overview',
 '/projects':'Projects',
 '/environments':'Environments',
 '/incidents':'Incidents',
 '/ask':'Ask Xentra',
 '/integrations':'Integrations',
 '/approvals':'Approvals',
 '/audit':'Audit trail',
 '/settings':'Settings',
};

export function AppShell(){
 const{principal,error,clearError,logout}=useWorkspace();
 const location=useLocation();
 if(!principal)return null;

 return <div className="shell">
  <aside className="sidebar">
   <div className="brand"><span>X</span>Xentra</div>
   <nav>{navigation.map(item=>
    <NavLink
     key={item.to}
     to={item.to}
     end={item.end}
     className={({isActive})=>isActive?'nav active':'nav'}
    >{item.label}</NavLink>
   )}</nav>
   <p className="muted">{principal.organizationName}<br/>{principal.email}<br/>{principal.role}</p>
   <button className="nav" onClick={()=>void logout()}>Sign out</button>
  </aside>
  <main className="main">
   <header>
    <div><p className="eyebrow">DEVOPS COMMAND CENTER</p><h1>{titles[location.pathname]??'Xentra'}</h1></div>
    <span className="status">● {principal.organizationName}</span>
   </header>
   {error&&<button className="error error-button" onClick={clearError}>{error}</button>}
   <Outlet/>
  </main>
 </div>;
}
