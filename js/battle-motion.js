/* Presentation only. The report owns targets, damage, order and final snapshots. */
(function (root) {
  "use strict";
  const active = new Set();
  const frame = callback => root.requestAnimationFrame(callback);
  const stopFrame = id => root.cancelAnimationFrame(id);
  const clamp = (value, lo, hi) => Math.min(hi, Math.max(lo, value));
  const ease = value => 1 - Math.pow(1 - value, 3);

  function current(battle) {
    if (!battle || battle.abort || battle.skip || battle.settled) return false;
    if (typeof root.battleCurrent === "function") return root.battleCurrent(battle);
    return !root.Store || (battle.epoch === root.Store.sessionEpoch && root.Store.ready());
  }
  function cancelBattleMotion(battle) {
    for (const action of [...active]) if (!battle || action.battle === battle) action.cancel();
  }
  function findUnit(uid) { return document.getElementById("u-" + uid); }

  function createAction(battle, source, event) {
    const scene = document.getElementById("battle-scene");
    const sourceNode = findUnit(source?.uid);
    const sourceBody = sourceNode?.querySelector(".u-body");
    const reduced = Boolean(root.matchMedia?.("(prefers-reduced-motion: reduce)").matches) || !sourceBody?.getBoundingClientRect().width || !scene?.getBoundingClientRect().width;
    const cleanup = [], pending = new Set();
    let cancelled = false, monitor = 0, actor = null, actorPosition = null;
    // Readability is a playback concern; ready_at and the report remain unchanged.
    const duration = Math.max(event.skill ? 900 : 450, Number(event.duration_ms) || 0);
    const introShare = event.skill ? .5 : .4;
    const impactShare = event.skill ? .3 : .4;
    const targets = event.targets || [];
    const attackType = source?.atkType || source?.girl?.atkType || "melee";
    const hostile = targets.filter(target => target.damage > 0 || target.absorbed > 0);
    const moves = hostile.length > 0 && ["melee", "heavy"].includes(attackType);
    let layer = null;
    const owns = node => {
      cleanup.push(() => node.remove());
      return node;
    };
    const cssClass = (node, name) => {
      if (!node || node.classList.contains(name)) return;
      node.classList.add(name);
      cleanup.push(() => node.classList.remove(name));
    };
    const valid = () => !cancelled && current(battle) && scene?.isConnected && sourceNode?.isConnected;
    function cancel() {
      if (cancelled) return;
      cancelled = true;
      stopFrame(monitor);
      for (const finish of [...pending]) finish(false);
      for (const restore of cleanup.reverse()) restore();
      active.delete(api);
    }
    // A watcher also cleans up while the caller is between two impact calls.
    function watch() {
      if (!valid()) { cancel(); return; }
      monitor = frame(watch);
    }
    function tween(milliseconds, draw) {
      if (!valid()) { cancel(); return Promise.resolve(false); }
      if ((reduced || milliseconds <= 0) && !battle.menuPaused) { draw?.(1); return Promise.resolve(true); }
      return new Promise(resolve => {
        const sharedClock = typeof root.battlePlaybackTime === "function";
        const now = () => sharedClock ? root.battlePlaybackTime(battle) : root.performance.now();
        let elapsed = 0, previous = now(), raf = 0, done = false;
        const finish = result => {
          if (done) return;
          done = true;
          stopFrame(raf);
          pending.delete(finish);
          resolve(result);
        };
        pending.add(finish);
        if (!battle.menuPaused) draw?.(0);
        const tick = () => {
          if (!valid()) { cancel(); finish(false); return; }
          const currentTime = now();
          if (!battle.menuPaused) elapsed += Math.max(0, currentTime - previous) * (sharedClock ? 1 : clamp(Number(battle.speed) || 1, 1, 4));
          previous = currentTime;
          if (battle.menuPaused) { raf = frame(tick); return; }
          const progress = reduced || milliseconds <= 0 ? 1 : Math.min(1, elapsed / milliseconds);
          draw?.(progress);
          if (progress >= 1) finish(true);
          else raf = frame(tick);
        };
        raf = frame(tick);
      });
    }
    function bounds(body) {
      if (!body || !scene) return null;
      const box = body.getBoundingClientRect(), stage = scene.getBoundingClientRect();
      if (!box.width || !box.height || !stage.width || !stage.height) return null;
      // Use scene coordinates even if a containing page is scaled.
      const sx = (scene.clientWidth || stage.width) / stage.width;
      const sy = (scene.clientHeight || stage.height) / stage.height;
      return { x: (box.left - stage.left) * sx, y: (box.top - stage.top) * sy,
        width: box.width * sx, height: box.height * sy };
    }
    function point(uid) {
      const rect = bounds(findUnit(uid)?.querySelector(".u-body"));
      return rect && { x: rect.x + rect.width / 2, y: rect.y + rect.height * .55, rect };
    }
    const origin = bounds(sourceBody);
    function inside(position) {
      const stage = scene.getBoundingClientRect();
      const width = scene.clientWidth || stage.width, height = scene.clientHeight || stage.height;
      return { x: clamp(position.x, 0, Math.max(0, width - origin.width)),
        y: clamp(position.y, 0, Math.max(0, height - origin.height)) };
    }
    function contact(uid) {
      const dst = point(uid);
      if (!dst || !origin) return null;
      const start = { x: origin.x + origin.width / 2, y: origin.y + origin.height / 2 };
      const end = { x: dst.rect.x + dst.rect.width / 2, y: dst.rect.y + dst.rect.height / 2 };
      const dx = end.x - start.x, dy = end.y - start.y, distance = Math.hypot(dx, dy);
      if (distance < 1) return inside({ x: origin.x, y: origin.y });
      const nx = dx / distance, ny = dy / distance;
      // Project both figures onto the approach vector. This works for diagonal
      // landscape lanes and upper/lower camps on a portrait phone.
      const reach = (Math.abs(nx) * (origin.width + dst.rect.width) + Math.abs(ny) * (origin.height + dst.rect.height)) / 2;
      const overlap = Math.min(origin.width, origin.height, dst.rect.width, dst.rect.height) * .18;
      const separation = Math.min(distance, Math.max(12, reach - overlap));
      return inside({ x: end.x - nx * separation - origin.width / 2,
        y: end.y - ny * separation - origin.height / 2 });
    }
    function place(position) {
      actorPosition = inside(position);
      actor.style.left = actorPosition.x + "px";
      actor.style.top = actorPosition.y + "px";
    }
    function travel(position, milliseconds, jumping = false) {
      if (!actor || !position) return tween(milliseconds);
      const from = { ...actorPosition };
      return tween(milliseconds, t => {
        const progress = ease(t);
        place({ x: from.x + (position.x - from.x) * progress,
          y: from.y + (position.y - from.y) * progress - (jumping ? Math.sin(Math.PI * t) * origin.height * .16 : 0) });
      });
    }
    function cloneActor() {
      const image = sourceBody?.querySelector("img");
      if (!image || !origin || !layer || reduced) return;
      actor = owns(document.createElement("div"));
      actor.className = "bm-actor" + (source.side === "enemy" ? " bm-enemy" : "");
      actor.style.width = origin.width + "px";
      actor.style.height = origin.height + "px";
      const face = document.createElement("img");
      face.className = "bm-avatar"; face.src = image.currentSrc || image.src; face.alt = "";
      actor.appendChild(face); layer.appendChild(actor); place({ x: origin.x, y: origin.y });
      const before = sourceBody.style.getPropertyValue("visibility");
      const priority = sourceBody.style.getPropertyPriority("visibility");
      sourceBody.style.setProperty("visibility", "hidden");
      cleanup.push(() => before ? sourceBody.style.setProperty("visibility", before, priority) : sourceBody.style.removeProperty("visibility"));
    }
    async function portrait(milliseconds) {
      if (!layer || reduced) return;
      const image = sourceBody?.querySelector("img");
      if (!image) return;
      const cutin = owns(document.createElement("div"));
      cutin.className = "bm-cutin" + (source.side === "enemy" ? " bm-enemy" : "");
      const face = document.createElement("img");
      face.src = image.currentSrc || image.src; face.alt = "";
      const caption = document.createElement("div");
      const name = document.createElement("small"), skill = document.createElement("strong");
      name.textContent = source.name || ""; skill.textContent = event.skill_name || "";
      caption.append(name, skill); cutin.append(face, caption); layer.appendChild(cutin);
      layer.classList.add("bm-cutscene");
      try {
        await tween(milliseconds, t => {
          cutin.style.opacity = String(Math.min(1, t * 7, (1 - t) * 7));
          const slide = Math.min(28, scene.clientWidth * .06);
          cutin.style.transform = `translateX(${(1 - ease(t)) * slide * (source.side === "enemy" ? 1 : -1)}px)`;
        });
      } finally {
        cutin.remove();
        layer.classList.remove("bm-cutscene");
      }
    }
    function joineryArt(phase, position) {
      if (!layer || !position || source.girlId !== "xs039" || !["skill","xs039-skill-2"].includes(event.skill_key) || phase === "impact" && event.variant === "rescue") return null;
      const art = owns(document.createElement("img"));
      art.className = "bm-joinery-art";
      art.src = `content/cards/xs039/assets/fx/${event.skill_key}/${phase}.webp`; art.alt = "";
      art.style.left = position.x + "px"; art.style.top = position.y + "px";
      layer.appendChild(art);
      return art;
    }
    async function begin() {
      if (!valid() || event.skipped || ["round", "wave", "ailment"].includes(event.kind)) { cancel(); return api; }
      active.add(api);
      monitor = frame(watch);
      if (battle.menuPaused && !await tween(0)) return api;
      if (!reduced) {
        layer = owns(document.createElement("div"));
        layer.className = "battle-motion-layer";
        layer.setAttribute("aria-hidden", "true");
        layer.style.setProperty("--motion-color", source.girl?.fxColor || "#f6cd87");
        scene.appendChild(layer);
        cssClass(sourceNode, "bm-source");
        if (event.skill) {
          const shade = owns(document.createElement("div")); shade.className = "bm-shade";
          layer.appendChild(shade);
        }
        // The sprite layer crosses both camps; labels and HP meters stay at their slots.
        cloneActor();
      }
      if (event.skill) await portrait(duration * .35);
      const castArt = joineryArt("cast", origin);
      if (valid()) {
        const preparation = duration * (introShare - (event.skill ? .35 : 0));
        if (moves) await travel(contact(hostile[0].uid), preparation, attackType === "heavy");
        else await tween(preparation, t => {
          if (actor) actor.style.transform = `translateY(${-Math.sin(Math.PI * t) * 5}px)`;
        });
      }
      castArt?.remove();
      return api;
    }
    async function presentImpact(target, index = 0, onImpact) {
      if (!valid()) { cancel(); return false; }
      if (battle.menuPaused && !await tween(0)) return false;
      const node = findUnit(target.uid), dst = point(target.uid);
      if (!node) return true;
      const attack = target.damage > 0 || target.absorbed > 0;
      const beat = duration * impactShare / Math.max(1, targets.length);
      if (reduced) { onImpact?.(); return true; }
      cssClass(node, "bm-target");
      if (attack && moves && index > 0) await travel(contact(target.uid), beat * .35);
      else if (attack && attackType === "ranged" && layer && dst) {
        const sp = point(source.uid);
        if (sp) {
          const orb = owns(document.createElement("i")); orb.className = "bm-projectile";
          layer.appendChild(orb);
          await tween(beat * .35, t => {
            orb.style.left = sp.x + (dst.x - sp.x) * t + "px";
            orb.style.top = sp.y + (dst.y - sp.y) * t - Math.sin(Math.PI * t) * 18 + "px";
          });
          orb.remove();
        }
      } else await tween(beat * .35);
      if (!valid()) { cancel(); return false; }
      if (battle.menuPaused && !await tween(0)) return false;
      onImpact?.();
      if (!layer || !dst) return true;
      const pulse = owns(document.createElement("i"));
      const joineryImpact = joineryArt("impact",dst);
      pulse.className = "bm-impact" + (attack ? "" : " bm-support");
      pulse.style.left = dst.x + "px"; pulse.style.top = dst.y + "px";
      layer.appendChild(pulse);
      await tween(beat * .65, t => {
        pulse.style.opacity = String(1 - t);
        pulse.style.transform = `translate(-50%, -50%) scale(${.35 + t * 1.2}) rotate(${attack ? -24 : 0}deg)`;
      });
      pulse.remove();
      joineryImpact?.remove();
      return valid();
    }
    async function impact(target, index = 0, onImpact) {
      try { return await presentImpact(target, index, onImpact); }
      catch (error) { cancel(); throw error; }
    }
    async function finish() {
      try {
        if (valid()) {
          if (actor && origin) await travel({ x: origin.x, y: origin.y }, duration * .2);
          else await tween(duration * .2);
        }
      } finally { cancel(); }
    }
    const api = { battle, begin, impact, finish, cancel, get active() { return valid(); } };
    return api;
  }
  async function beginBattleAction(battle, source, event) {
    cancelBattleMotion(battle);
    const action = createAction(battle, source, event);
    try { return await action.begin(); }
    catch (error) { action.cancel(); throw error; }
  }
  root.BattleMotion = Object.freeze({ beginBattleAction, cancelBattleMotion });
})(window);
