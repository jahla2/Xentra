import {FormEvent,useEffect,useState} from 'react';
import {CreateEnvironmentInput} from '../api';
import {useWorkspace} from '../app/WorkspaceProvider';

export function EnvironmentsPage(){
 const{projects,environments,isOwner,loading,selectedEnvironmentId,setSelectedEnvironmentId,createEnvironment}=useWorkspace();
 const[form,setForm]=useState<CreateEnvironmentInput>({
  projectId:'',name:'Production Ubuntu',type:'production',connectionType:'runner',
  runnerUrl:'https://runner.example.com:8090',sshPort:22,
 });

 useEffect(()=>{
  if(!form.projectId&&projects[0])setForm(current=>({...current,projectId:projects[0].id}));
 },[projects,form.projectId]);

 async function submit(event:FormEvent){
  event.preventDefault();
  await createEnvironment(form);
 }

 const update=(patch:Partial<CreateEnvironmentInput>)=>setForm(current=>({...current,...patch}));

 return <section className="grid workbench">
  <div className="panel">
   <div className="panel-title"><h2>Connected environments</h2><span>{environments.length} total</span></div>
   {environments.map(env=><button className={`env ${selectedEnvironmentId===env.id?'selected':''}`} key={env.id} onClick={()=>setSelectedEnvironmentId(env.id)}>
    <span className="dot"/><div><b>{env.name}</b><small>{env.hostname||env.sshHost||env.runnerUrl}</small><em>{env.type} · {env.connectionType} · {env.os||'unknown OS'}</em></div>
   </button>)}
   {!environments.length&&<p className="muted">No environments connected yet.</p>}
  </div>
  <div className="panel">
   <div className="panel-title"><h2>Add environment</h2><span>{isOwner?'Owner managed':'Owner permission required'}</span></div>
   {isOwner&&<form className="stack" onSubmit={submit}>
    <select value={form.projectId} onChange={e=>update({projectId:e.target.value})} required>
     <option value="">Select project</option>{projects.map(project=><option value={project.id} key={project.id}>{project.name}</option>)}
    </select>
    <input value={form.name} onChange={e=>update({name:e.target.value})} placeholder="Environment name" required/>
    <select value={form.type} onChange={e=>update({type:e.target.value})}><option value="development">Development</option><option value="staging">Staging</option><option value="production">Production</option></select>
    <select value={form.connectionType} onChange={e=>update({connectionType:e.target.value as 'runner'|'ssh'})}><option value="runner">Xentra Runner</option><option value="ssh">Ubuntu / Linux SSH</option></select>
    {form.connectionType==='runner'
     ?<input value={form.runnerUrl??''} onChange={e=>update({runnerUrl:e.target.value})} placeholder="https://runner.example.com:8090" required/>
     :<>
       <input value={form.sshHost??''} onChange={e=>update({sshHost:e.target.value})} placeholder="Host / IP" required/>
       <input type="number" value={form.sshPort??22} onChange={e=>update({sshPort:Number(e.target.value)})} placeholder="SSH port"/>
       <input value={form.sshUser??''} onChange={e=>update({sshUser:e.target.value})} placeholder="SSH username" required/>
       <input value={form.sshHostKeyFingerprint??''} onChange={e=>update({sshHostKeyFingerprint:e.target.value})} placeholder="Host key fingerprint (SHA256:...)" required/>
       <textarea value={form.sshPrivateKey??''} onChange={e=>update({sshPrivateKey:e.target.value})} placeholder="SSH private key" required/>
       <input type="password" value={form.sshPassphrase??''} onChange={e=>update({sshPassphrase:e.target.value})} placeholder="Key passphrase (optional)"/>
      </>}
    <button className="primary" disabled={loading||!projects.length}>Test & connect environment</button>
   </form>}
  </div>
 </section>;
}
