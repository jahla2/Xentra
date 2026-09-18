import {FormEvent,useState} from 'react';
import {useWorkspace} from '../app/WorkspaceProvider';

export function ProjectsPage(){
 const{projects,isOwner,loading,createProject}=useWorkspace();
 const[form,setForm]=useState({name:'',description:''});

 async function submit(event:FormEvent){
  event.preventDefault();
  await createProject(form.name,form.description);
  setForm({name:'',description:''});
 }

 return <section className="panel">
  <div className="panel-title"><h2>Projects</h2><span>{isOwner?'Organize environments':'Read access'}</span></div>
  {projects.map(project=><div className="env" key={project.id}><span className="dot"/><div><b>{project.name}</b><small>{project.description||'No description'}</small><em>{project.id}</em></div></div>)}
  {!projects.length&&<p className="muted">Create a project before connecting environments.</p>}
  {isOwner&&<form className="stack" onSubmit={submit}>
   <input value={form.name} onChange={e=>setForm({...form,name:e.target.value})} placeholder="Project name" required/>
   <input value={form.description} onChange={e=>setForm({...form,description:e.target.value})} placeholder="Description (optional)"/>
   <button className="primary" disabled={loading}>Create project</button>
  </form>}
 </section>;
}
