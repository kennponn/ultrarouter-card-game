/* Code-drawn skill studies. No image decoding, remote assets, or combat simulation. */
(() => {
  'use strict';
  let frame = 0;
  let current = null;
  const TAU = Math.PI * 2;
  const clamp = (n, a = 0, b = 1) => Math.min(b, Math.max(a, n));
  const ease = n => 1 - Math.pow(1 - clamp(n), 3);
  const modes = {
    flame: { label: '焰潮', color: '#ff936d', secondary: '#ffe1a0' },
    frost: { label: '霜晶', color: '#92e8ff', secondary: '#e6faff' },
    space: { label: '折界', color: '#b3a2ff', secondary: '#e8d9ff' },
    shield: { label: '星垒', color: '#a4d7ff', secondary: '#efe5ae' },
    heal: { label: '生息', color: '#9be1b5', secondary: '#ffe5b0' },
    blade: { label: '刃光', color: '#f3a6ba', secondary: '#fff1d8' },
    lightning: { label: '鸣闪', color: '#c4b3ff', secondary: '#f9f0ad' },
    tide: { label: '潮汐', color: '#78ded8', secondary: '#d5f5ef' },
    binding: { label: '结缚', color: '#d8a5ec', secondary: '#f0d7ff' },
    impact: { label: '坠星', color: '#ebc28d', secondary: '#fff0ce' }
  };
  function classify(card, skill) {
    const name = `${skill.name} ${card.title} ${card.element}`;
    if (/对榫|拆接|换榫|空间|折界|回溯|怀表|时之|time|space/i.test(name)) return 'space';
    if (/治疗|治愈|祝福|甘露|药|医|丰收|奶|春雨|圣愈|healing|plant/i.test(name)) return 'heal';
    if (/壁垒|铁壁|屏障|守护|星盾|星墙|护光|盾|shield/i.test(name)) return 'shield';
    if (/冰|霜|雪|凛冬|冷凝|frost|ice/i.test(name)) return 'frost';
    if (/火|焰|炉|煤|燃|炽|熔|燧|fire/i.test(name)) return 'flame';
    if (/雷|电|鸣闪|lightning|thunder/i.test(name)) return 'lightning';
    if (/潮|海|浪|泉|雨|water|tide/i.test(name)) return 'tide';
    if (/线|缚|绳|丝|结|网|影|锁|shadow/i.test(name)) return 'binding';
    if (/刀|刃|剑|矢|弓|箭|枪|风|斩|wind/i.test(name)) return 'blade';
    return 'impact';
  }
  function random(seed) {
    let value = seed >>> 0;
    return () => { value = (Math.imul(value, 1664525) + 1013904223) >>> 0; return value / 4294967296; };
  }
  function line(ctx, x1, y1, x2, y2, width = 1) {
    ctx.lineWidth = width;
    ctx.beginPath(); ctx.moveTo(x1, y1); ctx.lineTo(x2, y2); ctx.stroke();
  }
  function ring(ctx, x, y, radius, width = 1) {
    ctx.lineWidth = width;
    ctx.beginPath(); ctx.arc(x, y, Math.max(.1, radius), 0, TAU); ctx.stroke();
  }
  function polygon(ctx, x, y, radius, sides, rotation = 0, fill = false) {
    ctx.beginPath();
    for (let i = 0; i <= sides; i++) {
      const a = i / sides * TAU + rotation;
      if (!i) ctx.moveTo(x + Math.cos(a) * radius, y + Math.sin(a) * radius);
      else ctx.lineTo(x + Math.cos(a) * radius, y + Math.sin(a) * radius);
    }
    fill ? ctx.fill() : ctx.stroke();
  }
  function glow(ctx, x, y, radius, color, alpha = 1) {
    const gradient = ctx.createRadialGradient(x, y, 0, x, y, Math.max(1, radius));
    gradient.addColorStop(0, color); gradient.addColorStop(1, 'transparent');
    ctx.save(); ctx.globalAlpha = alpha; ctx.fillStyle = gradient; ctx.fillRect(x - radius, y - radius, radius * 2, radius * 2); ctx.restore();
  }
  function drawScene(time) {
    const { canvas, ctx, mode, card, skill, particles, caption } = current;
    const box = canvas.getBoundingClientRect();
    const width = Math.max(280, box.width), height = Math.max(200, box.height);
    const pixelRatio = Math.min(window.devicePixelRatio || 1, 2);
    if (canvas.width !== Math.round(width * pixelRatio) || canvas.height !== Math.round(height * pixelRatio)) {
      canvas.width = Math.round(width * pixelRatio); canvas.height = Math.round(height * pixelRatio);
    }
    ctx.setTransform(pixelRatio, 0, 0, pixelRatio, 0, 0);
    ctx.clearRect(0, 0, width, height);
    const spec = modes[mode], scale = Math.min(width / 720, height / 380);
    const x = width * .5, y = height * .49;
    const t = time / 1000, charge = ease(t / 1.35), release = ease((t - 1.2) / 1.4), fade = 1 - clamp((t - 3.2) / 2) * .78;
    const gradient = ctx.createLinearGradient(0, 0, width, height);
    gradient.addColorStop(0, '#08161f'); gradient.addColorStop(1, '#10192e');
    ctx.fillStyle = gradient; ctx.fillRect(0, 0, width, height);
    ctx.save();
    for (const p of particles.slice(0, 65)) {
      ctx.globalAlpha = .16 + .3 * Math.sin(p.angle + t * .5) ** 2;
      ctx.fillStyle = '#c5d9e7';
      ctx.beginPath(); ctx.arc(p.x * width, p.y * height, p.size * .55, 0, TAU); ctx.fill();
    }
    ctx.restore();
    glow(ctx, x, y, (90 + release * 100) * scale, spec.color, .08 + release * .06);
    ctx.save(); ctx.translate(x, y); ctx.scale(1, .35); ctx.strokeStyle = spec.color; ctx.globalAlpha = .18;
    ring(ctx, 0, 120 * scale, 210 * scale); ring(ctx, 0, 120 * scale, 180 * scale);
    ctx.restore();
    ctx.strokeStyle = spec.color; ctx.fillStyle = spec.color;
    ctx.shadowColor = spec.color; ctx.shadowBlur = 15 * scale;
    ctx.save(); ctx.globalAlpha = fade;
    const r = (35 + charge * 65) * scale;
    if (mode === 'flame') {
      for (const p of particles) {
        const life = (t * .7 + p.phase) % 1;
        const reach = (30 + release * 155) * scale;
        const px = x + Math.sin(p.angle * 2 + life * 4) * reach * (1 - life) * p.x;
        const py = y + 70 * scale - life * (75 + release * 155) * scale;
        ctx.globalAlpha = fade * Math.sin(life * Math.PI) * charge;
        ctx.fillStyle = life > .5 ? spec.color : spec.secondary;
        polygon(ctx, px, py, (3 + p.size * 4) * (1 - life) * scale, 3, -Math.PI / 2, true);
      }
      ctx.globalAlpha = fade * release;
      ctx.strokeStyle = spec.secondary;
      ring(ctx, x, y, 25 * scale + release * 135 * scale, 2);
    } else if (mode === 'frost') {
      for (let i = 0; i < 6; i++) {
        const a = i / 6 * TAU + t * .065;
        ctx.save(); ctx.translate(x, y); ctx.rotate(a);
        line(ctx, 0, 0, r * (1 + release * .3), 0, 2);
        for (let j = 1; j <= 3; j++) {
          const branch = r * .18 * charge;
          const bx = r * j / 4;
          line(ctx, bx, 0, bx - branch, branch, 1.5);
          line(ctx, bx, 0, bx - branch, -branch, 1.5);
        }
        ctx.restore();
      }
      for (const p of particles.slice(0, 24)) {
        ctx.globalAlpha = fade * release * .7;
        const reach = (105 + release * p.speed * 100) * scale;
        polygon(ctx, x + Math.cos(p.angle) * reach, y + Math.sin(p.angle) * reach * .7, p.size * 5 * scale, 4, t + p.angle);
      }
    } else if (mode === 'space') {
      for (let i = 0; i < 3; i++) {
        ctx.save(); ctx.translate(x, y); ctx.rotate(t * .25 * (i % 2 ? -1 : 1) + i * .7); ctx.scale(1, .35 + i * .22);
        ring(ctx, 0, 0, r * (1 + i * .35), 1.5);
        for (let j = 0; j < 8; j++) polygon(ctx, Math.cos(j / 8 * TAU) * r, Math.sin(j / 8 * TAU) * r, 3 * scale, 4, 0, true);
        ctx.restore();
      }
      ctx.globalAlpha = release * fade;
      line(ctx, x - 175 * scale, y + Math.sin(t * 2) * 15 * scale, x + 175 * scale, y, 2);
      for (let side = -1; side <= 1; side += 2) {
        const a = release * Math.PI;
        polygon(ctx, x + Math.cos(a) * side * 100 * scale, y + Math.sin(a) * side * 45 * scale, 15 * scale, 4, Math.PI / 4 + t);
      }
    } else if (mode === 'shield') {
      ctx.save(); ctx.translate(x, y); ctx.scale(charge, charge);
      polygon(ctx, 0, 0, 90 * scale, 6, -Math.PI / 2);
      polygon(ctx, 0, 0, 77 * scale, 6, -Math.PI / 2);
      for (let i = 0; i < 6; i++) {
        const a = i / 6 * TAU - Math.PI / 2;
        line(ctx, 0, 0, Math.cos(a) * 77 * scale, Math.sin(a) * 77 * scale, .7);
        glow(ctx, Math.cos(a) * 90 * scale, Math.sin(a) * 90 * scale, 14 * scale, spec.secondary, .8);
      }
      ctx.restore();
      for (let i = 0; i < 3; i++) {
        ctx.globalAlpha = (1 - release) * fade * charge;
        const px = x + (200 - release * 115) * scale;
        line(ctx, px, y + (i - 1) * 35 * scale, px + 28 * scale, y + (i - 1) * 35 * scale, 2);
      }
      ctx.globalAlpha = release * fade * .7;
      ring(ctx, x, y, (95 + release * 55) * scale);
    } else if (mode === 'heal') {
      for (let i = 0; i < 7; i++) {
        const a = i / 7 * TAU + t * .15;
        ctx.save(); ctx.translate(x, y); ctx.rotate(a);
        ctx.beginPath(); ctx.ellipse(r * .48, 0, r * .6, r * .18, 0, 0, TAU); ctx.stroke();
        ctx.restore();
      }
      for (const p of particles.slice(0, 35)) {
        const life = (t * .23 + p.phase) % 1;
        const px = x + Math.cos(p.angle) * 170 * scale * release;
        const py = y + 100 * scale - life * 230 * scale;
        ctx.globalAlpha = fade * Math.sin(life * Math.PI) * release;
        const len = (3 + p.size) * scale;
        line(ctx, px - len, py, px + len, py); line(ctx, px, py - len, px, py + len);
      }
    } else if (mode === 'blade') {
      for (let i = 0; i < 4; i++) {
        const progress = ease((t - .7 - i * .2) / .55);
        ctx.globalAlpha = Math.sin(progress * Math.PI) * fade;
        ctx.save(); ctx.translate(x, y + (i - 1.5) * 16 * scale); ctx.rotate(-.65 + i * .12);
        const span = 210 * scale;
        const end = -span + progress * span * 2;
        line(ctx, Math.max(-span, end - 150 * scale), 0, end, 0, (5 - i) * scale);
        ctx.strokeStyle = spec.secondary; line(ctx, Math.max(-span, end - 130 * scale), -2 * scale, end, -2 * scale, 1);
        ctx.restore();
      }
      ctx.globalAlpha = fade * release * .5;
      ctx.save(); ctx.translate(x, y); ctx.rotate(-.6);
      ctx.beginPath(); ctx.ellipse(0, 0, 165 * scale, 35 * scale, 0, 0, TAU); ctx.stroke(); ctx.restore();
    } else if (mode === 'lightning') {
      for (let i = 0; i < 6; i++) {
        const angle = i / 6 * TAU + .3;
        const reach = (50 + release * 115) * scale;
        ctx.beginPath(); ctx.moveTo(x, y);
        for (let j = 1; j <= 7; j++) {
          const d = j / 7 * reach;
          const jitter = Math.sin(j * 13 + i * 7 + Math.floor(t * 7)) * 15 * scale * release;
          ctx.lineTo(x + Math.cos(angle) * d + Math.sin(angle) * jitter, y + Math.sin(angle) * d - Math.cos(angle) * jitter);
        }
        ctx.lineWidth = 2 * scale; ctx.stroke();
      }
      glow(ctx, x, y, 40 * scale, spec.secondary, release * .65);
    } else if (mode === 'tide') {
      for (let i = 0; i < 6; i++) {
        ctx.globalAlpha = fade * (.3 + i * .1) * charge;
        ctx.beginPath();
        for (let j = 0; j <= 100; j++) {
          const px = x - 200 * scale + j * 4 * scale;
          const wave = Math.sin(j * .065 - t * 2.2 + i * .45) * (15 + release * 45) * scale;
          const py = y + (i - 2.5) * 12 * scale + wave;
          j ? ctx.lineTo(px, py) : ctx.moveTo(px, py);
        }
        ctx.lineWidth = i === 3 ? 3 : 1; ctx.stroke();
      }
    } else if (mode === 'binding') {
      for (let i = 0; i < 5; i++) {
        ctx.save(); ctx.translate(x, y); ctx.rotate(i * Math.PI / 5 + t * .1);
        const size = (160 - release * 90) * scale;
        ctx.beginPath(); ctx.ellipse(0, 0, size, 30 * scale * charge, 0, 0, TAU); ctx.stroke();
        for (let j = 0; j < 7; j++) polygon(ctx, Math.cos(j / 7 * TAU) * size, Math.sin(j / 7 * TAU) * 30 * scale, 4 * scale, 4, Math.PI / 4);
        ctx.restore();
      }
    } else {
      ctx.save(); ctx.translate(x, y); ctx.rotate(-Math.PI / 4 + t * .05);
      polygon(ctx, 0, 0, r * .6, 4, Math.PI / 4);
      line(ctx, -r, 0, r, 0, 2); line(ctx, 0, -r, 0, r, 2);
      ctx.restore();
      for (let i = 0; i < 3; i++) {
        ctx.globalAlpha = fade * release * (1 - i * .2);
        ring(ctx, x, y, (65 + release * 95 + i * 18) * scale);
      }
    }
    ctx.restore();
    ctx.shadowBlur = 0;
    ctx.save();
    ctx.globalAlpha = release * fade;
    for (const p of particles.slice(0, 28)) {
      const radius = (40 + release * 190 * p.speed) * scale;
      const px = x + Math.cos(p.angle) * radius, py = y + Math.sin(p.angle) * radius * .78;
      ctx.fillStyle = spec.secondary; ctx.beginPath(); ctx.arc(px, py, p.size * scale, 0, TAU); ctx.fill();
    }
    ctx.restore();
    ctx.fillStyle = '#aebfce'; ctx.textAlign = 'left'; ctx.font = `${Math.max(10, 11 * scale)}px sans-serif`;
    ctx.fillText(`${card.name}  /  ${spec.label}`, 20, 27);
    ctx.textAlign = 'right'; ctx.fillStyle = '#718998'; ctx.fillText('SKILL STUDY', width - 20, 27);
    const progress = clamp(time / 4800);
    ctx.fillStyle = '#263542'; ctx.fillRect(24, height - 15, width - 48, 2);
    ctx.fillStyle = spec.color; ctx.fillRect(24, height - 15, (width - 48) * progress, 2);
    const stage = t < 1.2 ? '01 · 蓄势' : t < 2.6 ? '02 · 释放' : t < 4 ? '03 · 共鸣' : '04 · 余响';
    if (caption && caption.textContent !== stage) caption.textContent = stage;
  }
  function tick(timestamp) {
    if (!current || current.paused) return;
    const delta = Math.min(80, timestamp - current.lastTime);
    current.lastTime = timestamp;
    current.elapsed += delta * current.speed;
    drawScene(Math.min(4800, current.elapsed));
    if (current.elapsed < 4800) frame = requestAnimationFrame(tick);
    else { current.canvas.dataset.effectState = 'complete'; current.onEnd?.(); }
  }
  function stop() {
    cancelAnimationFrame(frame);
    if (current) current.canvas.dataset.effectState = 'stopped';
    current = null;
  }
  function play(canvas, options) {
    stop();
    const ctx = canvas.getContext('2d');
    if (!ctx) { options.caption.textContent = '此浏览器未启用画布，请阅读下方施放意象。'; options.onEnd?.(); return; }
    const seed = [...options.card.id + options.skill.name].reduce((sum, char) => sum + char.codePointAt(0), 0);
    const rng = random(seed);
    current = { ...options, canvas, ctx, mode: classify(options.card, options.skill), elapsed: 0, speed: options.speed || 1, paused: false, lastTime: performance.now(), particles: Array.from({ length: 95 }, () => ({ x: rng(), y: rng(), size: .5 + rng() * 1.8, phase: rng(), angle: rng() * TAU, speed: .4 + rng() * .8 })) };
    canvas.dataset.effectKind = current.mode;
    canvas.dataset.effectState = 'playing';
    if (matchMedia('(prefers-reduced-motion: reduce)').matches) {
      drawScene(2800);
      current.elapsed = 4800;
      canvas.dataset.effectState = 'complete';
      options.caption.textContent = '共鸣定格 · 已遵循减少动态效果偏好';
      options.onEnd?.();
    } else frame = requestAnimationFrame(tick);
  }
  function togglePause() {
    if (!current || current.elapsed >= 4800) return false;
    current.paused = !current.paused;
    current.canvas.dataset.effectState = current.paused ? 'paused' : 'playing';
    if (current.paused) cancelAnimationFrame(frame);
    else { current.lastTime = performance.now(); frame = requestAnimationFrame(tick); }
    return current.paused;
  }
  document.addEventListener('visibilitychange', () => {
    if (document.hidden && current && !current.paused) {
      cancelAnimationFrame(frame);
    } else if (!document.hidden && current && !current.paused && current.elapsed < 4800) {
      current.lastTime = performance.now();
      frame = requestAnimationFrame(tick);
    }
  });
  window.CodexEffects = { play, stop, togglePause, setSpeed(speed) { if (current) current.speed = clamp(Number(speed) || 1, .25, 2); } };
})();
