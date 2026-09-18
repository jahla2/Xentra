import {FormEvent,useEffect,useState} from 'react';
import {api,ActionRequest,AuditEvent,AuthSession,CreateEnvironmentInput,Environment,GitHubIntegrationSetup,Incident,InvestigationResult,Principal,Project} from './api';

const nav=['Overview','Projects','Environments','Incidents','Ask AI','Approvals','Audit'];

export function App(){
 const[principal,setPrincipal]=useState<Principal|null>(null);
 const[authReady,setAuthReady]=useState(false);
 const[authMode,setAuthMode]=useState<'login'|'register'>('login');
 const[authForm,setAuthForm]=useState({email:'',password:'',organizationName:''});
 const[memberForm,setMemberForm]=useState({email:'',password:''});
 const[projects,setProjects]=useState<Project[]>([]);
 const[projectForm,setProjectForm]=useState({name:'Core Platform',description:''});
 const[environments,setEnvironments]=useState<Environment[]>([]);
 const[incidents,setIncidents]=useState<Incident[]>([]);
 const[audit,setAudit]=useState<AuditEvent[]>([]);
 const[selected,setSelected]=useState('');
 const[question,setQuestion]=useState('Why is the API down?');
 const[result,setResult]=useState<InvestigationResult|null>(null);
 const[latestIncident,setLatestIncident]=useState<Incident|null>(null);
 const[action,setAction]=useState<ActionRequest|null>(null);
 const[error,setError]=useState('');
 const[loading,setLoading]=useState(false);
 const[form,setForm]=useState<CreateEnvironmentInput>({projectId:'',name:'Production Ubuntu',type:'production',connectionType:'runner',runnerUrl:'http://localhost:8090',sshPort:22});
 const[github,setGitHub]=useState<{owner:string;repo:string;authMode:'github_app'|'token';accessToken:string}>({owner:'',repo:'',authMode:'github_app',accessToken:''});
 const[githubSetup,setGitHubSetup]=useState<GitHubIntegrationSetup|null>(null);
 const[remediation,setRemediation]=useState({action:'docker.restart',target:'',reason:'Recover unhealthy service'});

 const refresh=async()=>{
  try{
   const[projectItems,envItems,incidentItems,auditItems]=await Promise.all([api.listProjects(),api.listEnvironments(),api.listIncidents(),api.listAudit()]);
   setProjects(projectItems);setEnvironments(envItems);setIncidents(incidentItems);setAudit(auditItems);
   if(projectItems[0])setForm(current=>current.projectId?current:{...current,projectId:projectItems[0].id});
   if(!selected&&envItems[0])setSelected(envItems[0].id);
  }catch(err){setError((err as Error).message)}
 };

 useEffect(()=>{
  if(!api.hasToken()){setAuthReady(true);return}
  api.me().then(setPrincipal).catch(()=>api.setToken(null)).finally(()=>setAuthReady(true));
 },[]);

 useEffect(()=>{if(principal)void refresh()},[principal]);

 const update=(patch:Partial<CreateEnvironmentInput>)=>setForm(current=>({...current,...patch}));
 async function run<T>(operation:()=>Promise<T>,onSuccess:(value:T)=>void){setLoading(true);setError('');try{onSuccess(await operation());await refresh()}catch(err){setError((err as Error).message)}finally{setLoading(false)}}

 async function authenticate(event:FormEvent){
  event.preventDefault();setLoading(true);setError('');
  try{
   let session:AuthSession;
   if(authMode==='register')session=await api.register(authForm.email,authForm.password,authForm.organizationName);
   else session=await api.login(authForm.email,authForm.password);
   api.setToken(session.token);setPrincipal(session.principal);setAuthForm({email:'',password:'',organizationName:''});
  }catch(err){setError((err as Error).message)}
  finally{setLoading(false)}
 }

 async function logout(){
  try{await api.logout()}catch{/* session may already be expired */}
  api.setToken(null);setPrincipal(null);setProjects([]);setEnvironments([]);setIncidents([]);setAudit([]);setSelected('');
 }

 async function createProject(event:FormEvent){event.preventDefault();await run(()=>api.createProject(projectForm.name,projectForm.description),project=>{setProjectForm({name:'',description:''});setForm(current=>({...current,projectId:project.id}))})}
 async function addEnvironment(event:FormEvent){event.preventDefault();await run(()=>api.createEnvironment(form),env=>setSelected(env.id))}
 async function investigate(event:FormEvent){event.preventDefault();if(!selected)return;setResult(null);await run(()=>api.investigate(selected,question),setResult)}
 async function connectGitHub(event:FormEvent){event.preventDefault();if(!selected)return;setGitHubSetup(null);await run(()=>api.connectGitHub(selected,github.owner,github.repo,github.authMode,github.accessToken),setup=>{setGitHub(current=>({...current,accessToken:''}));setGitHubSetup(setup)})}
 async function createIncident(){if(!selected)return;await run(()=>api.createIncident(selected,question),incident=>{setLatestIncident(incident);setAction(null)})}
 async function proposeAction(event:FormEvent){event.preventDefault();if(!selected)return;await run(()=>api.proposeAction({incidentId:latestIncident?.id,environmentId:selected,action:remediation.action,target:remediation.target,reason:remediation.reason}),setAction)}
 async function approveAction(){if(!action)return;await run(()=>api.approveAction(action.id),setAction)}
 async function createMember(event:FormEvent){event.preventDefault();await run(()=>api.createMember(memberForm.email,memberForm.password),()=>setMemberForm({email:'',password:''}))}

 if(!authReady)return <main className="main"><section className="panel"><p>Loading Xentra…</p></section></main>;

 if(!principal)return <main className="main">
  <section className="panel" style={{maxWidth:520,margin:'64px auto'}}>
   <div className="brand"><span>X</span>Xentra</div>
   <div className="panel-title"><h2>{authMode==='login'?'Sign in':'Create workspace'}</h2><span>Secure DevOps access</span></div>
   {error&&<div className="error">{error}</div>}
   <form className="stack" onSubmit={authenticate}>
    <input type="email" value={authForm.email} onChange={e=>setAuthForm({...authForm,email:e.target.value})} placeholder="Email" required/>
    <input type="password" value={authForm.password} onChange={e=>setAuthForm({...authForm,password:e.target.value})} placeholder="Password (10+ characters)" required/>
    {authMode==='register'&&<input value={authForm.organizationName} onChange={e=>setAuthForm({...authForm,organizationName:e.target.value})} placeholder="Organization name" required/>}
    <button className="primary" disabled={loading}>{loading?'Working…':authMode==='login'?'Sign in':'Create organization'}</button>
   </form>
   <button className="nav" onClick={()=>{setAuthMode(authMode==='login'?'register':'login');setError('')}}>{authMode==='login'?'Need an account? Register':'Already have an account? Sign in'}</button>
  </section>
 </main>;

 const isOwner=principal.role==='owner';

 return <div className="shell">
  <aside className="sidebar"><div className="brand"><span>X</span>Xentra</div><nav>{nav.map((item,i)=><button key={item} className={i===0?'nav active':'nav'}>{item}</button>)}</nav><p className="muted">{principal.organizationName}<br/>{principal.email}<br/>{principal.role}</p><button className="nav" onClick={()=>void logout()}>Sign out</button></aside>
  <main className="main">
   <header><div><p className="eyebrow">DEVOPS COMMAND CENTER</p><h1>Infrastructure overview</h1></div><span className="status">● {principal.organizationName}</span></header>
   {error&&<div className="error">{error}</div>}
   <section className="grid metrics">
    <article><small>Projects</small><strong>{projects.length}</strong><span>Organization scoped</span></article>
    <article><small>Environments</small><strong>{environments.length}</strong><span>Attached to projects</span></article>
    <article><small>Incidents</small><strong>{incidents.filter(i=>i.status!=='resolved').length}</strong><span>Open / action required</span></article>
    <article><small>Audit events</small><strong>{audit.length}</strong><span>Recorded actions</span></article>
   </section>

   <section className="panel">
    <div className="panel-title"><h2>Projects</h2><span>{isOwner?'Organize environments':'Read access'}</span></div>
    {projects.map(project=><button className={`env ${form.projectId===project.id?'selected':''}`} key={project.id} onClick={()=>update({projectId:project.id})}><span className="dot"/><div><b>{project.name}</b><small>{project.description||'No description'}</small></div></button>)}
    {isOwner&&<form className="stack" onSubmit={createProject}>
     <input value={projectForm.name} onChange={e=>setProjectForm({...projectForm,name:e.target.value})} placeholder="Project name" required/>
     <input value={projectForm.description} onChange={e=>setProjectForm({...projectForm,description:e.target.value})} placeholder="Description (optional)"/>
     <button className="primary" disabled={loading}>Create project</button>
    </form>}
   </section>

   <section className="grid workbench">
    <div className="panel">
     <div className="panel-title"><h2>Environments</h2><span>{isOwner?'Owner managed':'Read access'}</span></div>
     {environments.map(env=><button className={`env ${selected===env.id?'selected':''}`} key={env.id} onClick={()=>setSelected(env.id)}><span className="dot"/><div><b>{env.name}</b><small>{env.hostname||env.sshHost||env.runnerUrl}</small><em>{env.connectionType} · {env.os} · {env.capabilities.join(' · ')||'basic'}</em></div></button>)}
     {isOwner&&<form className="stack" onSubmit={addEnvironment}>
      <select value={form.projectId} onChange={e=>update({projectId:e.target.value})} required><option value="">Select project</option>{projects.map(project=><option value={project.id} key={project.id}>{project.name}</option>)}</select>
      <input value={form.name} onChange={e=>update({name:e.target.value})} placeholder="Environment name"/>
      <select value={form.connectionType} onChange={e=>update({connectionType:e.target.value as 'runner'|'ssh'})}><option value="runner">Xentra Runner</option><option value="ssh">Ubuntu / Linux SSH</option></select>
      {form.connectionType==='runner'?<input value={form.runnerUrl??''} onChange={e=>update({runnerUrl:e.target.value})} placeholder="Runner URL"/>:<><input value={form.sshHost??''} onChange={e=>update({sshHost:e.target.value})} placeholder="Host / IP"/><input value={form.sshUser??''} onChange={e=>update({sshUser:e.target.value})} placeholder="SSH username"/><input value={form.sshHostKeyFingerprint??''} onChange={e=>update({sshHostKeyFingerprint:e.target.value})} placeholder="Host key fingerprint (SHA256:...)"/><textarea value={form.sshPrivateKey??''} onChange={e=>update({sshPrivateKey:e.target.value})} placeholder="SSH private key"/><input type="password" value={form.sshPassphrase??''} onChange={e=>update({sshPassphrase:e.target.value})} placeholder="Key passphrase (optional)"/></>}
      <button className="primary" disabled={loading}>Test & connect environment</button>
     </form>}
    </div>

    <div className="panel ai">
     <div className="panel-title"><h2>Ask Xentra</h2><span>Evidence-backed investigation</span></div>
     <form className="stack" onSubmit={investigate}>
      <select value={selected} onChange={e=>setSelected(e.target.value)}><option value="">Select environment</option>{environments.map(env=><option value={env.id} key={env.id}>{env.name}</option>)}</select>
      <textarea value={question} onChange={e=>setQuestion(e.target.value)}/>
      <button className="primary" disabled={!selected||loading}>{loading?'Working…':'Investigate'}</button>
     </form>
     {result&&<div className="finding"><div className="confidence">{result.confidence} confidence</div><h3>{result.probableRootCause}</h3><p>{result.summary}</p><b>Recommended action</b><p>{result.recommendedAction}</p><button className="primary" onClick={()=>void createIncident()} disabled={loading}>Create incident</button></div>}
    </div>
   </section>

   <section className="grid workbench">
    <div className="panel">
     <div className="panel-title"><h2>GitHub correlation</h2><span>{isOwner?'Encrypted credential':'Owner permission required'}</span></div>
     {isOwner&&<form className="stack" onSubmit={connectGitHub}><input value={github.owner} onChange={e=>setGitHub({...github,owner:e.target.value})} placeholder="GitHub owner"/><input value={github.repo} onChange={e=>setGitHub({...github,repo:e.target.value})} placeholder="Repository"/><select value={github.authMode} onChange={e=>setGitHub({...github,authMode:e.target.value as 'github_app'|'token'})}><option value="github_app">GitHub App (recommended)</option><option value="token">Personal/access token fallback</option></select>{github.authMode==='token'&&<input type="password" value={github.accessToken} onChange={e=>setGitHub({...github,accessToken:e.target.value})} placeholder="Access token"/>}<button className="primary" disabled={!selected||loading}>Connect repository</button></form>}
     {githubSetup&&<div className="finding"><div className="confidence">One-time webhook setup</div><p>Configure a GitHub webhook for <b>workflow_run</b> events using this path:</p><code>{githubSetup.webhookPath}</code><p>Webhook secret:</p><code>{githubSetup.webhookSecret}</code><p className="muted">Save this secret in GitHub now. Xentra stores only the encrypted copy and will not show it again after you dismiss this card.</p><button className="nav" onClick={()=>setGitHubSetup(null)}>I saved it</button></div>}
     {latestIncident&&<><h3>{latestIncident.rootCause}</h3><p>{latestIncident.status} · {latestIncident.confidence} confidence</p><details><summary>Timeline ({latestIncident.timeline.length})</summary>{latestIncident.timeline.map((event,i)=><p key={i}><b>{event.kind}</b> {event.summary}</p>)}</details></>}
    </div>

    <div className="panel">
     <div className="panel-title"><h2>Approved remediation</h2><span>{isOwner?'Human gate required':'Owner permission required'}</span></div>
     {isOwner&&<form className="stack" onSubmit={proposeAction}>
      <select value={remediation.action} onChange={e=>setRemediation({...remediation,action:e.target.value})}><option value="docker.restart">Restart Docker container</option><option value="system.service_restart">Restart system service</option></select>
      <input value={remediation.target} onChange={e=>setRemediation({...remediation,target:e.target.value})} placeholder="Target container / service"/>
      <input value={remediation.reason} onChange={e=>setRemediation({...remediation,reason:e.target.value})} placeholder="Reason"/>
      <button className="primary" disabled={!selected||loading}>Propose action</button>
     </form>}
     {isOwner&&action&&<div className="finding"><p><b>{action.action}</b> → {action.target}</p><p>Status: {action.status}</p>{action.status==='pending_approval'&&<button className="primary" disabled={loading} onClick={()=>void approveAction()}>Approve as {principal.email}</button>}{action.verification?.summary&&<p>Verification: {action.verification.summary}</p>}</div>}
    </div>
   </section>

   {isOwner&&<section className="panel">
    <div className="panel-title"><h2>Team access</h2><span>Create member account</span></div>
    <form className="stack" onSubmit={createMember}><input type="email" value={memberForm.email} onChange={e=>setMemberForm({...memberForm,email:e.target.value})} placeholder="Member email"/><input type="password" value={memberForm.password} onChange={e=>setMemberForm({...memberForm,password:e.target.value})} placeholder="Temporary password (10+ characters)"/><button className="primary" disabled={loading}>Add member</button></form>
   </section>}

   <section className="panel">
    <div className="panel-title"><h2>Audit trail</h2><span>Organization scoped</span></div>
    {audit.slice(0,8).map(item=><p key={item.id}><b>{item.eventType}</b> · {item.actor} · {item.success?'success':'failed'} — {item.detail}</p>)}
   </section>
  </main>
 </div>
}
