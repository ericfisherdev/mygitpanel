import { useState } from "react";
import {
  GitPullRequest, GitMerge, MessageSquare, AlertCircle, GitBranch,
  Bell, Settings, Search, LayoutDashboard, BookOpen,
  Filter, RefreshCw, ExternalLink, Clock,
  Users, Zap, Eye, ArrowRight,
  ChevronRight, Star, Circle, CheckCircle2, Inbox
} from "lucide-react";

const C = {
  bg:       "#0d0d10",
  surface:  "#141417",
  card:     "#1a1a1f",
  border:   "rgba(255,255,255,0.07)",
  text:     "#e8e8f0",
  muted:    "#6b6b80",
  subtle:   "#9090a8",
  indigo:   "#6366f1",
  purple:   "#8b5cf6",
  emerald:  "#10b981",
  amber:    "#f59e0b",
  red:      "#ef4444",
  blue:     "#3b82f6",
  sky:      "#38bdf8",
  teal:     "#14b8a6",
  violet:   "#7c3aed",
};

const AVATAR_BG = [C.indigo, "#ec4899", C.amber, C.emerald, C.blue, C.purple];
const Avatar = ({ name, size = 28 }) => {
  const initials = name?.split(" ").map(n=>n[0]).join("").slice(0,2).toUpperCase();
  const bg = AVATAR_BG[name?.charCodeAt(0) % AVATAR_BG.length];
  return (
    <div style={{width:size,height:size,borderRadius:"50%",background:bg,display:"flex",alignItems:"center",justifyContent:"center",fontSize:size<24?9:11,fontWeight:700,color:"#fff",flexShrink:0}}>
      {initials}
    </div>
  );
};

const Badge = ({ children, color }) => {
  const map = {
    blue:   {bg:"rgba(99,102,241,0.15)",  text:"#818cf8"},
    green:  {bg:"rgba(16,185,129,0.15)",  text:"#34d399"},
    red:    {bg:"rgba(239,68,68,0.15)",   text:"#f87171"},
    yellow: {bg:"rgba(245,158,11,0.15)",  text:"#fbbf24"},
    purple: {bg:"rgba(139,92,246,0.15)",  text:"#a78bfa"},
    gray:   {bg:"rgba(107,114,128,0.15)", text:"#9ca3af"},
  };
  const s = map[color] || map.gray;
  return <span style={{background:s.bg,color:s.text,padding:"2px 8px",borderRadius:99,fontSize:10,fontWeight:600,whiteSpace:"nowrap"}}>{children}</span>;
};

const activityFeed = [
  { id:1,  type:"pr_opened",    user:"Alex Kim",    repo:"frontend-web", title:"feat: add dark mode toggle",              time:"2m ago",  branch:"feature/dark-mode", num:"#312" },
  { id:2,  type:"jira_comment", user:"Sara Lee",    project:"GIT",       title:"API rate limiting not working",          time:"8m ago",  ticket:"GIT-204", comment:"Reproduced locally — looks like the middleware is skipped on /v2 routes." },
  { id:3,  type:"pr_merged",    user:"Dan Torres",  repo:"api-gateway",  title:"fix: resolve CORS headers on preflight", time:"14m ago", branch:"fix/cors", num:"#87" },
  { id:4,  type:"status_change",user:"Mia Chen",    project:"GIT",       title:"Dashboard pagination",                   time:"22m ago", ticket:"GIT-198", from:"In Progress", to:"In Review" },
  { id:5,  type:"pr_comment",   user:"Raj Patel",   repo:"frontend-web", title:"feat: add dark mode toggle",             time:"35m ago", num:"#312", comment:"LGTM, just one nit about the z-index stacking context." },
  { id:6,  type:"jira_opened",  user:"Priya Singh",  project:"GIT",      title:"Sync lag on large repo imports (>1k)",   time:"47m ago", ticket:"GIT-211", priority:"High" },
  { id:7,  type:"pr_opened",    user:"Tom White",   repo:"core-sync",    title:"chore: update dependencies to latest",   time:"1h ago",  branch:"chore/deps", num:"#54" },
  { id:8,  type:"status_change",user:"Alex Kim",    project:"GIT",       title:"OAuth token refresh",                    time:"1h ago",  ticket:"GIT-190", from:"To Do", to:"In Progress" },
  { id:9,  type:"pr_opened",    user:"Cleo Park",   repo:"core-sync",    title:"refactor: extract shared auth utilities", time:"2h ago", branch:"refactor/auth", num:"#55" },
  { id:10, type:"jira_comment", user:"Dan Torres",  project:"OPS",       title:"Deployment pipeline timeout on prod",    time:"2h ago",  ticket:"OPS-44", comment:"Added 30s buffer to step 3. Can someone re-test?" },
];

const myIssues = [
  { id:"GIT-204", title:"API rate limiting not working",  status:"In Review",   priority:"High",   updated:"2m ago" },
  { id:"GIT-190", title:"OAuth token refresh",            status:"In Progress", priority:"Medium", updated:"1h ago" },
  { id:"GIT-178", title:"Webhook delivery retry queue",   status:"In Progress", priority:"High",   updated:"3h ago" },
  { id:"OPS-39",  title:"CI runner memory leak",          status:"To Do",       priority:"Low",    updated:"1d ago" },
  { id:"GIT-201", title:"Bulk import CSV validation",     status:"Done",        priority:"Medium", updated:"2d ago" },
];

const repos = [
  { name:"frontend-web",  prs:4, issues:12, lastCommit:"3m ago",  lang:"TypeScript", stars:28 },
  { name:"api-gateway",   prs:2, issues:7,  lastCommit:"14m ago", lang:"Go",         stars:41 },
  { name:"core-sync",     prs:3, issues:5,  lastCommit:"2h ago",  lang:"Rust",       stars:19 },
  { name:"infra-charts",  prs:1, issues:3,  lastCommit:"5h ago",  lang:"YAML",       stars:6  },
];

const statCards = [
  { label:"Open PRs",       value:10, delta:"+3", color:C.indigo,  Icon: GitPullRequest },
  { label:"Jira In Review", value:6,  delta:"+1", color:C.amber,   Icon: Eye },
  { label:"Merged Today",   value:3,  delta:"-1", color:C.emerald, Icon: GitMerge },
  { label:"Overdue Issues", value:2,  delta:"0",  color:C.red,     Icon: AlertCircle },
];

const typeConfig = {
  pr_opened:    { Icon: GitPullRequest, color:C.blue,   bg:"rgba(59,130,246,0.12)",   label:"PR Opened" },
  pr_merged:    { Icon: GitMerge,       color:C.purple, bg:"rgba(139,92,246,0.12)",   label:"PR Merged" },
  pr_comment:   { Icon: MessageSquare,  color:C.sky,    bg:"rgba(56,189,248,0.12)",   label:"PR Comment" },
  merge_request:{ Icon: GitBranch,      color:C.emerald,bg:"rgba(16,185,129,0.12)",   label:"Merge Request" },
  jira_opened:  { Icon: AlertCircle,    color:C.amber,  bg:"rgba(245,158,11,0.12)",   label:"Ticket Created" },
  jira_comment: { Icon: MessageSquare,  color:C.teal,   bg:"rgba(20,184,166,0.12)",   label:"Jira Comment" },
  status_change:{ Icon: Zap,            color:C.violet, bg:"rgba(124,58,237,0.12)",   label:"Status Changed" },
};

const statusColor = { "To Do":"gray","In Progress":"blue","In Review":"yellow","Done":"green" };
const priorityColor = { "High":"red","Medium":"yellow","Low":"green" };

const navItems = [
  { Icon: LayoutDashboard, label:"Dashboard",    id:"dashboard" },
  { Icon: Inbox,           label:"Activity",     id:"activity" },
  { Icon: GitPullRequest,  label:"Pull Requests",id:"prs" },
  { Icon: AlertCircle,     label:"Issues",       id:"issues" },
  { Icon: BookOpen,        label:"Repos",        id:"repos" },
  { Icon: Users,           label:"Team",         id:"team" },
];

const s = {
  row: { display:"flex", alignItems:"center" },
  col: { display:"flex", flexDirection:"column" },
  card: { background:C.card, border:`1px solid ${C.border}`, borderRadius:16 },
};

export default function App() {
  const [activeNav, setActiveNav] = useState("dashboard");
  const [filterType, setFilterType] = useState("all");
  const [searchQ, setSearchQ] = useState("");

  const filtered = activityFeed.filter(a => {
    const matchType = filterType === "all" || a.type.startsWith(filterType);
    const matchSearch = !searchQ || a.title.toLowerCase().includes(searchQ.toLowerCase()) || a.user.toLowerCase().includes(searchQ.toLowerCase());
    return matchType && matchSearch;
  });

  return (
    <div style={{display:"flex",height:"100vh",background:C.bg,color:C.text,fontFamily:"Inter,system-ui,sans-serif",overflow:"hidden"}}>

      {/* ── Sidebar ── */}
      <aside style={{width:56,display:"flex",flexDirection:"column",alignItems:"center",padding:"12px 0",gap:4,background:C.surface,borderRight:`1px solid ${C.border}`,flexShrink:0}}>
        <div style={{width:36,height:36,borderRadius:10,background:`linear-gradient(135deg,${C.indigo},${C.purple})`,display:"flex",alignItems:"center",justifyContent:"center",marginBottom:12}}>
          <GitBranch size={16} color="#fff" />
        </div>
        {navItems.map(({ Icon, label, id }) => (
          <button key={id} title={label} onClick={() => setActiveNav(id)}
            style={{width:38,height:38,borderRadius:10,border:"none",cursor:"pointer",display:"flex",alignItems:"center",justifyContent:"center",
              background: activeNav===id ? "rgba(99,102,241,0.2)" : "transparent",
              color: activeNav===id ? C.indigo : C.muted,
              transition:"all 0.15s"}}>
            <Icon size={17} />
          </button>
        ))}
        <div style={{flex:1}} />
        <button style={{width:38,height:38,borderRadius:10,border:"none",cursor:"pointer",display:"flex",alignItems:"center",justifyContent:"center",background:"transparent",color:C.muted}}>
          <Settings size={17} />
        </button>
        <Avatar name="Jordan Wells" size={30} />
      </aside>

      {/* ── Main ── */}
      <div style={{flex:1,display:"flex",flexDirection:"column",overflow:"hidden"}}>

        {/* Topbar */}
        <header style={{...s.row,gap:12,padding:"10px 20px",background:C.surface,borderBottom:`1px solid ${C.border}`,flexShrink:0}}>
          <div style={{...s.row,gap:6}}>
            <span style={{fontSize:18,fontWeight:700,letterSpacing:"-0.5px"}}>Gitira</span>
            <span style={{color:C.muted,fontSize:13}}>/</span>
            <span style={{color:C.muted,fontSize:13}}>Dashboard</span>
          </div>
          <div style={{position:"relative",marginLeft:16,maxWidth:300,flex:1}}>
            <Search size={13} style={{position:"absolute",left:10,top:"50%",transform:"translateY(-50%)",color:C.muted}} />
            <input value={searchQ} onChange={e=>setSearchQ(e.target.value)}
              placeholder="Search issues, PRs, repos…"
              style={{width:"100%",background:"rgba(255,255,255,0.05)",border:`1px solid ${C.border}`,borderRadius:8,padding:"6px 10px 6px 30px",fontSize:12,color:C.text,outline:"none",boxSizing:"border-box"}} />
          </div>
          <div style={{flex:1}} />
          <button style={{...s.row,gap:6,fontSize:11,color:C.subtle,background:"rgba(255,255,255,0.05)",border:`1px solid ${C.border}`,borderRadius:8,padding:"5px 10px",cursor:"pointer"}}>
            <RefreshCw size={11} /> Sync now
          </button>
          <div style={{position:"relative",cursor:"pointer"}}>
            <Bell size={17} color={C.subtle} />
            <span style={{position:"absolute",top:-4,right:-4,width:15,height:15,background:C.red,borderRadius:"50%",fontSize:9,display:"flex",alignItems:"center",justifyContent:"center",fontWeight:700,color:"#fff"}}>5</span>
          </div>
          <Avatar name="Jordan Wells" size={28} />
        </header>

        {/* Content */}
        <div style={{flex:1,overflow:"auto",padding:16,display:"grid",gridTemplateColumns:"repeat(12,1fr)",gridAutoRows:"min-content",gap:12}}>

          {/* Stat cards */}
          {statCards.map(({ label, value, delta, color, Icon }, i) => (
            <div key={i} style={{...s.card,...s.row,gridColumn:"span 3",padding:"14px 16px",gap:12}}>
              <div style={{width:38,height:38,borderRadius:10,background:color+"22",display:"flex",alignItems:"center",justifyContent:"center",flexShrink:0}}>
                <Icon size={17} color={color} />
              </div>
              <div>
                <div style={{fontSize:22,fontWeight:700,lineHeight:1}}>{value}</div>
                <div style={{fontSize:11,color:C.muted,marginTop:3}}>{label}</div>
              </div>
              <div style={{marginLeft:"auto"}}>
                <span style={{fontSize:11,fontWeight:600,color: delta.startsWith("+") ? C.emerald : delta==="0" ? C.muted : C.red}}>
                  {delta !== "0" ? delta+" today" : "no change"}
                </span>
              </div>
            </div>
          ))}

          {/* Activity Feed */}
          <div style={{...s.card,...s.col,gridColumn:"span 7",gridRow:"span 2",maxHeight:500,overflow:"hidden"}}>
            {/* Feed header */}
            <div style={{...s.row,gap:8,padding:"14px 16px 10px",borderBottom:`1px solid ${C.border}`,flexShrink:0}}>
              <span style={{fontWeight:600,fontSize:13}}>Activity Feed</span>
              <span style={{fontSize:10,color:C.muted,background:"rgba(255,255,255,0.05)",padding:"1px 7px",borderRadius:99}}>{filtered.length}</span>
              <div style={{flex:1}} />
              {[["all","All"],["pr","PRs"],["jira","Jira"],["status","Status"]].map(([v,l]) => (
                <button key={v} onClick={() => setFilterType(v)}
                  style={{fontSize:11,padding:"3px 9px",borderRadius:7,border:"none",cursor:"pointer",
                    background: filterType===v ? "rgba(99,102,241,0.2)" : "transparent",
                    color: filterType===v ? C.indigo : C.muted}}>
                  {l}
                </button>
              ))}
              <Filter size={13} color={C.muted} style={{cursor:"pointer"}} />
            </div>
            {/* Items */}
            <div style={{overflowY:"auto",flex:1,padding:"6px 8px"}}>
              {filtered.map(item => {
                const cfg = typeConfig[item.type];
                const Icon = cfg.Icon;
                return (
                  <div key={item.id} style={{...s.row,alignItems:"flex-start",gap:10,padding:"9px 10px",borderRadius:10,cursor:"pointer",transition:"background 0.1s"}}
                    onMouseEnter={e=>e.currentTarget.style.background="rgba(255,255,255,0.04)"}
                    onMouseLeave={e=>e.currentTarget.style.background="transparent"}>
                    <div style={{width:28,height:28,borderRadius:8,background:cfg.bg,display:"flex",alignItems:"center",justifyContent:"center",flexShrink:0,marginTop:1}}>
                      <Icon size={13} color={cfg.color} />
                    </div>
                    <div style={{flex:1,minWidth:0}}>
                      <div style={{...s.row,gap:6,flexWrap:"wrap"}}>
                        <span style={{fontSize:11,fontWeight:600,color:C.subtle}}>{item.user}</span>
                        <span style={{fontSize:9,color:cfg.color,background:cfg.bg,padding:"2px 6px",borderRadius:99,fontWeight:600}}>{cfg.label}</span>
                        {item.repo    && <span style={{fontSize:9,color:C.muted}}>in <span style={{color:C.subtle}}>{item.repo}</span></span>}
                        {item.project && <span style={{fontSize:9,color:C.muted}}>in <span style={{color:C.subtle}}>{item.project}</span></span>}
                      </div>
                      <div style={{fontSize:12,fontWeight:500,color:C.text,marginTop:2,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>
                        {item.num    && <span style={{color:C.muted,marginRight:4}}>{item.num}</span>}
                        {item.ticket && <span style={{color:C.indigo,marginRight:4,fontFamily:"monospace"}}>{item.ticket}</span>}
                        {item.title}
                      </div>
                      {item.comment && <div style={{fontSize:10,color:C.muted,marginTop:2,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap",fontStyle:"italic"}}>"{item.comment}"</div>}
                      {item.type==="status_change" && (
                        <div style={{...s.row,gap:6,marginTop:4}}>
                          <Badge color={statusColor[item.from]||"gray"}>{item.from}</Badge>
                          <ArrowRight size={10} color={C.muted} />
                          <Badge color={statusColor[item.to]||"gray"}>{item.to}</Badge>
                        </div>
                      )}
                    </div>
                    <span style={{fontSize:9,color:C.muted,flexShrink:0,marginTop:2}}>{item.time}</span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* My Jira Issues */}
          <div style={{...s.card,...s.col,gridColumn:"span 5",maxHeight:240,overflow:"hidden"}}>
            <div style={{...s.row,padding:"14px 16px 10px",borderBottom:`1px solid ${C.border}`,flexShrink:0}}>
              <span style={{fontWeight:600,fontSize:13}}>My Jira Issues</span>
              <span style={{marginLeft:8,fontSize:10,color:C.muted,background:"rgba(255,255,255,0.05)",padding:"1px 7px",borderRadius:99}}>{myIssues.length}</span>
              <div style={{flex:1}} />
              <button style={{...s.row,gap:3,fontSize:11,color:C.indigo,background:"none",border:"none",cursor:"pointer"}}>View all <ChevronRight size={11} /></button>
            </div>
            <div style={{overflowY:"auto",flex:1}}>
              {myIssues.map(issue => (
                <div key={issue.id} style={{...s.row,gap:10,padding:"8px 14px",borderBottom:`1px solid rgba(255,255,255,0.04)`,cursor:"pointer"}}
                  onMouseEnter={e=>e.currentTarget.style.background="rgba(255,255,255,0.04)"}
                  onMouseLeave={e=>e.currentTarget.style.background="transparent"}>
                  {issue.status==="Done"        && <CheckCircle2 size={14} color={C.emerald} style={{flexShrink:0}} />}
                  {issue.status==="In Review"   && <Eye           size={14} color={C.amber}   style={{flexShrink:0}} />}
                  {issue.status==="In Progress" && <Clock         size={14} color={C.blue}    style={{flexShrink:0}} />}
                  {issue.status==="To Do"       && <Circle        size={14} color={C.muted}   style={{flexShrink:0}} />}
                  <div style={{flex:1,minWidth:0}}>
                    <div style={{...s.row,gap:6}}>
                      <span style={{fontSize:10,color:C.indigo,fontFamily:"monospace",flexShrink:0}}>{issue.id}</span>
                      <span style={{fontSize:11,color:C.text,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>{issue.title}</span>
                    </div>
                  </div>
                  <Badge color={priorityColor[issue.priority]}>{issue.priority}</Badge>
                  <span style={{fontSize:9,color:C.muted,flexShrink:0}}>{issue.updated}</span>
                </div>
              ))}
            </div>
          </div>

          {/* Repos */}
          <div style={{...s.card,...s.col,gridColumn:"span 5",maxHeight:240,overflow:"hidden"}}>
            <div style={{...s.row,padding:"14px 16px 10px",borderBottom:`1px solid ${C.border}`,flexShrink:0}}>
              <span style={{fontWeight:600,fontSize:13}}>Repositories</span>
              <span style={{marginLeft:8,fontSize:10,color:C.muted,background:"rgba(255,255,255,0.05)",padding:"1px 7px",borderRadius:99}}>{repos.length}</span>
              <div style={{flex:1}} />
              <button style={{...s.row,gap:3,fontSize:11,color:C.indigo,background:"none",border:"none",cursor:"pointer"}}>View all <ChevronRight size={11} /></button>
            </div>
            <div style={{overflowY:"auto",flex:1}}>
              {repos.map(r => (
                <div key={r.name} style={{...s.row,gap:10,padding:"8px 14px",borderBottom:`1px solid rgba(255,255,255,0.04)`,cursor:"pointer"}}
                  onMouseEnter={e=>e.currentTarget.style.background="rgba(255,255,255,0.04)"}
                  onMouseLeave={e=>e.currentTarget.style.background="transparent"}>
                  <div style={{width:28,height:28,borderRadius:8,background:"rgba(99,102,241,0.15)",display:"flex",alignItems:"center",justifyContent:"center",flexShrink:0}}>
                    <GitBranch size={13} color={C.indigo} />
                  </div>
                  <div style={{flex:1,minWidth:0}}>
                    <div style={{fontSize:12,fontWeight:500,color:C.text,overflow:"hidden",textOverflow:"ellipsis",whiteSpace:"nowrap"}}>{r.name}</div>
                    <div style={{...s.row,gap:8,marginTop:2}}>
                      <span style={{fontSize:9,color:C.muted}}>{r.lang}</span>
                      <span style={{...s.row,gap:3,fontSize:9,color:C.muted}}><Star size={8} />{r.stars}</span>
                      <span style={{fontSize:9,color:C.muted}}>{r.lastCommit}</span>
                    </div>
                  </div>
                  <div style={{...s.row,gap:6,flexShrink:0}}>
                    <span style={{...s.row,gap:3,fontSize:10,color:C.blue,background:"rgba(59,130,246,0.12)",padding:"2px 7px",borderRadius:6}}>
                      <GitPullRequest size={9} />{r.prs}
                    </span>
                    <span style={{...s.row,gap:3,fontSize:10,color:C.amber,background:"rgba(245,158,11,0.12)",padding:"2px 7px",borderRadius:6}}>
                      <AlertCircle size={9} />{r.issues}
                    </span>
                  </div>
                  <ExternalLink size={11} color={C.muted} />
                </div>
              ))}
            </div>
          </div>

        </div>
      </div>
    </div>
  );
}
