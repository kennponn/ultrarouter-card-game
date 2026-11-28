/* The atlas lists implemented game heroes; ownership and fragments come from SAVE. */
const CODEX_ROLES = {tank:"守护",sdps:"单体输出",adps:"群体输出",heal:"治疗",support:"辅助"};
const CODEX_TABS = {skills:"卡牌技能",story:"人物档案",bonds:"人物羁绊",sources:"收录与获取"};
let codexState = null;
let codexDialogState = null;

function currentCodex() {
  if (!codexState || codexState.epoch !== Store.sessionEpoch || codexState.owner !== Store.authUserId) {
    closeCodexDialogs(false);
    codexState = {epoch:Store.sessionEpoch,owner:Store.authUserId,id:GIRLS[0]?.id || "",query:"",rarity:"all",role:"all",status:"all",tab:"skills",mobileDetail:false,busy:false,syncing:false,error:""};
  }
  return codexState;
}

function codexOwnership(girl) {
  const own = SAVE?.girls?.[girl.id], owned = Boolean(own?.star > 0), shards = own?.shards || 0, need = RARITY[girl.rarity].shardToUnlock;
  return {owned,shards,need,status:owned ? "owned" : shards >= need ? "ready" : shards > 0 ? "shards" : "missing"};
}

function codexLore(id) { return GIRLS.find(girl => girl.id === id)?.atlas || (typeof CODEX_LORE === "undefined" ? {} : CODEX_LORE[id] || {}); }

function codexMatches() {
  const state = currentCodex(), query = state.query.trim().toLocaleLowerCase();
  return GIRLS.filter(girl => {
    const own = codexOwnership(girl), lore = codexLore(girl.id);
    const status = state.status === "all" || state.status === "owned" && own.owned || state.status === "missing" && !own.owned || state.status === "shards" && !own.owned && own.shards > 0 || state.status === "ready" && own.status === "ready";
    return status && (state.rarity === "all" || girl.rarity === state.rarity) && (state.role === "all" || girl.role === state.role) && (!query || [girl.name,girl.desc,lore.title,lore.faction,lore.address,girl.skills.normal.name,girl.skills.skill.name,...(lore.abilities || []).map(entry => entry.name)].join(" ").toLocaleLowerCase().includes(query));
  });
}

function openCodex(id) {
  if (!requireGame() || id && !GIRLS.some(girl => girl.id === id)) return;
  const state = currentCodex();
  if (id) { state.id=id; state.mobileDetail=true; state.query=""; state.rarity=state.role=state.status="all"; }
  closeModal();
  nav("page-codex");
  if (id) document.getElementById("codex-detail")?.focus({preventScroll:true});
}

function selectCodexHero(id) {
  if (!requireGame() || !GIRLS.some(girl => girl.id === id)) return;
  closeCodexDialogs(false);
  const state = currentCodex();
  state.id=id; state.mobileDetail=true;
  renderCodex();
  const detail = document.getElementById("codex-detail");
  if (detail) { detail.scrollTop=0; detail.focus({preventScroll:true}); }
}

function renderCodex() {
  if (!requireGame()) return;
  const root = document.getElementById("codex-root");
  if (!root) return;
  const state = currentCodex();
  if (root.dataset.session !== `${state.epoch}:${state.owner}` || !root.querySelector("#codex-grid")) {
    root.dataset.session = `${state.epoch}:${state.owner}`;
    root.innerHTML = `<aside class="cx-roster" aria-label="人物名录"><header class="cx-roster-head"><button type="button" class="cx-back" data-codex-action="squad" aria-label="返回编队">‹</button><div><h2>人物名录</h2><small>群星档案 · 星霜少女</small></div><button type="button" id="codex-sync" class="cx-back" data-codex-action="sync" aria-label="同步角色与碎片" title="同步角色与碎片">↻</button></header>
      <label class="cx-search"><span aria-hidden="true">⌕</span><input id="codex-search" type="search" placeholder="搜索姓名、技能或地域" aria-label="搜索人物" autocomplete="off"></label>
      <div class="cx-rarity" role="group" aria-label="稀有度筛选">${["all", "SSR", "SR", "R"].map(rarity => `<button type="button" data-codex-rarity="${rarity}" aria-pressed="false">${rarity === "all" ? "全部" : rarity}</button>`).join("")}</div>
      <div class="cx-filters"><select id="codex-role" aria-label="战斗定位"><option value="all">全部定位</option>${Object.entries(CODEX_ROLES).map(([value,label]) => `<option value="${value}">${label}</option>`).join("")}</select><select id="codex-status" aria-label="获取状态"><option value="all">全部状态</option><option value="owned">已获得</option><option value="missing">未获得</option><option value="shards">已有碎片</option><option value="ready">可合成</option></select></div>
      <div class="cx-roster-tools"><span id="codex-filter-summary"></span><button type="button" data-codex-action="reset">重置筛选</button></div><p id="codex-count" role="status"></p><p id="codex-status-message" role="status" hidden></p><div id="codex-grid" class="cx-grid" aria-label="英雄列表"></div><footer class="cx-roster-footer">✧ 每一颗星，都有自己的来处。</footer></aside><section id="codex-detail" class="cx-detail" aria-label="人物详情" tabindex="-1"></section>`;
    root.onclick=codexClick;
    root.oninput=event => { if (event.target.id === "codex-search") { currentCodex().query=event.target.value; renderCodexRoster(); } };
    root.onchange=event => {
      const key = {"codex-role":"role","codex-status":"status"}[event.target.id];
      if (key) { currentCodex()[key]=event.target.value; renderCodexRoster(); }
    };
    root.onkeydown=codexKeydown;
  }
  root.dataset.mobileDetail=String(state.mobileDetail);
  renderCodexRoster();
  renderCodexDetail();
}

function renderCodexRoster() {
  if (!Store.ready()) return;
  const state = currentCodex(), grid = document.getElementById("codex-grid");
  if (!grid) return;
  const heroes=codexMatches(), owned=GIRLS.filter(girl => codexOwnership(girl).owned).length;
  const focused = grid.contains(document.activeElement) ? document.activeElement.dataset.codexId : "";
  document.getElementById("codex-search").value=state.query;
  document.getElementById("codex-role").value=state.role;
  document.getElementById("codex-status").value=state.status;
  document.querySelectorAll("[data-codex-rarity]").forEach(button => button.setAttribute("aria-pressed",String(button.dataset.codexRarity===state.rarity)));
  document.getElementById("codex-filter-summary").textContent=`当前显示 ${heroes.length} 位`;
  document.getElementById("codex-count").textContent=`已获得 ${owned} / ${GIRLS.length} · ${GIRLS.filter(girl => codexOwnership(girl).status === "ready").length} 位可合成`;
  const statusMessage=document.getElementById("codex-status-message");
  statusMessage.textContent=state.error || (state.syncing ? "正在同步角色与碎片…" : "");
  statusMessage.hidden=!statusMessage.textContent;
  const sync = document.getElementById("codex-sync");
  sync.disabled=state.syncing || state.busy;
  sync.setAttribute("aria-label",state.syncing ? "正在同步" : "同步角色与碎片");
  grid.innerHTML=heroes.map(girl => {
    const own=codexOwnership(girl), label=own.owned ? "已获得" : own.status === "ready" ? "可合成" : "未获得";
    return `<button type="button" class="cx-card" data-codex-id="${girl.id}" data-rarity="${girl.rarity}" data-owned="${own.owned}" data-status="${own.status}" aria-pressed="${state.id === girl.id}" aria-label="${esc(girl.name)}，${label}，碎片 ${own.shards}/${own.need}"><span class="cx-avatar"><img src="${esc(girl.img)}" alt="" loading="lazy"></span><b>${esc(girl.name)}</b><small>${girl.rarity} · ${CODEX_ROLES[girl.role]}</small><span class="cx-card-state">${label}${!own.owned && own.shards ? `<span>${own.shards}/${own.need} 碎片</span>` : ""}</span></button>`;
  }).join("") || '<div class="cx-empty"><span>✧</span><p>没有符合条件的伙伴</p><button type="button" data-codex-action="reset">清除筛选</button></div>';
  if (focused) grid.querySelector(`[data-codex-id="${focused}"]`)?.focus({preventScroll:true});
}

function codexParagraphs(value) { return String(value || "").split(/\n+/).filter(Boolean).map(part=>`<p>${esc(part)}</p>`).join(""); }

function codexHeading(title, eyebrow, note = "") {
  return `<div class="cx-section-heading"><div>${eyebrow ? `<span class="cx-eyebrow">${esc(eyebrow)}</span>` : ""}<h3>${esc(title)}</h3></div><small>${esc(note)}</small></div>`;
}

function codexSkillMarkup(skill, index, basic = false) {
  const kind = basic || /普攻|普通/.test(skill.type) ? "basic" : /必杀|终结|怒气/.test(skill.type) ? "ultimate" : /被动|特性/.test(skill.type) ? "passive" : "skill";
  const content = skill.text || "", long = content.length > 280;
  let intro = content.split(/\n+/).filter(Boolean)[0] || "";
  if (intro.length > 280) intro=intro.slice(0,240)+"…";
  return `<article class="cx-skill-card" data-kind="${kind}"><div class="cx-skill-heading"><span class="cx-skill-index" aria-hidden="true">${basic ? "◇" : String(index+1).padStart(2,"0")}</span><div class="cx-skill-title"><small>${esc(skill.type || (basic ? "普攻" : "技能"))}</small><h4>${esc(skill.name)}</h4></div><button type="button" class="cx-preview-button" data-codex-preview="${basic ? "basic" : index}" aria-label="演示${esc(skill.name)}"><span aria-hidden="true">▷</span> 演示</button></div>
    <div class="cx-skill-meta">${skill.target ? `<span>${esc(skill.target)}</span>` : ""}${Number(skill.rage)>0 ? `<span>${skill.rage} 怒气</span>` : ""}${Number(skill.cooldown)>0 ? `<span>冷却 ${skill.cooldown} 次行动</span>` : ""}${kind === "passive" ? "<span>条件触发</span>" : ""}</div><div class="cx-skill-text">${codexParagraphs(long ? intro : content)}</div>
    ${long ? `<details class="cx-skill-rules"><summary>展开完整结算规则</summary>${codexParagraphs(content)}</details>` : ""}${skill.upgrade ? `<div class="cx-skill-enhancement"><strong>✧ 星霜演化</strong>${codexParagraphs(skill.upgrade)}</div>` : ""}${skill.visual ? `<p class="cx-skill-visual"><span>施放意象</span>${esc(skill.visual)}</p>` : ""}</article>`;
}

function codexSkillsTab(lore) {
  const skills=lore.skills || [], stars=lore.highStars || [];
  const separate=lore.basic && !skills.some(skill => skill.id === lore.basic.id);
  return `${codexHeading("把本领，写成星光","THE ART OF STARS",`${skills.length} 项技能 · ${separate ? "另含独立普攻" : "含基础普攻"}`)}<div class="cx-skill-list">${separate ? codexSkillMarkup(lore.basic,-1,true) : ""}${skills.map((skill,index)=>codexSkillMarkup(skill,index)).join("")}</div>
    ${stars.length ? `${codexHeading("星阶共鸣","",lore.implementationNote ? "设计预览 · 尚未启用" : "依培养阶段解锁")}<div class="cx-awakening-grid">${stars.map(step=>`<article class="cx-awakening-card" data-star-milestone="${step.star}"><span class="cx-star-level">${step.star} ✦</span><h4>${esc(step.name)}</h4>${codexParagraphs(step.text)}</article>`).join("")}</div>` : ""}
    ${lore.counter ? `<article class="cx-story-card cx-combat-note"><span class="cx-eyebrow">知其所长，也知其所限</span><h3>应对与边界</h3>${codexParagraphs(lore.counter)}</article>` : ""}
    <div class="cx-pair-note"><span aria-hidden="true">✧</span><p>${esc(lore.implementationNote || "演示展现招式的施放意象；具体目标、费用与次数以技能条目为准。星霜演化是此图鉴的融合设计，尚未接入实战结算。")}</p></div>`;
}

function codexStoryTab(lore) {
  const stats=lore.stats || {}, fields=[["hp","生命",2000],["speed","速度",150],["atk","攻击",500],["def","防御",130],["spAtk","特攻",220],["spDef","特防",130]];
  return `${codexHeading("此身有来处","A LIFE AMONG STARS",`档案 ${String(lore.number).padStart(3,"0")}`)}<div class="cx-stats-grid">${fields.map(([key,label,max])=>`<div class="cx-stat-item"><label>${label}</label><strong>${esc(stats[key] ?? "—")}</strong><span class="cx-stat-track"><i style="width:${Math.min(100,(Number(stats[key]) || 0)/max*100)}%"></i></span></div>`).join("")}</div><p class="cx-stat-note">${esc(lore.statNote || "")}</p>
    <div class="cx-story-grid"><article class="cx-story-card"><span class="cx-eyebrow">01 / 来时的路</span><h3>生平</h3>${codexParagraphs(lore.biography)}</article><article class="cx-story-card cx-event-card"><span class="cx-eyebrow">02 / 命运的回声</span><h3>那一次选择</h3>${codexParagraphs(lore.event)}</article></div>
    <details class="cx-appearance"><summary>形貌与随身物件</summary>${codexParagraphs(lore.appearance)}</details>${codexHeading("四项本领","",lore.origin || "")}<div class="cx-ability-grid">${(lore.abilities || []).map((ability,index)=>`<article class="cx-ability-card"><span>${String(index+1).padStart(2,"0")}</span><h4>${esc(ability.name)}</h4>${codexParagraphs(ability.description)}</article>`).join("")}</div>`;
}

function closeCodexDialogs(restoreFocus = true) {
  const view=codexDialogState;
  if (!view) return;
  codexDialogState=null;
  if (view.preview) window.CodexEffects?.stop();
  if (view.dialog.open && typeof view.dialog.close === "function") view.dialog.close();
  view.dialog.remove();
  if (restoreFocus && Store.ready() && view.epoch === Store.sessionEpoch && view.owner === Store.authUserId && document.body.dataset.page === "page-codex") {
    if (view.opener?.isConnected) view.opener.focus({preventScroll:true});
    else document.getElementById("codex-detail")?.focus({preventScroll:true});
  }
}

function codexDialog(kind, labelledBy, markup) {
  closeCodexDialogs(false);
  const dialog=document.createElement("dialog");
  dialog.id=`codex-${kind}-dialog`; dialog.className=`cx-${kind}-dialog`;
  dialog.setAttribute("aria-labelledby",labelledBy);
  dialog.innerHTML=`<button type="button" class="cx-dialog-close" data-codex-close aria-label="${kind === "world" ? "关闭百骸洲纪事" : "关闭技能演示"}">×</button>${markup}`;
  const view={dialog,epoch:Store.sessionEpoch,owner:Store.authUserId,opener:document.activeElement,preview:kind === "preview"};
  codexDialogState=view;
  dialog.querySelector("[data-codex-close]").onclick=()=>closeCodexDialogs();
  dialog.addEventListener("cancel",event=>{event.preventDefault(); closeCodexDialogs();});
  dialog.addEventListener("close",()=>{if (codexDialogState === view) closeCodexDialogs();});
  document.body.append(dialog);
  if (typeof dialog.showModal === "function") dialog.showModal();
  else dialog.setAttribute("open","");
  dialog.querySelector("[data-codex-close]").focus({preventScroll:true});
  return view;
}

function openCodexWorld() {
  if (!requireGame() || document.body.dataset.page !== "page-codex" || typeof CODEX_WORLD_HTML === "undefined") return;
  codexDialog("world","cx-world-title",CODEX_WORLD_HTML);
}

function openCodexPreview(key) {
  if (!requireGame() || document.body.dataset.page !== "page-codex") return;
  const state=currentCodex(), girl=GIRLS.find(hero=>hero.id === state.id), lore=codexLore(state.id);
  const skill=key === "basic" ? lore.basic : lore.skills?.[Number(key)];
  if (!girl || !skill) return;
  const view=codexDialog("preview","codex-preview-title",`<header class="cx-preview-heading"><p class="cx-eyebrow">A MOMENT OF STARRIME</p><h2 id="codex-preview-title">${esc(skill.name)}</h2><p>${esc(girl.name)} / ${esc(skill.type || "技能")} · ${esc(skill.target || "施放意象")}</p></header><div class="cx-preview-stage"><canvas id="codex-effect-canvas" aria-label="技能施放视觉演示"></canvas><div id="codex-effect-caption" class="cx-effect-caption" aria-live="polite"></div><span class="cx-stage-label" aria-hidden="true">✦ SKILL PREVIEW</span></div><div class="cx-preview-controls"><button type="button" class="cx-primary-button" data-codex-preview-control="replay"><span aria-hidden="true">↻</span> 再次施放</button><button type="button" class="cx-secondary-button" data-codex-preview-control="pause" aria-pressed="false">暂停</button><label class="cx-speed-control"><span>播放速度</span><select data-codex-preview-speed aria-label="技能演示播放速度"><option value="0.65">0.65×</option><option value="1" selected>1×</option><option value="1.5">1.5×</option></select></label></div><p class="cx-preview-mechanic">${esc(skill.visual || `${girl.name}施展${skill.name}。`)}</p><p class="cx-preview-note">视觉演示用于呈现技能意象；具体效果请以技能条目为准。</p>`);
  const {dialog}=view, pause=dialog.querySelector('[data-codex-preview-control="pause"]'), speed=dialog.querySelector("[data-codex-preview-speed]"), caption=dialog.querySelector("#codex-effect-caption");
  const valid=()=>codexDialogState === view && Store.ready() && view.epoch === Store.sessionEpoch && view.owner === Store.authUserId && document.body.dataset.page === "page-codex";
  const play=()=>{
    if (!valid()) { if (codexDialogState === view) closeCodexDialogs(false); return; }
    pause.disabled=false; pause.textContent="暂停"; pause.setAttribute("aria-pressed","false");
    if (!window.CodexEffects) { caption.textContent="此浏览器暂不能播放演示，请阅读下方施放意象。"; pause.disabled=true; return; }
    window.CodexEffects.play(dialog.querySelector("canvas"),{card:{...lore,id:girl.id},skill,caption,speed:Number(speed.value),onEnd(){if (valid()) {pause.textContent="已播放";pause.disabled=true;}}});
  };
  dialog.querySelector('[data-codex-preview-control="replay"]').onclick=play;
  pause.onclick=()=>{
    if (!valid()) { if (codexDialogState === view) closeCodexDialogs(false); return; }
    const paused=window.CodexEffects?.togglePause();
    pause.textContent=paused ? "继续" : "暂停"; pause.setAttribute("aria-pressed",String(!!paused));
  };
  speed.onchange=()=>{if (valid()) window.CodexEffects?.setSpeed(Number(speed.value));};
  play();
}

function codexTabMarkup(girl,state) {
  const lore=codexLore(girl.id);
  if (state.tab === "story") return codexStoryTab(lore);
  if (state.tab === "bonds") {
    const rules=typeof cultivationRules === "function" ? cultivationRules() : null;
    const bonds=rules && typeof collectionBonds === "function" ? collectionBonds(girl.id) : [];
    const links=(lore.storyLinks || []).filter(link=>GIRLS.some(hero=>hero.id===link.targetId));
    const stories=links.length ? `${codexHeading("故事中的交集","THREADS THAT REMAIN","来自人物小传")}${links.map(link=>`<article class="cx-source-card"><span class="cx-eyebrow">${esc(link.type)}</span><h4>${esc(link.name)}</h4><p>${esc(link.text)}</p><div class="cx-story-links"><button type="button" data-codex-id="${link.targetId}">${esc(link.name)}的档案 ↗</button></div></article>`).join("")}` : "";
    return `${stories}<div class="cx-section-heading"><h3>收藏与战术共鸣</h3><small>获得整卡后激活</small></div><p>集齐同组伙伴即可激活，无需同时上阵；仅有碎片不计入已获得。</p>${bonds.length ? bonds.map(bond=>`<article class="cx-bond"><header><h4>${esc(bond.spec.name)}</h4><em>${bond.percent ? "已激活" : `待集齐 ${bond.owned}/${bond.spec.members.length}`}</em></header><div class="cx-bond-members">${bond.spec.members.map(id=>GIRLS.find(hero=>hero.id===id)).filter(Boolean).map(hero=>`<button type="button" data-codex-id="${hero.id}" data-owned="${codexOwnership(hero).owned}"><img src="${esc(hero.img)}" alt="" loading="lazy"><b>${esc(hero.name)}</b><small>${codexOwnership(hero).owned ? "已获得" : "未获得"}</small></button>`).join("")}</div><p>当前${esc(CULTIVATION_STATS[bond.spec.stat] || bond.spec.stat)} +${bond.percent}% · 集齐 +${bond.spec.base}% · 全员 ${rules.bond_star} 星 +${bond.spec.advanced}%</p></article>`).join("") : '<div class="cx-empty"><p>暂无可展示的羁绊资料。</p></div>'}${rules ? `<p class="cx-bond-total">图鉴常驻加成：${esc(codexSummary())}</p>` : ""}`;
  }
  if (state.tab === "sources") return `${codexHeading("循星寻人","WHERE OUR PATHS MEET",lore.group || "")}<article class="cx-source-card"><span class="cx-eyebrow">人物收录</span><h4>${esc(lore.group)} · 第 ${String(lore.number).padStart(3,"0")} 号</h4><p>${esc(girl.name)}，${esc(lore.title)}。门牌记作「${esc(lore.address || lore.faction)}」。</p></article><article class="cx-source-card"><span class="cx-eyebrow">获取途径</span><h4>${esc(girl.name)}</h4><p>${esc(cardSourceDescription(girl))}</p><button type="button" class="cx-primary" data-codex-action="source">前往获取途径 ›</button></article><article class="cx-source-card"><h4>同卡碎片合成</h4><p>集齐 ${RARITY[girl.rarity].shardToUnlock} 枚${esc(girl.name)}碎片后，可在本页主动合成。未合成前不加入编队；合成后可在编队中上阵。</p><p>已获得伙伴的剩余碎片可用于后续培养。</p></article><button type="button" class="cx-secondary" data-codex-action="world">读一页百骸洲纪事 ↗</button>`;
  return codexSkillsTab(lore);
}

function renderCodexDetail() {
  const state=currentCodex(), girl=GIRLS.find(hero=>hero.id===state.id), detail=document.getElementById("codex-detail");
  if (!girl || !detail) return;
  const own=codexOwnership(girl), lore=codexLore(girl.id), busy=state.busy || state.syncing;
  const summary=(lore.biography || "").split(/(?<=[。！？])/).slice(0,2).join("");
  const hadFocus=detail.contains(document.activeElement), scroll=detail.scrollTop, focusAction=hadFocus ? document.activeElement.dataset.codexAction : "", focusTab=document.activeElement?.dataset.codexTab;
  detail.dataset.hero=girl.id;
  detail.innerHTML=`<div class="cx-detail-top"><button type="button" class="cx-mobile-back" data-codex-action="roster">‹ 人物名录</button><span>群星档案 <b>NO. ${String(lore.number || Number(girl.id.slice(1))).padStart(3,"0")}</b></span><div class="cx-pager"><button type="button" data-codex-action="previous" aria-label="上一位英雄">‹</button><button type="button" data-codex-action="next" aria-label="下一位英雄">›</button></div></div>
    <div class="cx-hero" style="--cx-accent:${ELEM[girl.elem].color}"><div class="cx-visual"><div class="cx-orbit" aria-hidden="true"></div><img class="cx-portrait" src="${esc(girl.battleImg || girl.img)}" alt="${esc(girl.name)}立绘"><span class="cx-visual-caption">群星初见 · 旧梦新篇</span></div><div class="cx-intro"><div class="cx-kicker"><span class="cx-badge" data-rarity="${girl.rarity}">${girl.rarity}</span><span>${esc(lore.group || "初见群星")}</span><span class="cx-hero-serial">NO. ${String(lore.number).padStart(3,"0")}</span></div><h2 class="cx-name">${esc(girl.name)}</h2><p class="cx-title">${esc(lore.title || girl.desc)}</p><div class="cx-tags"><span>${esc(lore.roleLabel || CODEX_ROLES[girl.role])}</span>${lore.origin ? `<span>${esc(lore.origin.split(/[（(＋+]/)[0])}</span>` : ""}</div><blockquote>${esc(summary)}</blockquote><p class="cx-address"><span>落脚之处</span><strong>${esc(lore.address || lore.faction || "")}</strong></p></div></div>
    <section class="cx-acquisition" data-status="${own.status}" aria-label="获取与碎片进度"><div><b>${own.owned ? `已获得 · ${SAVE.girls[girl.id].star} 星` : own.status === "ready" ? "碎片已集齐 · 可合成" : "尚未获得"}</b><small>同卡碎片 <strong>${own.shards}/${own.need}</strong></small></div><div class="cx-shard-meter" role="progressbar" aria-label="同卡碎片" aria-valuemin="0" aria-valuemax="${own.need}" aria-valuenow="${Math.min(own.shards,own.need)}"><i style="width:${Math.min(100,own.shards/own.need*100)}%"></i></div><div class="cx-action-row">${own.owned ? '<button type="button" class="cx-primary" data-codex-action="workshop">角色培养</button><button type="button" class="cx-secondary" data-codex-action="squad">前往编队</button>' : `<button type="button" class="cx-primary" data-codex-action="unlock"${busy || own.status !== "ready" ? " disabled" : ""}>${state.busy ? "正在确认合成…" : own.status === "ready" ? `合成召唤 · ${own.need} 碎片` : `还需 ${own.need-own.shards} 枚碎片`}</button><button type="button" class="cx-secondary" data-codex-action="source">获取途径 ›</button>`}</div>${state.error ? `<p role="status">${esc(state.error)}</p>` : ""}</section>
    <div class="cx-tabs" role="tablist" aria-label="人物资料">${Object.entries(CODEX_TABS).map(([key,label],index)=>`<button type="button" id="codex-tab-${key}" role="tab" data-codex-tab="${key}" aria-selected="${state.tab === key}" aria-controls="codex-tab-content" tabindex="${state.tab === key ? 0 : -1}"><span class="cx-tab-index" aria-hidden="true">0${index+1}</span>${label}</button>`).join("")}</div><section id="codex-tab-content" class="cx-tab-content" role="tabpanel" aria-labelledby="codex-tab-${state.tab}">${codexTabMarkup(girl,state)}</section>`;
  detail.scrollTop=scroll;
  if (focusTab) detail.querySelector(`[data-codex-tab="${focusTab}"]`)?.focus({preventScroll:true});
  else if (focusAction) detail.querySelector(`[data-codex-action="${focusAction}"]:not(:disabled)`)?.focus({preventScroll:true});
  if (hadFocus && !detail.contains(document.activeElement) && (state.mobileDetail || !window.matchMedia?.("(max-width:760px)").matches)) detail.focus({preventScroll:true});
}

async function unlockCodexHero(id) {
  if (!requireGame()) return null;
  const state=currentCodex(), girl=GIRLS.find(hero=>hero.id===id);
  if (!girl || state.busy || state.syncing || codexOwnership(girl).status !== "ready") return null;
  state.busy=true; state.error="";
  if (document.body.dataset.page === "page-codex") renderCodex();
  try {
    const receipt=await Store.mutate("shard_unlock",{girl_id:id});
    if (state !== currentCodex() || !Store.ready()) return null;
    if (receipt) { renderTopbar(); toast((receipt.drops || []).join(" · ") || "合成已确认"); }
    else state.error="合成尚未确认，请查看提示后重试。";
    return receipt;
  } catch (error) {
    if (state === currentCodex() && Store.ready()) state.error=error.message || "合成失败，请稍后重试。";
    return null;
  } finally {
    state.busy=false;
    if (state === currentCodex() && Store.ready()) {
      if (document.body.dataset.page === "page-codex") renderCodex();
      else if (document.body.dataset.page === "page-squad") renderSquad();
    }
  }
}

async function syncCodex() {
  if (!requireGame()) return;
  const state=currentCodex();
  if (state.busy || state.syncing) return;
  state.syncing=true; state.error=""; renderCodex();
  try {
    if (!await Store.refresh() && state===currentCodex() && Store.ready()) state.error="暂时无法同步，请稍后重试。";
  } catch (error) {
    if (state===currentCodex() && Store.ready()) state.error=error.message || "暂时无法同步。";
  } finally {
    state.syncing=false;
    if (state===currentCodex() && Store.ready() && document.body.dataset.page === "page-codex") { renderCodex(); renderTopbar(); }
  }
}

function codexClick(event) {
  const button=event.target.closest("button");
  if (!button || button.disabled || !requireGame()) return;
  const state=currentCodex();
  if (button.hasAttribute("data-codex-preview")) return openCodexPreview(button.dataset.codexPreview);
  if (button.dataset.codexId) return selectCodexHero(button.dataset.codexId);
  if (button.dataset.codexRarity) { state.rarity=button.dataset.codexRarity; renderCodexRoster(); return; }
  if (button.dataset.codexTab) { state.tab=button.dataset.codexTab; renderCodexDetail(); document.getElementById(`codex-tab-${state.tab}`)?.focus({preventScroll:true}); return; }
  const action=button.dataset.codexAction;
  if (action === "unlock") return unlockCodexHero(state.id);
  if (action === "sync") return syncCodex();
  if (action === "source") return openCardSource(state.id);
  if (action === "world") return openCodexWorld();
  if (action === "squad") return nav("page-squad");
  if (action === "workshop" && SAVE.girls[state.id]?.star > 0) return openGirl(state.id);
  if (action === "reset") { state.query=""; state.rarity=state.role=state.status="all"; renderCodexRoster(); return; }
  if (action === "roster") {
    state.mobileDetail=false; document.getElementById("codex-root").dataset.mobileDetail="false";
    (document.querySelector(`#codex-grid [data-codex-id="${state.id}"]`) || document.getElementById("codex-search"))?.focus({preventScroll:true});
  }
  if (action === "previous" || action === "next") {
    const matches=codexMatches(), heroes=matches.length ? matches : GIRLS, index=heroes.findIndex(girl=>girl.id===state.id);
    const next=index < 0 ? (action === "next" ? 0 : heroes.length-1) : (index+(action === "next" ? 1 : -1)+heroes.length)%heroes.length;
    selectCodexHero(heroes[next].id);
  }
}

function codexKeydown(event) {
  if (!event.target.matches('[data-codex-tab]') || !["ArrowLeft","ArrowRight","Home","End"].includes(event.key)) return;
  event.preventDefault();
  const state=currentCodex(), keys=Object.keys(CODEX_TABS), index=keys.indexOf(state.tab);
  state.tab=keys[event.key === "Home" ? 0 : event.key === "End" ? keys.length-1 : (index+(event.key === "ArrowRight" ? 1 : -1)+keys.length)%keys.length];
  renderCodexDetail(); document.getElementById(`codex-tab-${state.tab}`)?.focus({preventScroll:true});
}
