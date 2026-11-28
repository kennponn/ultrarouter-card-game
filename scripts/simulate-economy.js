const fs = require('node:fs');
const path = require('node:path');

const rules = name => JSON.parse(fs.readFileSync(path.join(__dirname, '..', 'backend', 'model', `gacha_${name}.json`), 'utf8'));
const game = rules('catalog'), training = rules('training'), captain = rules('captain'), market = rules('market');
const cultivation = rules('cultivation'), festival = rules('festival'), payouts = game.ACTIVITY_PAYOUTS;
const worldBook = rules('world'), world = worldBook.editions[worldBook.edition];
const stages = Object.fromEntries(game.STAGES.map(stage => [stage.id, stage]));
const offers = Object.fromEntries(market.offers.map(item => [item.id, item]));
const offer = id => offers[id];
const profiles = {
  full: {}, holdSeat: { challenges: false }, noArena: { challenges: false, rank: 0 },
  noCaravan: { caravan: false }, noFestival: { festival: false }, noGuild: { guild: false },
  noExpedition: { expedition: false }, topThree: { rank: 3, skill: 47, locks: [1, 2, 3, 4] },
  noSupplyPacks: { packs: false }, staminaFirst: { gearShopping: false },
  treasureSquad: { goldBonus: 0.30 }, cooperativeGuild: { collective: true },
  maxTokenStamina: { tokenStamina: true, farm: 'n1' },
};

function generator(seed) {
  let value = seed >>> 0;
  return () => {
    value = (Math.imul(value, 1664525) + 1013904223) >>> 0;
    return value / 4294967296;
  };
}

function sampleWeek(seed, options = {}, prices = {}) {
  const policy = { challenges: true, rank: 50, festival: true, caravan: true, expedition: true, guild: true,
    skill: 40, locks: [1, 2, 3, 3], packs: true, gearShopping: true, goldBonus: 0, collective: false, ...options };
  const random = generator(seed), roll = ([low, high]) => low + Math.floor(random() * (high - low + 1));
  const rank = policy.rank && market.ladder_rewards.find(tier => !tier.max_rank || policy.rank <= tier.max_rank);
  const taskReward = captain.tasks.filter(task => !(task.id === 'ladder' && !policy.challenges || task.id === 'festival' && !policy.festival));
  const points = taskReward.reduce((total, task) => total + task.points, 0);
  const chests = captain.chests.filter(chest => chest.points <= points).map(chest => chest.resources);
  const chest = key => chests.reduce((sum, resources) => sum + (resources[key] || 0), 0);
  const loginMinutes = [12*60, 5*60, 7*60], staminaCap = 120;
  const dailyStamina = loginMinutes.reduce((sum, minutes) => sum + Math.min(staminaCap, Math.floor(minutes / game.STAMINA_REGEN_MIN)), 0)
    + captain.supplies.reduce((sum, meal) => sum + meal.stamina, 0) + chest('stamina_pack_30') * market.packs.stamina_pack_30;
  const output = { gold: 0, tokens: 0, goldPlan: 0, tokenPlan: 0, limitedTickets: 0, arenaCoins: 0, expeditionCoins: 0, dungeonCoins: 0,
    stamps: 0, guildTokens: 0, packStamina: 0, oreTickets: 0, paidDraws: 0 };
  const activeSkill = policy.skill + roll([-2, 2]);
  let nextFloor = 1, doubles = 1, ore = 0, guildCoins = 0, solvent = 0, boughtLens = false, boughtLog = false;
  const wallets = { arena: 0, dungeon: 0, expedition: 0 };
  const tickets = { gem_ticket: 0, chip_ticket: 0 };
  for (let day = 0; day < 7; day++) {
    const earned = { gold: taskReward.reduce((sum, task) => sum + task.gold, 0), tokens: chest('token') + (rank?.token || 0) };
    const usedStock = {};
    const buy = (id, quantity) => {
      const item = offer(id), used = usedStock[id] || 0;
      const count = Math.min(quantity, 3 * item.limit - used, Math.floor(wallets[item.shop] / item.price));
      wallets[item.shop] -= count * item.price;
      usedStock[id] = used + count;
      return count * item.amount;
    };
    output.limitedTickets += chest('limited_ticket') + (rank?.limited_ticket || 0);
    let arenaToday = rank?.arena_coin || 0;
    if (policy.challenges) for (let fight = 0; fight < 5; fight++) arenaToday += random() < 0.55 ? market.challenge_win : market.challenge_loss;
    output.arenaCoins += arenaToday;
    wallets.arena += arenaToday;
    let energy = dailyStamina;
    if (policy.tokenStamina) {
      const supply = offer('stamina_supply'), purchases = 3 * supply.limit;
      energy += purchases * supply.amount;
      output.tokenPlan += purchases * supply.price;
    }
    if (policy.guild) {
      energy -= 20;
      const guild = market.guild;
      guildCoins += guild.checkin + guild.sortie + guild.training + guild.contribution + 2 * guild.raid + (policy.challenges ? guild.arena : 0);
      if (policy.collective) {
        if (day === 2) guildCoins += 150;
        if (day === 4) { guildCoins += 250; ore += 20; }
        if (day === 6) { guildCoins += 400; earned.gold += 100 + 180 + 300; ore += 2 + 4 + 8; }
      }
      const units = Math.min(3 * guild.token_limit, Math.floor(guildCoins / guild.token_price));
      guildCoins -= units * guild.token_price;
      earned.tokens += units * guild.token_amount;
      output.guildTokens += units * guild.token_amount;
    }
    for (const [index, price] of market.dungeon_stamina.entries()) {
      if (index === 3 && !policy.caravan) continue;
      energy -= 3 * price;
      output.dungeonCoins += market.dungeon_coins[index] * 3;
      wallets.dungeon += market.dungeon_coins[index] * 3;
      if (index === 3) earned.gold += 3 * payouts.caravan_gold;
    }
    if (policy.expedition) {
      const floors = day < 3 ? 2 : 1;
      for (let battle = 0; battle < floors; battle++, nextFloor++) {
        energy -= 12 + nextFloor;
        earned.gold += Math.ceil((roll(payouts.expedition_gold) + nextFloor * payouts.expedition_gold_step) * 1.25);
        const multiplier = doubles > 0 ? 2 : 1;
        if (multiplier === 2) doubles--;
        earned.tokens += multiplier * Math.ceil((roll(payouts.expedition_tokens) + nextFloor * payouts.expedition_token_step) * 1.25);
        const expeditionCoins = market.expedition_base + nextFloor * market.expedition_per_floor;
        output.expeditionCoins += expeditionCoins;
        wallets.expedition += expeditionCoins;
      }
    }
    if (day === 4) { energy -= 20; earned.gold += payouts.boss_gold; }
    if (day >= 5) { energy -= 14; if (day === 6) earned.gold += payouts.dual_gold; }
    const events = policy.festival ? festival.slice(2*day, 2*day+2) : [];
    const previousStamps = output.stamps;
    output.stamps += events.length;
    const prizes = events.map(event => event.reward);
    for (const [mark, award] of Object.entries(payouts.festival_album)) if (previousStamps < +mark && +mark <= output.stamps) prizes.push(award);
    for (const award of prizes) {
      output.limitedTickets += award.limited_ticket || 0;
      ore += award.forge_ore || 0;
      solvent += award.iv_solvent || 0;
      for (const key of Object.keys(tickets)) tickets[key] += award[key] || 0;
    }
    ore += 3 * 7;
    solvent += 3;
    if (solvent < policy.locks.length) solvent += buy('dungeon_wash', Math.ceil((policy.locks.length - solvent) / offer('dungeon_wash').amount));
    if (solvent < policy.locks.length) throw new Error('Insufficient weekly refining supplies');
    solvent -= policy.locks.length;
    if (policy.gearShopping) {
      buy('arena_effort', 1);
      buy('arena_stone_green', 1);
      if (!boughtLens) boughtLens = buy('arena_lens', 1) > 0;
      buy('dungeon_book', 1);
      if (!boughtLog) boughtLog = buy('expedition_log', 1) > 0;
      if (day === 0 || day === 4) buy('expedition_stone', 1);
    }
    tickets.gem_ticket += buy('dungeon_ticket', 1);
    if (policy.packs) for (const id of ['arena_pack_120', 'arena_pack_60', 'arena_pack_30', 'dungeon_pack', 'expedition_pack']) {
      if (policy.gearShopping && (offer(id).shop === 'arena' && !boughtLens || offer(id).shop === 'expedition' && !boughtLog)) continue;
      const extra = buy(id, Infinity) * market.packs[offer(id).resource];
      energy += extra;
      output.packStamina += extra;
    }
    for (const farm of [stages[policy.farm || 'r1'], stages.s1]) {
      const attempts = Math.floor(energy / farm.cost);
      energy %= farm.cost;
      for (let attempt = 0; attempt < attempts; attempt++) {
        const boost = doubles > 0 && (!policy.expedition || nextFloor > 10);
        if (boost) doubles--;
        if (random() >= 0.97) continue;
        earned.gold += Math.round(roll(farm.gold) * (1 + policy.goldBonus));
        earned.tokens += roll(farm.tokenRange) * (boost ? 2 : 1);
        if (random() < 0.08) doubles++;
        ore += 1 + Math.floor(farm.cost / 6);
        if (farm.relicTier > 0) tickets.chip_ticket++;
        else if (farm.hasBoss) tickets.gem_ticket++;
      }
    }
    let paidDraws = 0;
    for (const key of Object.keys(tickets)) {
      tickets[key] += 3 + chest(key);
      const fromOre = Math.min(Math.max(0, 9 - tickets[key]), Math.floor(ore / cultivation.artifacts.ticket_ore));
      ore -= fromOre * cultivation.artifacts.ticket_ore;
      tickets[key] += fromOre;
      output.oreTickets += fromOre;
      const used = Math.min(9, tickets[key]); tickets[key] -= used;
      paidDraws += 9 - used;
    }
    output.paidDraws += paidDraws;
    const lockPrices = prices.lockTokens || cultivation.individual.lock_tokens;
    output.tokenPlan += paidDraws * cultivation.artifacts.draw_token + policy.locks.reduce((sum, locks) => sum + lockPrices[locks], 0);
    const breakSteps = day % 3 === 0 ? (prices.breakGold ?? training.break_cost.gold_base) + training.break_cost.gold_step * (5 + day / 3) : 0;
    const skillGold = 5 * (training.skill_pool.upgrade_gold_base + activeSkill * (prices.skillStep ?? training.skill_pool.upgrade_gold_step));
    output.goldPlan += skillGold + 10 * ((prices.effortGold ?? training.effort_gold_base) + 2 * training.effort_gold_step) + breakSteps + policy.locks.length * cultivation.individual.gold + 5 * game.GACHA.cost1;
    output.gold += earned.gold;
    output.tokens += earned.tokens;
  }
  return output;
}

function simulateWorld(tickets, rank, players = 8192) {
  const random = generator(41082), prize = rank > 0 && world.tiers.find(tier => rank <= tier.through)?.shards || 0;
  let pulls = 0, chestShards = 0;
  for (let remaining = tickets; remaining > 0; remaining--) {
    pulls++;
    const chest = world.chests.find(chest => chest.points === pulls);
    if (chest) { chestShards += chest.shards; remaining += chest.tickets; }
  }
  let total = 0, composed = 0;
  for (let player = 0; player < players; player++) {
    let shards = chestShards + prize;
    for (let roll = 0; roll < pulls; roll++) if (random() * 100 < world.shard_percent) shards++;
    total += shards;
    if (shards >= game.RARITY.SSR.shardToUnlock) composed++;
  }
  return { tickets, rank, pulls, totalDraws: pulls * players, rankShards: prize, meanShards: +(total / players).toFixed(2), summonPct: +(100 * composed / players).toFixed(1) };
}

function simulate(players = 2048, prices = {}, scenarios = profiles) {
  const results = {};
  for (const [name, policy] of Object.entries(scenarios)) {
    const weeks = Array.from({ length: players }, (_, index) => sampleWeek(Math.imul(2654435761, index + 1), policy, prices));
    const mean = key => Math.round(weeks.reduce((sum, week) => sum + week[key], 0) / players / 7 * 10) / 10;
    const quantiles = (income, demand) => {
      const values = weeks.map(week => 100 * week[income] / week[demand]).sort((left, right) => left - right);
      return [0.1, 0.5, 0.9].map(quantile => Number(values[Math.floor((players-1) * quantile)].toFixed(1)));
    };
    const goldBox = offer('gold');
    const feasiblePct = (goldSaved, tokensSaved) => +(100 * weeks.filter(week => {
      const boxes = Math.max(0, Math.ceil((week.goldPlan - goldSaved - week.gold) / goldBox.amount));
      return boxes <= 21 * goldBox.limit && week.tokens >= week.tokenPlan - tokensSaved + boxes * goldBox.price;
    }).length / players).toFixed(1);
    results[name] = { goldPerDay: mean('gold'), goldPlan: mean('goldPlan'), goldCoverage: quantiles('gold', 'goldPlan'),
      tokensPerDay: mean('tokens'), tokenPlan: mean('tokenPlan'), tokenCoverage: quantiles('tokens', 'tokenPlan'),
      budgetFeasiblePct: feasiblePct(0, 0), onePolishDeferredPct: feasiblePct(cultivation.individual.gold, (prices.lockTokens || cultivation.individual.lock_tokens)[Math.max(...(policy.locks || [1, 2, 3, 3]))]), ticketsPerWeek: weeks[0].limitedTickets,
      arenaCoinsPerDay: mean('arenaCoins'), dungeonCoinsPerDay: mean('dungeonCoins'), expeditionCoinsPerDay: mean('expeditionCoins'),
      guildTokensPerDay: mean('guildTokens'), packStaminaPerDay: mean('packStamina'), oreTicketsPerDay: mean('oreTickets'), paidItemDrawsPerDay: mean('paidDraws') };
  }
  return results;
}

if (require.main === module) {
  const results = simulate();
  const worldResults = [[results.full.ticketsPerWeek, 21], [results.full.ticketsPerWeek, 20], [results.topThree.ticketsPerWeek, 2], [sampleWeek(0, { rank: 1 }).limitedTickets, 1]].map(([tickets, rank]) => simulateWorld(tickets, rank));
  const firstPass = process.argv.includes('--compare') ? simulate(2048, { skillStep: 22, effortGold: 85, breakGold: 360, lockTokens: [0, 12, 36, 84, 168, 300] }, { full: {} }).full : undefined;
  console.log(JSON.stringify({ playersPerPolicy: 2048, days: 7, firstPass, results, worldResults }, null, 2));
}
module.exports = { game, market, offer, sampleWeek, simulate, simulateWorld, stages };
