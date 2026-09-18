import {FormEvent,useEffect,useMemo,useState} from 'react';
import {Link} from 'react-router-dom';
import {api,CreateEnvironmentInput} from '../api';
import {useWorkspace} from '../app/WorkspaceProvider';

type ConnectionChoice='runner_outbound'|'runner'|'ssh';

export function EnvironmentsPage(){
 const{
  projects,environments,isOwner,loading,selectedEnvironmentId,setSelectedEnvironmentId,
  createEnvironment,createRunnerEnrollment,runnerEnrollment,dismissRunnerEnrollment,
 }=useWorkspace();
 const[connectionChoice,setConnectionChoice]=useState<ConnectionChoice>('runner_outbound');
 const[form,setForm]=useState<CreateEnvironmentInput>({
  projectId:'',name:'Production Ubuntu',type:'production',connectionType:'runner',
  runnerUrl:'https://runner.example.com:8090',sshPort:22,
 });

 useEffect(()=>{
  if(!form.projectId&&projects[0])setForm(current=>({...current,projectId:projects[0].id}));
 },[projects,form.projectId]);

 const controlBase=useMemo(()=>{
  const configured=api.runnerControlUrl();
  if(configured)return configured.replace(/\/$/,'');
  return typeof window!=='undefined'?window.location.origin:'https://xentra.example.com';
 },[]);

 async function submit(event:FormEvent){
  event.preventDefault();
  if(connectionChoice==='runner_outbound'){
   await createRunnerEnrollment(form.projectId,form.name,form.type);
   return;
  }
  await createEnvironment({...form,connectionType:connectionChoice});
 }

 const update=(patch:Partial<CreateEnvironmentInput>)=>setForm(current=>({...current,...patch}));

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Connected environments</h2><span>{environments.length} total</span></div>
   {environments.map(env=><Link className={`env ${selectedEnvironmentId===env.id?'selected':''}`} to={`/environments/${env.id}`} key={env.id} onClick={()=>setSelectedEnvironmentId(env.id)}>
    <span className="dot"/><div>
     <b>{env.name}</b>
     <small>{env.hostname||env.sshHost||env.runnerUrl||'Waiting for Runner check-in'}</small>
     <em>{env.type} · {env.connectionType} · {env.os||'unknown OS'} · {env.cpu||'CPU pending'}</em>
    </div>
   </Link>)}
   {!environments.length&&<p className="muted">No environments connected yet.</p>}
  </div>

  <div className="panel">
   <div className="panel-title"><h2>Add environment</h2><span>{isOwner?'Owner managed':'Owner permission required'}</span></div>
   {isOwner&&<form className="stack" onSubmit={submit}>
    <select value={form.projectId} onChange={e=>update({projectId:e.target.value})} required>
     <option value="">Select project</option>{projects.map(project=><option value={project.id} key={project.id}>{project.name}</option>)}
    </select>
    <input value={form.name} onChange={e=>update({name:e.target.value})} placeholder="Environment name" required/>
    <select value={form.type} onChange={e=>update({type:e.target.value})}>
     <option value="development">Development</option>
     <option value="staging">Staging</option>
     <option value="production">Production</option>
    </select>
    <select value={connectionChoice} onChange={e=>setConnectionChoice(e.target.value as ConnectionChoice)}>
     <option value="runner_outbound">Outbound Xentra Runner (recommended)</option>
     <option value="runner">Legacy inbound Runner</option>
     <option value="ssh">Ubuntu / Linux SSH</option>
    </select>

    {connectionChoice==='runner_outbound'&&
     <div className="finding">
      <b>No inbound port required</b>
      <p>The Runner will connect outward to Xentra, poll typed tasks, and post results back. Enrollment credentials are shown once after creation.</p>
     </div>}

    {connectionChoice==='runner'&&
     <input value={form.runnerUrl??''} onChange={e=>update({runnerUrl:e.target.value})} placeholder="https://runner.example.com:8090" required/>}

    {connectionChoice==='ssh'&&<>
     <input value={form.sshHost??''} onChange={e=>update({sshHost:e.target.value})} placeholder="Host / IP" required/>
     <input type="number" value={form.sshPort??22} onChange={e=>update({sshPort:Number(e.target.value)})} placeholder="SSH port"/>
     <input value={form.sshUser??''} onChange={e=>update({sshUser:e.target.value})} placeholder="SSH username" required/>
     <input value={form.sshHostKeyFingerprint??''} onChange={e=>update({sshHostKeyFingerprint:e.target.value})} placeholder="Host key fingerprint (SHA256:...)" required/>
     <textarea value={form.sshPrivateKey??''} onChange={e=>update({sshPrivateKey:e.target.value})} placeholder="SSH private key" required/>
     <input type="password" value={form.sshPassphrase??''} onChange={e=>update({sshPassphrase:e.target.value})} placeholder="Key passphrase (optional)"/>
    </>}

    <button className="primary" disabled={loading||!projects.length}>
     {connectionChoice==='runner_outbound'?'Create Runner enrollment':'Test & connect environment'}
    </button>
   </form>}

   {runnerEnrollment&&<div className="finding">
    <div className="confidence">One-time Runner credentials</div>
    <h3>{runnerEnrollment.environment.name}</h3>
    <p>Copy these values to the target host now. The token is not shown again after dismissal.</p>
    <b>Production environment</b>
    <pre>{`XENTRA_CONTROL_URL=${controlBase}
XENTRA_RUNNER_ID=${runnerEnrollment.runnerId}
XENTRA_RUNNER_TOKEN=${runnerEnrollment.runnerToken}
XENTRA_CONTROL_CLIENT_CERT=/etc/xentra/tls/runner-client.crt
XENTRA_CONTROL_CLIENT_KEY=/etc/xentra/tls/runner-client.key
XENTRA_CONTROL_SERVER_CA=/etc/xentra/tls/control-ca.crt`}</pre>
    <b>Local development only</b>
    <pre>{`XENTRA_CONTROL_URL=${controlBase}
XENTRA_RUNNER_ID=${runnerEnrollment.runnerId}
XENTRA_RUNNER_TOKEN=${runnerEnrollment.runnerToken}
XENTRA_CONTROL_INSECURE_DEV=true`}</pre>
    <button className="primary" onClick={dismissRunnerEnrollment}>I saved the Runner credentials</button>
   </div>}
  </div>
 </section>;
}
