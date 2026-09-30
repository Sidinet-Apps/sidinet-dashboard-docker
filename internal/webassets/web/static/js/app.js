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
            : 'AplicaciÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â³n';
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
            if(d.online === false) return 'Sin conexiÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â³n';
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
function controls(w){
    if(!editing)
        return '';

    return `
        <div class="cardEditTools">
            <button
                type="button"
                data-a="props"
                title="Propiedades"
                aria-label="Propiedades">Editar</button>

            <button
                type="button"
                data-a="duplicate"
                title="Duplicar"
                aria-label="Duplicar">Duplicar</button>

            <button
                type="button"
                data-a="delete"
                class="danger"
                title="Eliminar"
                aria-label="Eliminar">Eliminar</button>
        </div>

        <i
            class="resizeHandle"
            data-resize="1"
            title="Cambiar tamaño">
        </i>
    `;
}
function pushUndo(){undoStack.push(clone(widgets));if(undoStack.length>40)undoStack.shift();redoStack=[]}
function widgetData(w){
    return w.data?.data ?? w.data ?? {};
}

function metricPercent(w){
    const d=widgetData(w);

    if(w.type==='system.cpu' && d.percent!=null)
        return Number(d.percent);

    if(w.type==='system.memory' && d.percent!=null)
        return Number(d.percent);

    if(w.type==='storage.disks'){
        if(Array.isArray(d.disks) && d.disks.length && d.disks[0].percent!=null)
            return Number(d.disks[0].percent);
    }

    return null;
}

function metricBody(w){
    const percent=metricPercent(w);
    const title=esc(w.title||metricTitle(w.type));

    return `
        <div class="metricCardHeader">
            <span>${title}</span>
        </div>

        <div class="metricCardValue">${value(w)}</div>

        ${percent!=null?`
            <div class="metricProgress" aria-hidden="true">
                <div style="width:${Math.max(0,Math.min(100,percent))}%"></div>
            </div>
        `:''}
    `;
}

function metricTitle(type){
    const titles={
        'system.cpu':'CPU',
        'system.memory':'Memoria RAM',
        'system.load':'Carga',
        'system.uptime':'Uptime',
        'system.temperature':'Temperatura',
        'storage.disks':'Almacenamiento',
        'network.internet':'Internet'
    };

    return titles[type]||type;
}

function applicationBody(w){
    const app=w.application||{};
    const title=esc(w.title||app.name||'Aplicación');
    const description=esc(w.subtitle||app.description||'Acceso rápido');
    const url=app.url||'';
    const openMode=app.open_mode||'_blank';

    const target=
        openMode==='same' ||
        openMode==='_self'
            ? '_self'
            : '_blank';

    const initial=String(w.title||app.name||'A').trim().charAt(0).toUpperCase();
    const iconHint=String(app.icon_value||app.name||'').toLowerCase();
    const iconSlug=iconHint.replace(/^.*\//,'').replace(/[:@].*$/,'').replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
    const iconURL=iconSlug?'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/'+encodeURIComponent(iconSlug)+'.svg':'';

    const content=`
        <div class="applicationTop">
            <div class="applicationIcon" aria-hidden="true">
                ${iconURL?`<img src="${esc(iconURL)}" alt="" loading="lazy" data-icon-fallback="application"><span class="applicationFallback" hidden>${esc(initial)}</span>`:`<span class="applicationFallback">${esc(initial)}</span>`}
            </div>

            ${url?`
                <span class="applicationOpen" aria-hidden="true"></span>
            `:''}
        </div>

        <div class="applicationInfo">
            <strong>${title}</strong>
            <span>${description}</span>
        </div>

        ${monitor(w.monitor)}
    `;

    if(!url)
        return content;

    return `
        <a
            class="applicationLaunch"
            href="${esc(url)}"
            target="${target}"
            rel="noopener">
            ${content}
        </a>
    `;
}
function groupBody(w,children){
    const items=children.get(w.id)||[];

    return `
        <div class="groupHeader">
            <strong>${esc(w.title||'Grupo')}</strong>
            <span>${items.length} ${items.length===1?'elemento':'elementos'}</span>
        </div>

        <div class="groupChildren">
            ${items.map(c=>`
                <div class="card childCard">
                    <span>${esc(c.title||metricTitle(c.type))}</span>
                    <strong>${value(c)}</strong>
                </div>
            `).join('')}
        </div>
    `;
}

function widgetCategory(w){
    if(w.type==='application.shortcut')
        return 'applications';

    if(w.type==='layout.group')
        return 'groups';

    return 'system';
}

function createSectionHeader(title,subtitle,section){
    const header=document.createElement('div');

    header.className='dashboardGridHeader';
    header.dataset.section=section;

    header.innerHTML=`
        <div>
            <h2>${esc(title)}</h2>
            ${subtitle?`<p>${esc(subtitle)}</p>`:''}
        </div>
    `;

    return header;
}

function createWidgetCard(w,children){
    const l=w.layout||{x:0,y:0,w:3,h:2};
    const a=document.createElement('article');

    const isApplication=w.type==='application.shortcut';
    const isGroup=w.type==='layout.group';

    a.className=[
        'card',
        isApplication?'applicationCard':'',
        isGroup?'groupCard':'',
        !isApplication&&!isGroup?'metricCard':'',
        editing?'editing':''
    ].filter(Boolean).join(' ');

    a.dataset.id=w.id;
    a.dataset.type=w.type;

    a.style.gridColumn=
        `${Math.min(l.x,cols[bp()]-1)+1} / span ${Math.min(l.w,cols[bp()])}`;

    a.style.gridRow=
        `span ${Math.max(1,l.h)}`;

    let body;

    if(isGroup){
        body=groupBody(w,children);
    }else if(isApplication){
        body=applicationBody(w);
    }else{
        body=metricBody(w);
    }

    a.innerHTML=body+controls(w);

    return a;
}

function render(){
    const g=$('grid');

    g.className=`grid ${bp()}`;
    g.innerHTML='';

    const children=new Map();

    widgets.forEach(w=>{
        if(w.parent_widget_id){
            if(!children.has(w.parent_widget_id))
                children.set(w.parent_widget_id,[]);

            children.get(w.parent_widget_id).push(w);
        }
    });

    const visible=widgets.filter(w=>!w.parent_widget_id);

    const applications=visible.filter(
        w=>widgetCategory(w)==='applications'
    );

    const groups=visible.filter(
        w=>widgetCategory(w)==='groups'
    );

    const system=visible.filter(
        w=>widgetCategory(w)==='system'
    );

    if(applications.length){
        g.appendChild(
            createSectionHeader(
                'Aplicaciones',
                'Accesos rápidos a tus servicios',
                'applications'
            )
        );

        applications.forEach(w=>{
            g.appendChild(createWidgetCard(w,children));
        });
    }

    if(groups.length){
        g.appendChild(
            createSectionHeader(
                'Grupos',
                '',
                'groups'
            )
        );

        groups.forEach(w=>{
            g.appendChild(createWidgetCard(w,children));
        });
    }

    if(system.length){
        g.appendChild(
            createSectionHeader(
                'Sistema',
                'Estado y recursos del servidor',
                'system'
            )
        );

        system.forEach(w=>{
            g.appendChild(createWidgetCard(w,children));
        });
    }

    if(!visible.length){
        const empty=document.createElement('div');

        empty.className='dashboardEmpty';
        empty.innerHTML=`
            <strong>Tu dashboard está vacío</strong>
            <span>Entra en Editar para agregar aplicaciones y widgets.</span>
        `;

        g.appendChild(empty);
    }
}
async function loadPages(){
    const j=await api('/api/v1/pages');
    const pages=j.pages||[];

    $('pages').innerHTML=pages.map(p=>`
        <button
            type="button"
            data-id="${p.id}"
            data-slug="${esc(p.slug)}"
            class="${p.slug===slug?'active':''}">
            ${esc(p.name)}
        </button>
    `).join('');

    const p=pages.find(x=>x.slug===slug);
    if(p)pageId=p.id;
}
async function refresh(){
    try{
        const j=await api(`/api/v1/runtime/page/${encodeURIComponent(slug)}?breakpoint=${bp()}`);

        widgets=j.widgets||[];
        pageId=j.page?.id||pageId;

        $('pageTitle').textContent=j.page?.name||slug;
        $('status').textContent='Conectado';

        render();
        loadTheme();

        document.querySelectorAll('#pages [data-slug]').forEach(button=>{
            button.classList.toggle('active',button.dataset.slug===slug);
        });
    }catch(e){
        $('status').textContent=e.message;
    }
}
function start(){clearInterval(timer);refresh();if(!editing&&!document.hidden)timer=setInterval(refresh,30000)}
$('pages').onclick=e=>{if(e.target.dataset.slug){slug=e.target.dataset.slug;pageId=Number(e.target.dataset.id);start()}};
function setEditing(enabled){
    editing=enabled;
    document.body.classList.toggle('editingMode',editing);
    $('editor').hidden=!editing;
    $('edit').textContent=editing?'Finalizar':'Editar';
}

$('edit').onclick=()=>{
    if(editing){
        widgets=clone(snapshot);
        setEditing(false);
        start();
        return;
    }

    snapshot=clone(widgets);
    undoStack=[];
    redoStack=[];

    setEditing(true);
    clearInterval(timer);
    render();
};

$('cancel').onclick=()=>{
    widgets=clone(snapshot);
    setEditing(false);
    start();
};

$('breakpoint').onchange=refresh;
$('undo').onclick=()=>{if(!undoStack.length)return;redoStack.push(clone(widgets));widgets=undoStack.pop();render()};$('redo').onclick=()=>{if(!redoStack.length)return;undoStack.push(clone(widgets));widgets=redoStack.pop();render()};
$('grid').addEventListener('pointerdown',e=>{if(!editing)return;if(e.target.closest('.cardEditTools,button,a,input,select,textarea'))return;const card=e.target.closest('.card[data-id]');if(!card)return;const w=widgets.find(x=>String(x.id)===card.dataset.id);if(!w)return;pushUndo();const l=w.layout;drag={w,card,sx:e.clientX,sy:e.clientY,start:clone(l),resize:!!e.target.dataset.resize};card.classList.add('dragging');card.setPointerCapture(e.pointerId)});
$('grid').addEventListener('pointermove',e=>{if(!drag)return;const rect=$('grid').getBoundingClientRect(),cw=rect.width/cols[bp()],rh=64+parseFloat(getComputedStyle($('grid')).gap||0),dx=Math.round((e.clientX-drag.sx)/cw),dy=Math.round((e.clientY-drag.sy)/rh),l=drag.w.layout,c=cols[bp()];if(drag.resize){l.w=Math.max(1,Math.min(c-drag.start.x,drag.start.w+dx));l.h=Math.max(1,drag.start.h+dy)}else{l.x=Math.max(0,Math.min(c-l.w,drag.start.x+dx));l.y=Math.max(0,drag.start.y+dy)}render()});
$('grid').addEventListener('pointerup',e=>{if(drag){drag.card.classList.remove('dragging');drag=null}});
$('grid').addEventListener('click',async e=>{const b=e.target.closest('button[data-a]');if(!b)return;const w=widgets.find(x=>String(x.id)===b.closest('.card').dataset.id),a=b.dataset.a;try{if(a==='props')return properties(w);if(a==='duplicate')await api(`/api/v1/widgets/${w.id}/duplicate`,{method:'POST'});if(a==='hide')await api(`/api/v1/widgets/${w.id}/hide`,{method:'POST'});if(a==='delete'&&confirm('ÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã‚Â¢ÃƒÂ¢Ã¢â‚¬Å¡Ã‚Â¬Ãƒâ€¦Ã‚Â¡ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¿Eliminar este widget?'))await api(`/api/v1/widgets/${w.id}`,{method:'DELETE'});await refresh()}catch(x){alert(x.message)}});
function properties(w){
    const groups=widgets.filter(x=>x.type==='layout.group'&&x.id!==w.id);
    const app=w.application||null;
    $('modalBody').innerHTML=`
        <h2>Editar</h2>
        <form id="propForm" class="applicationForm">
            <label><span>Título de la tarjeta</span><input name="title" value="${esc(w.title||'')}" required></label>
            <label><span>Subtítulo</span><input name="subtitle" value="${esc(w.subtitle||'')}"></label>
            ${app?`
                <label><span>Nombre de la aplicación</span><input name="app_name" value="${esc(app.name||'')}" required></label>
                <label><span>URL</span><input name="app_url" type="url" value="${esc(app.url||'')}" required></label>
                <label><span>Descripción</span><textarea name="app_description" rows="3">${esc(app.description||'')}</textarea></label>
                <label><span>Icono</span><input name="icon_value" value="${esc(app.icon_value||'')}" placeholder="Ej. jellyfin, portainer o imagen Docker"></label>
                <label><span>Abrir</span><select name="open_mode"><option value="new_tab" ${app.open_mode!=='same'?'selected':''}>Nueva pestaña</option><option value="same" ${app.open_mode==='same'?'selected':''}>Misma pestaña</option></select></label>
            `:''}
            <label><span>Grupo</span><select name="parent"><option value="0">Ninguno</option>${groups.map(g=>`<option value="${g.id}" ${w.parent_widget_id===g.id?'selected':''}>${esc(g.title)}</option>`).join('')}</select></label>
            ${['information.iframe','information.json'].includes(w.type)?`<label><span>Configuración JSON</span><textarea name="config" rows="6">${esc(JSON.stringify(w.config||{},null,2))}</textarea></label>`:''}
            <div class="formActions"><button type="button" onclick="document.getElementById('modal').close()">Cancelar</button><button class="primaryButton" type="submit">Guardar cambios</button></div>
        </form>`;
    $('modal').showModal();
    $('propForm').onsubmit=async e=>{
        e.preventDefault();
        const f=new FormData(e.target);
        let config=w.config||{};
        try{
            if(f.get('config'))config=JSON.parse(f.get('config'));
            if(app){
                await api('/api/v1/applications',{method:'PUT',body:JSON.stringify({
                    id:Number(app.id),name:f.get('app_name'),url:f.get('app_url'),
                    description:f.get('app_description'),icon_type:'auto',
                    icon_value:f.get('icon_value'),open_mode:f.get('open_mode')
                })});
            }
            await api(`/api/v1/widgets/${w.id}`,{method:'PUT',body:JSON.stringify({
                title:f.get('title'),subtitle:f.get('subtitle'),
                parent_widget_id:Number(f.get('parent')),config
            })});
            $('modal').close();
            $('status').textContent='Cambios guardados';
            await refresh();
        }catch(x){alert(x.message)}
    };
}
$('save').onclick=async()=>{
    try{
        await api('/api/v1/layouts',{
            method:'PUT',
            body:JSON.stringify(
                widgets.map(w=>({
                    widget_id:w.id,
                    breakpoint:bp(),
                    x:w.layout.x,
                    y:w.layout.y,
                    w:w.layout.w,
                    h:w.layout.h
                }))
            )
        });

        snapshot=clone(widgets);
        $('status').textContent='Guardado';

        setEditing(false);
        start();
    }catch(e){
        $('status').textContent=e.message;
    }
};
$('addPage').onclick=async()=>{const name=prompt('Nombre de la nueva pÃƒÆ’Ã†â€™Ãƒâ€ Ã¢â‚¬â„¢ÃƒÆ’Ã¢â‚¬Â ÃƒÂ¢Ã¢â€šÂ¬Ã¢â€žÂ¢ÃƒÆ’Ã†â€™ÃƒÂ¢Ã¢â€šÂ¬Ã…Â¡ÃƒÆ’Ã¢â‚¬Å¡Ãƒâ€šÃ‚Â¡gina');if(!name)return;const j=await api('/api/v1/pages',{method:'POST',body:JSON.stringify({name})});slug=j.slug;await loadPages();start()};
$('store').onclick=async()=>{
    $('modalBody').innerHTML=`
        <div class="addPanel">
            <div class="addPanelHeader">
                <div>
                    <h2>Agregar al dashboard</h2>
                    <p>Agrega una aplicación o un widget de información.</p>
                </div>
            </div>

            <div class="addTypeSelector">
                <button
                    type="button"
                    class="addType active"
                    data-add-view="application">
                    <strong>Aplicación</strong>
                    <span>Acceso directo a un servicio o página web</span>
                </button>

                <button
                    type="button"
                    class="addType"
                    data-add-view="widget">
                    <strong>Widget</strong>
                    <span>Información y métricas del sistema</span>
                </button>
            </div>

            <section id="applicationAddView" class="addView">
                <form id="applicationForm" class="applicationForm">
                    <label>
                        <span>Nombre</span>
                        <input
                            id="applicationName"
                            type="text"
                            maxlength="100"
                            placeholder="Ej. Portainer"
                            required>
                    </label>

                    <label>
                        <span>URL</span>
                        <input
                            id="applicationURL"
                            type="url"
                            placeholder="https://..."
                            required>
                    </label>

                    <label>
                        <span>Descripción</span>
                        <textarea
                            id="applicationDescription"
                            maxlength="250"
                            rows="3"
                            placeholder="Descripción opcional"></textarea>
                    </label>

                    <div id="applicationFormError" class="formError" hidden></div>

                    <div class="formActions">
                        <button
                            type="button"
                            class="secondary"
                            data-close-add>
                            Cancelar
                        </button>

                        <button
                            type="submit"
                            class="primary">
                            Agregar aplicación
                        </button>
                    </div>
                </form>
            </section>

            <section id="widgetAddView" class="addView" hidden>
                <div id="widgetCatalog" class="widgetCatalog">
                    <div class="loadingState">Cargando widgets...</div>
                </div>
            </section>
        </div>
    `;

    $('modal').showModal();

    try{
        const catalog=await api('/api/v1/widgets/catalog');

        $('widgetCatalog').innerHTML=(catalog.widgets||[]).map(x=>`
            <button
                type="button"
                class="widgetChoice storeItem"
                data-type="${esc(x.type)}">
                <strong>${esc(x.name)}</strong>
                <span>${esc(x.category||'Widget')}</span>
            </button>
        `).join('')||`
            <div class="emptyState">
                No hay widgets disponibles.
            </div>
        `;
    }catch(e){
        $('widgetCatalog').innerHTML=`
            <div class="formError">
                ${esc(e.message)}
            </div>
        `;
    }
};
$('modal').addEventListener('click',e=>{
    const viewButton=e.target.closest('[data-add-view]');
    if(viewButton){
        const view=viewButton.dataset.addView;
        document.querySelectorAll('.addType').forEach(b=>b.classList.toggle('active',b===viewButton));
        const appView=$('applicationAddView');
        const widgetView=$('widgetAddView');
        if(appView) appView.hidden=view!=='application';
        if(widgetView) widgetView.hidden=view!=='widget';
        return;
    }

    if(e.target.closest('[data-close-add]')){
        $('modal').close();
    }
});

$('modal').addEventListener('submit',async e=>{
    if(e.target.id!=='applicationForm')
        return;

    e.preventDefault();

    const submit=e.target.querySelector('[type="submit"]');
    const error=$('applicationFormError');
    if(submit) submit.disabled=true;
    if(error) error.hidden=true;

    try{
        const app=await api('/api/v1/applications',{
            method:'POST',
            body:JSON.stringify({
                Name:$('applicationName').value.trim(),
                URL:$('applicationURL').value.trim(),
                Description:$('applicationDescription').value.trim()
            })
        });

        await api('/api/v1/widgets',{
            method:'POST',
            body:JSON.stringify({
                page_id:pageId,
                type:'application.shortcut',
                title:$('applicationName').value.trim(),
                application_id:Number(app.id)
            })
        });

        $('modal').close();
        $('status').textContent='Aplicación agregada';
        await refresh();
    }catch(err){
        if(error){
            error.textContent=err.message;
            error.hidden=false;
        }
        if(submit) submit.disabled=false;
    }
});

$('modal').addEventListener('click',async e=>{
    const widget=e.target.closest('.storeItem');

    if(!widget)
        return;

    if(!widget.dataset.type)
        return;

    widget.disabled=true;

    try{
        await api('/api/v1/widgets',{
            method:'POST',
            body:JSON.stringify({
                page_id:pageId,
                type:widget.dataset.type,
                title:''
            })
        });

        $('modal').close();
        $('status').textContent='Widget agregado';

        await refresh();

    }catch(err){
        widget.disabled=false;
        $('status').textContent=err.message;
    }
});
$('closeModal').onclick=()=>$('modal').close();
$('search').oninput=e=>{const q=e.target.value.trim().toLowerCase(),box=$('searchResults');if(!q){box.hidden=true;return}const hits=widgets.filter(w=>(w.title||w.type).toLowerCase().includes(q));box.innerHTML=hits.map(w=>`<button class="searchHit" data-id="${w.id}">${esc(w.title||w.type)}</button>`).join('')||'Sin resultados';box.hidden=false};$('searchResults').onclick=e=>{const b=e.target.closest('[data-id]');if(!b)return;document.querySelector(`.card[data-id="${b.dataset.id}"]`)?.scrollIntoView({behavior:'smooth',block:'center'})};
$('kiosk').onclick=()=>{document.body.classList.toggle('kiosk');history.replaceState(null,'',document.body.classList.contains('kiosk')?'?kiosk=1':location.pathname)};if(new URLSearchParams(location.search).get('kiosk')==='1')document.body.classList.add('kiosk');
document.addEventListener('visibilitychange',()=>document.hidden?clearInterval(timer):start());
function applyTheme(t){const v=t?.values||{},r=document.documentElement;for(const [k,css] of Object.entries({bg:'--bg',surface:'--surface',text:'--text',muted:'--muted',accent:'--accent',border:'--border',radius:'--radius',gap:'--gap',padding:'--card-padding',blur:'--card-blur',overlay:'--overlay'}))if(v[k]!=null)r.style.setProperty(css,v[k]);if(v.card_opacity!=null)r.style.setProperty('--card-opacity',v.card_opacity);if(v.background_image)r.style.setProperty('--bg-image',`url("${String(v.background_image).replace(/["\\]/g,'')}")`)}async function loadTheme(){try{applyTheme((await api(`/api/v1/themes?page=${encodeURIComponent(slug)}`)).theme)}catch{}}
$('theme').onclick=async()=>{const ps=await api('/api/v1/themes/presets');$('modalBody').innerHTML=`<h2>Apariencia</h2>${Object.entries(ps.presets||{}).map(([k,x])=>`<button class="preset" data-preset="${k}">${esc(x.name)}</button>`).join('')}`;$('modal').showModal();$('modalBody').querySelectorAll('.preset').forEach(b=>b.onclick=async()=>{const x=ps.presets[b.dataset.preset];await api(`/api/v1/themes?page=${encodeURIComponent(slug)}`,{method:'PUT',body:JSON.stringify(x)});$('modal').close();loadTheme()})};
$('discover').onclick=async()=>{
    try{
        const j=await api('/api/v1/discovery/docker');
        const services=j.services||[];
        $('modalBody').innerHTML=`
            <div class="discoveryHeader">
                <div><h2>Contenedores Docker</h2><p>Elige los servicios que quieres mostrar en tu dashboard.</p></div>
                <span class="discoveryCount">${services.length}</span>
            </div>
            <div class="discoveryList">
            ${services.map((s,i)=>{
                const hint=String(s.suggested_icon||s.image||s.suggested_name||'').toLowerCase();
                const slug=hint.replace(/^.*\\//,'').replace(/[:@].*$/,'').replace(/[^a-z0-9]+/g,'-').replace(/^-|-$/g,'');
                const icon=slug?'https://cdn.jsdelivr.net/gh/homarr-labs/dashboard-icons/svg/'+encodeURIComponent(slug)+'.svg':'';
                const initial=String(s.suggested_name||s.service||s.container_name||'D').charAt(0).toUpperCase();
                return `<article class="discoveryCard">
                    <div class="discoveryIcon">${icon?`<img src="${esc(icon)}" alt="" loading="lazy" data-icon-fallback="docker" data-fallback="${esc(initial)}">`:esc(initial)}</div>
                    <div class="discoveryInfo">
                        <strong>${esc(s.suggested_name||s.service||s.container_name||'Servicio')}</strong>
                        <span>${esc(s.image||'')}</span>
                        <small><i class="stateDot ${String(s.state||'').toLowerCase()}"></i>${esc(s.state||'desconocido')}${s.suggested_url?' · '+esc(s.suggested_url):''}</small>
                    </div>
                    <button class="discoveryAdd primaryButton" data-key="${esc(s.stable_key)}" ${!s.suggested_url?'disabled title="Este contenedor no publica un puerto HTTP utilizable"':''}>Agregar</button>
                </article>`;
            }).join('')||'<div class="dashboardEmpty"><strong>No se encontraron contenedores</strong><span>Verifica la conexión con Docker.</span></div>'}
            </div>`;
        $('modal').showModal();
        $('modalBody').querySelectorAll('.discoveryAdd').forEach(b=>b.onclick=async()=>{
            b.disabled=true;b.textContent='Agregando...';
            try{
                await api('/api/v1/discovery/adopt',{method:'POST',body:JSON.stringify({stable_key:b.dataset.key,page_id:pageId})});
                b.textContent='Agregado'; b.classList.add('added');
                $('status').textContent='Contenedor agregado';
                await refresh();
            }catch(err){b.disabled=false;b.textContent='Agregar';alert(err.message)}
        });
    }catch(e){alert(e.message)}
};
document.addEventListener('error',e=>{
    const img=e.target;
    if(!(img instanceof HTMLImageElement)||!img.dataset.iconFallback)return;
    const parent=img.parentElement;
    img.remove();
    if(img.dataset.iconFallback==='application'){
        const fallback=parent?.querySelector('.applicationFallback');
        if(fallback)fallback.hidden=false;
    }else if(parent){
        parent.textContent=img.dataset.fallback||'D';
    }
},true);
loadPages().then(start);
