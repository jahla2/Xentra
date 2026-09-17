const navigation = ['Overview', 'Environments', 'Incidents', 'Ask AI', 'Runners', 'Audit'];

export function App() {
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">X</span>Xentra</div>
        <nav>
          {navigation.map((item, index) => (
            <button className={index === 0 ? 'nav-item active' : 'nav-item'} key={item}>{item}</button>
          ))}
        </nav>
      </aside>
      <main className="content">
        <header className="topbar">
          <div>
            <p className="eyebrow">DEVOPS COMMAND CENTER</p>
            <h1>Infrastructure overview</h1>
          </div>
          <button className="primary-action">Ask Xentra</button>
        </header>
        <section className="metrics-grid">
          <article className="metric-card"><span>Healthy</span><strong>0</strong><small>Connect your first environment</small></article>
          <article className="metric-card"><span>Warnings</span><strong>0</strong><small>No active warnings</small></article>
          <article className="metric-card"><span>Incidents</span><strong>0</strong><small>No open incidents</small></article>
        </section>
        <section className="empty-panel">
          <div className="orb">X</div>
          <h2>Connect an environment</h2>
          <p>Add an Ubuntu/Linux host or Xentra Runner to start safe infrastructure discovery.</p>
          <button className="primary-action">Add environment</button>
        </section>
      </main>
    </div>
  );
}
