const $=id=>document.getElementById(id);let timer,slug='inicio',pageId=1,editing=false,widgets=[],snapshot=[],undoStack=[],redoStack=[],drag=null;const cols={desktop:12,tablet:8,mobile:4};
async function api(u,o={}){const r=await fetch(u,{headers:{'Content-Type':'application/json',...(o.headers||{})},...o});if(!r.ok)throw Error((await r.json().catch(()=>({}))).error||r.statusText);return r.status===204?{}:r.json()}
const bp=()=>editing?$('breakpoint').value:(innerWidth<700?'mobile':innerWidth<1000?'tablet':'desktop');const clone=x=>JSON.parse(JSON.stringify(x));
function value(w){
    if(w.type==='information.clock'){
        return new Date().toLocaleTimeString([],{
            hour:'2-digit',
            minute:'2-digit'
        });
    }

    if(w.type==='application.shortcut'){
        return w.application
            ? `<a class="launch" href="${esc(w.application.url)}" target="_blank" rel="noopener">Abrir</a>${monitor(w.application.monitor)}`
            : 'Aplicación';
    }

    if(w.type==='information.iframe'){
        return `<iframe class="iframeWidget" loading="lazy" sandbox="allow-scripts allow-forms allow-same-origin" src="${esc(w.config?.url||'about:blank')}"></iframe>`;
    }

    if(w.type==='information.json'){
        return `<pre class="jsonWidget">${esc(JSON.stringify(w.config?.data||{},null,2))}</pre>`;
    }

    const d = w.data?.data ?? w.data ?? {};

    switch(w.type){
        case 'system.cpu':
            return d.percent != null
                ? `${Number(d.percent).toFixed(1)} %`
                : 'No disponible';

        case 'system.memory':
            return d.percent != null
                ? `${Number(d.percent).toFixed(1)} %`
                : 'No disponible';

        case 'system.load':
            return d.load1 != null
                ? String(d.load1)
                : 'No disponible';

        case 'system.uptime':
            if(d.seconds == null) return 'No disponible';

            {
                const total = Math.floor(Number(d.seconds));
                const days = Math.floor(total / 86400);
                const hours = Math.floor((total % 86400) / 3600);
                const minutes = Math.floor((total % 3600) / 60);

                if(days > 0) return `${days}d ${hours}h`;
                if(hours > 0) return `${hours}h ${minutes}m`;

                return `${minutes}m`;
            }

        case 'system.temperature':
            return d.celsius != null
                ? `${Number(d.celsius).toFixed(1)} C`
                : 'No disponible';

        case 'network.internet':
            if(d.online === true) return 'Conectado';
            if(d.online === false) return 'Sin conexión';
            return 'No disponible';

        case 'storage.disks':
            if(Array.isArray(d.disks) && d.disks.length && d.disks[0].percent != null){
                return `${Number(d.disks[0].percent).toFixed(1)} %`;
            }
            return 'No disponible';
    }

    return esc(String(
        d.percent ??
        d.temperature_c ??
        d.uptime_seconds ??
        d.online ??
        d.used_percent ??
        d.value ??
        ''
    ));
}
function esc(x){return String(x).replace(/[&<>"']/g,m=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[m]))}function monitor(m){if(!m)return'';return `<div class="monitor ${(m.status||'unknown').toLowerCase()}"><i></i>${esc(m.status||'UNKNOWN')}</div>`}
function controls(w){if(!editing)return'';return `<div class="tools"><button data-a="props">Propiedades</button><button data-a="duplicate">Duplicar</button><button data-a="hide">Ocultar</button><button data-a="delete">Eliminar</button></div><i class="resizeHandle" data-resize="1"></i>`}
function pushUndo(){undoStack.push(clone(widgets));if(undoStack.length>40)undoStack.shift();redoStack=[]}
function render(){const g=$('grid');g.className=`grid ${bp()}`;g.innerHTML='';const children=new Map();widgets.forEach(w=>{if(w.parent_widget_id){if(!children.has(w.parent_widget_id))children.set(w.parent_widget_id,[]);children.get(w.parent_widget_id).push(w)}});for(const w of widgets){if(w.parent_widget_id)continue;const l=w.layout||{x:0,y:0,w:3,h:2},a=document.createElement('article');a.className=`card ${editing?'editing':''} ${w.type==='layout.group'?'groupCard':''}`;a.dataset.id=w.id;a.style.gridColumn=`${Math.min(l.x,cols[bp()]-1)+1} / span ${Math.min(l.w,cols[bp()])}`;a.style.gridRow=`${l.y+1} / span ${Math.max(1,l.h)}`;let body=w.type==='layout.group'?`<strong>${esc(w.title||'Grupo')}</strong><div class="groupChildren">${(children.get(w.id)||[]).map(c=>`<div class="card"><span>${esc(c.title||c.type)}</span><strong>${value(c)}</strong></div>`).join('')}</div>`:`<span>${esc(w.title||w.type)}</span><strong>${value(w)}</strong>`;a.innerHTML=body+controls(w);g.appendChild(a)}}
async function loadPages(){const j=await api('/api/v1/pages');$('pages').innerHTML=(j.pages||[]).map(p=>`<button data-id="${p.id}" data-slug="${esc(p.slug)}">${esc(p.name)}</button>`).join('');const p=(j.pages||[]).find(x=>x.slug===slug);if(p)pageId=p.id}
async function refresh(){try{const j=await api(`/api/v1/runtime/page/${encodeURIComponent(slug)}?breakpoint=${bp()}`);widgets=j.widgets||[];pageId=j.page?.id||pageId;$('pageTitle').textContent=j.page?.name||slug;render();$('status').textContent='Conectado';loadTheme()}catch(e){$('status').textContent=e.message}}
function start(){clearInterval(timer);refresh();if(!editing&&!document.hidden)timer=setInterval(refresh,30000)}
$('pages').onclick=e=>{if(e.target.dataset.slug){slug=e.target.dataset.slug;pageId=Number(e.target.dataset.id);start()}};
$('edit').onclick=()=>{editing=true;snapshot=clone(widgets);undoStack=[];redoStack=[];$('editor').hidden=false;clearInterval(timer);render()};$('cancel').onclick=()=>{widgets=clone(snapshot);editing=false;$('editor').hidden=true;start()};$('breakpoint').onchange=refresh;
$('undo').onclick=()=>{if(!undoStack.length)return;redoStack.push(clone(widgets));widgets=undoStack.pop();render()};$('redo').onclick=()=>{if(!redoStack.length)return;undoStack.push(clone(widgets));widgets=redoStack.pop();render()};
$('grid').addEventListener('pointerdown',e=>{if(!editing)return;const card=e.target.closest('.card[data-id]');if(!card)return;const w=widgets.find(x=>String(x.id)===card.dataset.id);if(!w)return;pushUndo();const l=w.layout;drag={w,card,sx:e.clientX,sy:e.clientY,start:clone(l),resize:!!e.target.dataset.resize};card.classList.add('dragging');card.setPointerCapture(e.pointerId)});
$('grid').addEventListener('pointermove',e=>{if(!drag)return;const rect=$('grid').getBoundingClientRect(),cw=rect.width/cols[bp()],rh=64+parseFloat(getComputedStyle($('grid')).gap||0),dx=Math.round((e.clientX-drag.sx)/cw),dy=Math.round((e.clientY-drag.sy)/rh),l=drag.w.layout,c=cols[bp()];if(drag.resize){l.w=Math.max(1,Math.min(c-drag.start.x,drag.start.w+dx));l.h=Math.max(1,drag.start.h+dy)}else{l.x=Math.max(0,Math.min(c-l.w,drag.start.x+dx));l.y=Math.max(0,drag.start.y+dy)}render()});
$('grid').addEventListener('pointerup',e=>{if(drag){drag.card.classList.remove('dragging');drag=null}});
$('grid').addEventListener('click',async e=>{const b=e.target.closest('button[data-a]');if(!b)return;const w=widgets.find(x=>String(x.id)===b.closest('.card').dataset.id),a=b.dataset.a;try{if(a==='props')return properties(w);if(a==='duplicate')await api(`/api/v1/widgets/${w.id}/duplicate`,{method:'POST'});if(a==='hide')await api(`/api/v1/widgets/${w.id}/hide`,{method:'POST'});if(a==='delete'&&confirm('Â¿Eliminar este widget?'))await api(`/api/v1/widgets/${w.id}`,{method:'DELETE'});await refresh()}catch(x){alert(x.message)}});
function properties(w){const groups=widgets.filter(x=>x.type==='layout.group'&&x.id!==w.id);$('modalBody').innerHTML=`<h2>Propiedades</h2><form id="propForm"><input name="title" value="${esc(w.title||'')}" placeholder="TÃ­tulo"><input name="subtitle" value="${esc(w.subtitle||'')}" placeholder="SubtÃ­tulo"><label>Grupo<select name="parent"><option value="0">Ninguno</option>${groups.map(g=>`<option value="${g.id}" ${w.parent_widget_id===g.id?'selected':''}>${esc(g.title)}</option>`).join('')}</select></label>${['information.iframe','information.json'].includes(w.type)?`<textarea name="config" rows="6">${esc(JSON.stringify(w.config||{},null,2))}</textarea>`:''}<button>Aplicar</button></form>`;$('modal').showModal();$('propForm').onsubmit=async e=>{e.preventDefault();const f=new FormData(e.target);let config=w.config||{};try{if(f.get('config'))config=JSON.parse(f.get('config'));await api(`/api/v1/widgets/${w.id}`,{method:'PUT',body:JSON.stringify({title:f.get('title'),subtitle:f.get('subtitle'),parent_widget_id:Number(f.get('parent')),config})});$('modal').close();refresh()}catch(x){alert(x.message)}}}
$('save').onclick=async()=>{try{await api('/api/v1/layouts',{method:'PUT',body:JSON.stringify(widgets.map(w=>({widget_id:w.id,breakpoint:bp(),x:w.layout.x,y:w.layout.y,w:w.layout.w,h:w.layout.h})))});snapshot=clone(widgets);$('status').textContent='Guardado'}catch(e){$('status').textContent=e.message}};
$('addPage').onclick=async()=>{const name=prompt('Nombre de la nueva pÃ¡gina');if(!name)return;const j=await api('/api/v1/pages',{method:'POST',body:JSON.stringify({name})});slug=j.slug;await loadPages();start()};
$('store').onclick=async()=>{const [c,a]=await Promise.all([api('/api/v1/widgets/catalog'),api('/api/v1/applications')]);$('modalBody').innerHTML=`<h2>Widget Store</h2>${(c.widgets||[]).map(x=>`<button class="storeItem" data-type="${x.type}">${esc(x.name)}</button>`).join('')}<h3>Aplicaciones</h3>${(a.applications||[]).map(x=>`<button class="appItem" data-app="${x.id}" data-name="${esc(x.name)}">${esc(x.name)}</button>`).join('')}`;$('modal').showModal()};
$('modal').addEventListener('click',async e=>{const b=e.target.closest('.storeItem,.appItem');if(!b)return;await api('/api/v1/widgets',{method:'POST',body:JSON.stringify({page_id:pageId,type:b.dataset.type||'application.shortcut',title:b.dataset.name||'',application_id:Number(b.dataset.app||0)})});$('modal').close();refresh()});$('closeModal').onclick=()=>$('modal').close();
$('search').oninput=e=>{const q=e.target.value.trim().toLowerCase(),box=$('searchResults');if(!q){box.hidden=true;return}const hits=widgets.filter(w=>(w.title||w.type).toLowerCase().includes(q));box.innerHTML=hits.map(w=>`<button class="searchHit" data-id="${w.id}">${esc(w.title||w.type)}</button>`).join('')||'Sin resultados';box.hidden=false};$('searchResults').onclick=e=>{const b=e.target.closest('[data-id]');if(!b)return;document.querySelector(`.card[data-id="${b.dataset.id}"]`)?.scrollIntoView({behavior:'smooth',block:'center'})};
$('kiosk').onclick=()=>{document.body.classList.toggle('kiosk');history.replaceState(null,'',document.body.classList.contains('kiosk')?'?kiosk=1':location.pathname)};if(new URLSearchParams(location.search).get('kiosk')==='1')document.body.classList.add('kiosk');
document.addEventListener('visibilitychange',()=>document.hidden?clearInterval(timer):start());
function applyTheme(t){const v=t?.values||{},r=document.documentElement;for(const [k,css] of Object.entries({bg:'--bg',surface:'--surface',text:'--text',muted:'--muted',accent:'--accent',border:'--border',radius:'--radius',gap:'--gap',padding:'--card-padding',blur:'--card-blur',overlay:'--overlay'}))if(v[k]!=null)r.style.setProperty(css,v[k]);if(v.card_opacity!=null)r.style.setProperty('--card-opacity',v.card_opacity);if(v.background_image)r.style.setProperty('--bg-image',`url("${String(v.background_image).replace(/["\\]/g,'')}")`)}async function loadTheme(){try{applyTheme((await api(`/api/v1/themes?page=${encodeURIComponent(slug)}`)).theme)}catch{}}
$('theme').onclick=async()=>{const ps=await api('/api/v1/themes/presets');$('modalBody').innerHTML=`<h2>Apariencia</h2>${Object.entries(ps.presets||{}).map(([k,x])=>`<button class="preset" data-preset="${k}">${esc(x.name)}</button>`).join('')}`;$('modal').showModal();$('modalBody').querySelectorAll('.preset').forEach(b=>b.onclick=async()=>{const x=ps.presets[b.dataset.preset];await api(`/api/v1/themes?page=${encodeURIComponent(slug)}`,{method:'PUT',body:JSON.stringify(x)});$('modal').close();loadTheme()})};
$('discover').onclick=async()=>{try{const j=await api('/api/v1/discovery');$('modalBody').innerHTML='<h2>Docker Discovery</h2>'+JSON.stringify(j,null,2);$('modal').showModal()}catch(e){alert(e.message)}};
loadPages().then(start);
