// Presentation only: callers keep their existing claim handlers and eligibility checks.
function rewardChestArt({tier = 1, state = "locked"} = {}) {
  const level = Math.max(1, Math.min(3, Math.trunc(Number(tier)) || 1));
  const appearance = ["ready", "claimed"].includes(state) ? state : "locked";
  return `<span class="reward-chest-art" data-chest-state="${appearance}" aria-hidden="true"><img src="assets/world/box_${level}${appearance === "claimed" ? "_open" : ""}.webp" alt="" width="84" height="84" decoding="async" draggable="false"></span>`;
}

// rewards: [{name, count}]; attributes: data-* attributes for the claim button.
// disabled may override the visual state when a view is waiting for a response.
function rewardChestCard({title = "奖励宝箱", threshold = "", state = "locked", rewards = [], status, tier = 1, attributes = {}, disabled} = {}) {
  const appearance = ["ready", "claimed"].includes(state) ? state : "locked";
  const label = status ?? {ready:"领取宝箱", locked:"未达成", claimed:"已领取"}[appearance];
  const blocked = disabled ?? appearance !== "ready";
  const data = Object.entries(attributes).filter(([key]) => /^data-[a-z][a-z0-9-]*$/.test(key)).map(([key, value]) => ` ${key}="${esc(String(value))}"`).join("");
  const items = rewards.map(reward => `<li><span>${esc(String(reward.name))}</span><b>×${esc(Number.isSafeInteger(reward.count) ? reward.count.toLocaleString("zh-CN") : String(reward.count))}</b></li>`).join("");
  return `<article class="reward-chest-card" data-chest-state="${appearance}"><h4 class="reward-chest-title">${esc(String(title))}</h4>${threshold === "" ? "" : `<p class="reward-chest-threshold">${esc(String(threshold))}</p>`}<button type="button" class="reward-chest-button"${data}${blocked ? " disabled" : ""} aria-label="${esc(`${title}，${label}`)}">${rewardChestArt({tier, state:appearance})}<span class="reward-chest-status">${esc(String(label))}</span></button>${items ? `<ul class="reward-chest-rewards" aria-label="宝箱奖励">${items}</ul>` : ""}</article>`;
}
