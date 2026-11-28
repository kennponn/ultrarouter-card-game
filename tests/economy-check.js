const assert = require('node:assert/strict');
const test = require('node:test');
const { game, market, sampleWeek, simulate, simulateWorld } = require('../scripts/simulate-economy');

test('repeatable stamina purchases cannot fund themselves even with the largest doubled token roll', () => {
  const maxRate = Math.max(...game.STAGES.map(stage => 2 * stage.tokenRange[1] / stage.cost));
  const energy = offer => (offer.resource === 'stamina' ? 1 : market.packs[offer.resource] || 0) * offer.amount;
  const supplies = market.offers.filter(item => item.shop === 'token' && energy(item));
  assert(supplies.length);
  for (const item of supplies) assert(energy(item) * maxRate < item.price, item.id);
  const dungeonRate = Math.max(...market.dungeon_coins.map((coins, index) => coins / market.dungeon_stamina[index]));
  for (const item of market.offers.filter(item => item.shop === 'dungeon' && energy(item))) assert(energy(item) * dungeonRate < item.price, item.id);
  for (const item of market.offers) {
    assert(Number.isSafeInteger(item.limit) && item.limit > 0);
    assert(Number.isSafeInteger(item.price) && item.price > 0);
    assert(Number.isSafeInteger(item.amount) && item.amount > 0);
  }
});

test('the ranked midgame weekly plan has narrow headroom and no single omitted activity replaces its rewards', () => {
  const { full, noArena, noCaravan, noGuild, noExpedition, noFestival, holdSeat, staminaFirst, noSupplyPacks, treasureSquad, cooperativeGuild } = simulate(512);
  for (const coverage of [full.goldCoverage, full.tokenCoverage]) assert(coverage[1] >= 98 && coverage[1] <= 104, String(coverage));
  assert(full.budgetFeasiblePct >= 50);
  assert(noArena.tokenCoverage[1] < 75 && noCaravan.goldCoverage[1] < 80);
  assert(noGuild.tokenCoverage[1] < full.tokenCoverage[1] && noExpedition.expeditionCoinsPerDay === 0);
  assert(noFestival.ticketsPerWeek < full.ticketsPerWeek && holdSeat.arenaCoinsPerDay < full.arenaCoinsPerDay);
  assert(holdSeat.ticketsPerWeek > noArena.ticketsPerWeek, 'keeping a seat still earns its daily settlement');
  assert(full.packStaminaPerDay > 0 && noSupplyPacks.packStaminaPerDay === 0 && staminaFirst.packStaminaPerDay > full.packStaminaPerDay);
  assert(full.oreTicketsPerDay > 1 && full.guildTokensPerDay > market.guild.token_amount && noGuild.guildTokensPerDay === 0);
  assert(treasureSquad.goldPerDay > full.goldPerDay && cooperativeGuild.guildTokensPerDay > full.guildTokensPerDay);
  assert(full.onePolishDeferredPct >= 90 && full.budgetFeasiblePct < full.onePolishDeferredPct);
});

test('week-long shard cohorts reflect chest refunds and a sharp top-20 cutoff without gifting a whole SSR', () => {
  const unranked = simulateWorld(24, 21), twentieth = simulateWorld(24, 20), winner = simulateWorld(45, 1);
  assert.equal(twentieth.meanShards - unranked.meanShards, twentieth.rankShards);
  assert.equal(unranked.rankShards, 0);
  assert.equal(unranked.pulls, 26);
  assert.equal(unranked.summonPct, 0);
  assert(winner.summonPct > 75 && winner.summonPct < 100, 'first place is an advantage, not an automatic full card');
  assert(winner.rankShards > twentieth.rankShards * 5);
});

test('buying every token stamina restock raises farming income but spends the posted price', () => {
  const supply = market.offers.find(item => item.id === 'stamina_supply');
  const normal = sampleWeek(64, {farm:'n1'}), bought = sampleWeek(64, {farm:'n1', tokenStamina:true});
  assert(bought.gold > normal.gold);
  assert.equal(bought.tokenPlan - normal.tokenPlan, 7 * 3 * supply.price * supply.limit);
});
