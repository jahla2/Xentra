import {BrowserRouter,Navigate,Route,Routes} from 'react-router-dom';
import {WorkspaceProvider,useWorkspace} from './app/WorkspaceProvider';
import {AppShell} from './components/AppShell';
import {ApprovalsPage} from './pages/ApprovalsPage';
import {AskPage} from './pages/AskPage';
import {AuditPage} from './pages/AuditPage';
import {DashboardPage} from './pages/DashboardPage';
import {EnvironmentDetailPage} from './pages/EnvironmentDetailPage';
import {EnvironmentsPage} from './pages/EnvironmentsPage';
import {IncidentsPage} from './pages/IncidentsPage';
import {IntegrationsPage} from './pages/IntegrationsPage';
import {LiveExecutionPage} from './pages/LiveExecutionPage';
import {LoginPage} from './pages/LoginPage';
import {ProjectsPage} from './pages/ProjectsPage';
import {SettingsPage} from './pages/SettingsPage';

function RoutedApp(){
 const{authReady,principal}=useWorkspace();

 if(!authReady){
  return <main className="main"><section className="panel"><p>Loading Xentra…</p></section></main>;
 }
 if(!principal)return <LoginPage/>;

 return <Routes>
  <Route element={<AppShell/>}>
   <Route index element={<DashboardPage/>}/>
   <Route path="projects" element={<ProjectsPage/>}/>
   <Route path="environments" element={<EnvironmentsPage/>}/>
   <Route path="environments/:environmentId" element={<EnvironmentDetailPage/>}/>
   <Route path="incidents" element={<IncidentsPage/>}/>
   <Route path="ask" element={<AskPage/>}/>
   <Route path="integrations" element={<IntegrationsPage/>}/>
   <Route path="approvals" element={<ApprovalsPage/>}/>
   <Route path="executions/:actionId" element={<LiveExecutionPage/>}/>
   <Route path="audit" element={<AuditPage/>}/>
   <Route path="settings" element={<SettingsPage/>}/>
   <Route path="*" element={<Navigate to="/" replace/>}/>
  </Route>
 </Routes>;
}

export function App(){
 return <BrowserRouter>
  <WorkspaceProvider>
   <RoutedApp/>
  </WorkspaceProvider>
 </BrowserRouter>;
}
