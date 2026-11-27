# Integrating The Game Into Your Own Host

The exported Go files are game-specific packages from the existing new-api integration. The complete host is **not included**. The public upstream attribution/reference is recorded in `SOURCE.lock.json`; the reference is not a promise that every custom host helper exists in an arbitrary upstream release.

## Files And Initialization

Copy the allowlisted game `model/gacha*.go`, `controller/gacha*.go` and their companion embedded `model/gacha_*.json` catalogs into your compatible host packages, preserving the new-api import identifiers. Equipment, item design, training, carry, artifact grades, investigations, vigil rites and dispatch desks each have their own catalog. Use the examples in `integration/` to add only game routes and tables. The examples are not an automatic installer and never contact a maintainer server.

The game uses ten tables: `GachaRun`, `GachaCard`, `GachaDaily`, `GachaPlayer`, `GachaAction`, `GachaArenaGate`, `GachaArenaSeat`, `GachaArenaDuel`, `GachaArenaEntry`, and `GachaArenaRoom`. Initialize power and arena state after migrations succeed. The host must also own the user, option/configuration and log stores consumed by the game.

Auto-migrate `GachaPlayer.State` as `GachaDocument` before enabling the mailbox. MySQL must widen `gacha_players.state` from `TEXT` to `LONGTEXT` because a valid mailbox can exceed 65,535 bytes; SQLite and PostgreSQL keep `TEXT`. The column migration preserves the existing document and revision. No separate mail table or conversion of saved JSON is required. In a disposable host, run the fresh/upgrade/repeated-migration matrix and verify a mailbox larger than 64 KiB round-trips unchanged.

Serve the game files at `/gacha/` and the host APIs at the same origin. A navigation link may use `target="_blank"` and `rel="noopener noreferrer"`; no deployment domain or IP is supplied here.

Ship the styles and scripts referenced by `index.html`, including `harbor-mail.css` and `js/mail.js`. The authenticated profile supplies `captain_rules`, `voyage`, `festival` and the current player's `save.mailbox`. Embedded game catalogs such as `gacha_festival.json` are not account stores. Static-only delivery cannot enable the new game actions.

The journal accepts `captain_supply` with the issued Beijing `date:id`; the festival accepts `festival_play` with `date:event`, the expected attempt and the player's choice, or `festival_album` with the issued ISO week and threshold. These use the existing transaction/replay store; reject stale dates and do not derive claimability from the browser clock. Voyage chests take a catalog `item_id` only. The optional sweep takes a stage, issued day and count; it is an inventory operation, not a battle report, wallet operation or first clear.

Expose only the game HTML, CSS, `js/` and artwork as static assets. Do not expose the host's environment/data directories, backend source, development scripts or test files through the game mount.

## External Host Contracts

These dependencies are intentionally referenced rather than bundled as a complete account platform:

| Host Responsibility | Existing Integration Points |
| --- | --- |
| Validated browser identity | `middleware.GetSessionAuthIdentity(c)` must return a server-authenticated session identity, including `UserID`; game controllers reject non-session API credentials |
| Browser login/refresh | `/api/user/auth/refresh`, `/sign-in`; refresh returns the authenticated user and a short-lived access token kept only in browser memory |
| Database and account rows | `model.DB`, host `User`/configuration/log models, `common.UserStatusEnabled`, `common.RoleAdminUser` |
| JSON and API responses | `common.RawMessage`, `common.Marshal`, `common.Unmarshal`, `common.UnmarshalJsonStr`, `common.ApiSuccess` |
| Wallet unit and bounds | 1 E equals 500,000 integer quota; provide compatible `common.MaxWalletQuota`, `common.QuotaRound` and `common.WalletQuotaFromDecimalStrict` behavior |
| Wallet/cache consistency | host-local `cacheIncrUserQuota`, `syncCreditUserQuotaCache`; update caches only after a committed wallet mutation |
| Dialect-safe transactions | host-local `lockForUpdate(tx)`; use row locks on MySQL/PostgreSQL and compatible SQLite serialization |
| Accounting logs | host `RecordLog` and `LogTypeSystem`; do not log credentials |
| Rate limits | authenticated-user-scoped `UserBurstRateLimit`, a game action budget of 60 per 60 seconds, run-start budget of 30 per 60 seconds, and no cached private responses |
| Multiplier-card effects | `model.ActiveCardDiscount(userID, now)` returns the active server-owned discount; integrate it into your own host's multiplier policy if that feature is used |

The model also consumes non-secret host runtime flags such as batch-update/Redis availability. Adapt these to your own services rather than shipping their production environment. Atomic E exchange must not silently run against a delayed wallet mode that cannot enforce the transaction contract.

This list describes the integration boundary; it is not an implementation of the host's authentication, billing, cache or account management. If you change hosts, implement and test those adapters first. Do not bypass them with test logins, fixed IDs, static tokens or browser wallets.

## Game Requests

The route example lists the game GET/POST endpoints. The browser submits only action intent and server-issued run/card identifiers. Controllers reject unexpected fields, uploaded battle results and local saves. PvP is entered voluntarily through the queue or custom-room lobby; escrow and settlement are server-side.

The refresh interface is a host authentication endpoint, not one of the exported game handlers. Match its response contract in `js/store.js` when adapting your own host. Keep HttpOnly session cookies, trusted-origin/CSRF controls, memory-only access tokens and logout invalidation.

### Six-Slot Formations And Battle Playback

`GachaSquadLimit` is six for PvE, PvP and temporary awakening formations. The `squad` action accepts zero to six unique owned card IDs; starting combat requires at least one. Array order is the saved formation: positions 0–2 are the front row and 3–5 the back row. Dead units keep their slots. A normal attack targets a taunting enemy first, then the matching front column, another living front slot, the matching rear column, or another rear slot. Existing rage skills keep their individual target rules. Four or more distinct elements retain the existing five-percent defense formation bonus.

New battle reports have `version: 2`. Initial `allies` and `waves` entries include `slot` (0–5). Every event has a consecutive `seq` starting at 1. A `round` event records the authoritative `order` of living unit IDs after speed and PvP tie resolution, with `duration_ms: 0` and the usual `units` snapshots. Later action events remain authoritative when a unit dies or receives an extra turn. Existing `src`, `skill`, `skill_name`, `targets`, `units` and event-duration fields keep their contracts. Playback pause, stepping, speed and skip controls must not alter victory, drops or `ready_at`; settlements still wait for the server-issued deadline.

`teams_save` accepts 8, 10 or 12 distinct owned IDs and splits them into two equal squads. Dual trials accept four to six eligible cards in each squad and freeze both arrays at the first encounter. Existing four-card squads and already-issued dual trials remain valid; no automatic recruitment or account-data migration is needed. Adding the awakening subject to a full squad replaces slot 5. Existing reports and settled receipts are replayed as stored, including reports without version, slot, sequence or round-order metadata.

The formation UI supports mouse and touch dragging as well as selecting two cards. Dropping on an occupied slot swaps two cards; dropping on an empty slot moves the card to the end of the compact squad array. It sends the existing `squad` intent with the complete ordered list and waits for the authoritative receipt. A cancelled drag, changed session or stale source lineup submits no change. No new drag endpoint or browser-owned formation state is introduced.

The ranked arena uses the same six-slot simulator for challenges and defense snapshots. New NPC guards occupy six slots. Re-running `InitializeGachaLadder` keeps existing NPC defenses, player defenses, ranks, battle history and pending rewards; four-card defenses remain valid opponents. Updating a player's defense freezes the current lineup, including all six positions when filled.

### Mailbox And Gift Codes

Use the existing session-only `POST /api/user/gacha/action`. Every request includes `action` and a UUID `request_id`; reuse that UUID when retrying the same intent. The client selects a message using its exact `save.mailbox.messages[].id`, not its list position. It never supplies an account, attachment contents, XP or resource amounts.

| Action | Additional JSON fields | Result |
| --- | --- | --- |
| `mail_read` | `item_id`: message ID, 1–100 bytes | Marks this player's message read without claiming the attachment. |
| `mail_claim` | `item_id`: message ID, 1–100 bytes | Credits its unclaimed resources and/or captain XP once and marks it read and claimed. |
| `mail_all` | None | Claims all pending attachments in one transaction. An inventory capacity error leaves the entire batch unclaimed. |
| `mail_delete_read` | None | Deletes read messages only when they are already claimed or have no attachment. |
| `redeem_code` | `item_id`: public gift code, 1–40 bytes before trimming | Trims and uppercases the input, immediately credits the configured resources, and records the gift ID as redeemed in the same transaction. It creates no new mail. |

Configure public promotional gifts in `model/gacha_mail.json`, embedded into the game backend at build time. Each entry supplies a stable unique `id`, uppercase `code`, display `name`, inclusive Beijing dates `starts` and `ends` (`YYYY-MM-DD`), and positive server-defined `resources` quantities. Keep a gift's ID stable when editing its copy or code: the once-per-account marker is keyed by that ID and survives deletion of the letter. Treat codes as publishable game content, not login credentials. Rebuild the game backend to change the embedded catalog.

All player-initiated reward collection (including historical rank milestones, gift codes, chests, sweeps, captain tasks/supplies, progression, festival and guild claims/shop) and battle settlements credit game inventory immediately in the authoritative transaction. The daily 21:00 ranked settlement remains automatic mail, whose attachments are credited when claimed. Resource capacity errors roll back both rewards and claim markers. Shards unlock an eligible partner at settlement. Old unclaimed mail and existing claim markers remain valid; the upgrade never repeats an already mailed daily task, supply, gift or historical milestone.

`captain_task` uses `item_id=date:task` and claims only that completed task's gold and captain XP. Completed tasks remain available until claimed within that game day. `captain_supply` directly adds stamina during the 09:00–12:00, 12:00–21:00 and 21:00–24:00 Beijing windows (end exclusive); stamina above the natural cap is retained. For banked progression, `contract_claim` and `daily_contract_claim` share the same entitlement; send its issued day. Dispatch uses the separate `dispatch_contract_claim` action.

### Captain XP And Replay

In current rules the captain earns one XP per point of stamina the server actually spends, up to the captain level cap. This includes starting paid runs, sweeps and paid guild actions. The only other source is claiming daily task XP (or a previously issued task-mail attachment). Recovering or claiming stamina is not spending it; free runs, replaying a receipt and reading task mail add no XP. Card leveling is capped at the current captain level; card experience is separate from captain XP.

New runs store `rules_version=9` and record the stamina-XP notice in the snapshot when the stamina charge commits. Finishing or replaying this run displays the original notice without granting that XP again; a defeat or retreat does not undo the original spend. An already-issued but unsettled version-8 run uses a transition rule: its first settlement credits one captain XP per point of the stage's stamina cost, whether it was won, lost or abandoned. Free version-8 stages add no captain XP. Unversioned and pre-8 runs receive no retroactive captain XP. Already-settled runs of every version replay their stored receipt without recalculation.

### Fleet Directory And Rankings

Register the session-only, uncached `GET /api/user/gacha/fleet`. Use `board=directory` for open guilds (12 per page, pass `next` as `after`). The five ranking boards are `power` (active squad), `collection` (distinct owned cards), `guild` (weekly contribution), `arena` (settled arena wins) and `stars` (dungeon stars). They return the top 50 and `my_rank`, `my_value`, `my_name` and, when available, `my_level`, including positions beyond 50. Ties use registration order. Public rows contain display aliases or guild names, the metric, optional captain level and up to six public cards (`team`: catalog ID, level and star only). Listed guilds also expose member count and the public recruitment code; private guilds remain unlisted. Players without a squad can still rank for collections and dungeon stars.

Dungeon stars sum the server's single best rating for each distinct story, hard and nightmare stage across all four chapters (up to `star_limit`, currently 156). Resource dungeons, rifts, repeated clears and sweeps add no extra stars. `InitializeGachaPower` also backfills the versioned fleet projections, including rows already indexed by a previous edition. It does not rewrite save documents or revisions. The independent `#rank-demo-stars` preview works without a session, and the same hash suffix supports each of the other four ranking keys. All demo identities and scores are fictional; the preview does not read or write rankings or rewards.

Auto-migrate `GachaPlayer` **before** running `InitializeGachaPower`: it also backfills the new derived recruitment and collection columns in batches using revision checks. No changes to stored game JSON, combat snapshot versions, or site balances are needed. Existing guilds stay unlisted until the founder opts in. The current client offers that choice on creation via `guild_create` with `enabled`; `guild_listing` changes it later. Reuse the existing guild member/leader transaction for `guild_supply` (`item_id=charts|timber|lamps`) and `guild_supply_claim` (`count=1|2|3`). Every intent uses the regular UUID receipt. Daily participation and weekly claims survive membership changes. Test fresh and upgrade schemas and repeat the backfill on SQLite, MySQL and PostgreSQL as part of the host matrix.

## Card Cultivation And Item Draws

`GET /api/user/gacha/profile` publishes `cultivation` (crews, weapons, milestones, shards, sockets), `equipment` (gear quality and star gates), `artifact_grades` (graded item progression), `training` (colours, effort, XP books and skill costs), `carry` (wearer sets) and `items` (item guide). Catalog entries are not inventory grants. Actions use the existing authenticated game route, normal UUID receipts and one transaction. A request may select what to spend; it never establishes ownership, inventory, experience, draw results or stats. These extensions add fields to game JSON, not SQL tables or a new wallet service.

- `star`, `potential_unlock`, `weapon_upgrade`, `weapon_awaken`, `use_manual`: `girl_id`. `weapon_engrave`: `girl_id` and `choice`. All members of a collection crew must be owned, even if none are deployed. R/SR/SSR cards have 2/4/6 bonds; bonuses to a given stat add up to the configured cap. Role-specific star passives apply in combat at 3/6/9/12 stars.
- `gear_upgrade`, `gear_auto`, `gear_star`: `girl_id` and `item_id` (`weapon`, `head`, `hands`, `body`, `feet`), not a target level or spend. The server computes payable steps for the selected slot from committed inventory. `weapon_level` is still the single source for weapon reinforcement; other levels and all gear stars live in the card’s `equipment` map. An absent entry means built-in, untrained gear, not a missing item.
- `shard_decompose`: `girl_id` and `count` in whole groups, after reserving unlock/next-star shards. Check `decomposition.item_id` and `decomposition.quantity` in the receipt for forge ore; historical receipts can still report `dust`. `salvage_dust` takes only `item_id` and converts one group of old dust per transaction. No dust substitutes for missing card-specific shards.
- `iv_roll` takes only `girl_id`. It spends forge ore and rerolls only the unlocked individual attributes; the values may decrease. `iv_lock` takes `girl_id`, `choice` (a stat key) and `enabled` and is free. Absent `iv_values` denotes an unrolled card, and `iv_locks` uses the stat order supplied by `cultivation.individual`. An unrolled card cannot be locked; a fully locked card cannot be rerolled.
- `draw_gems` and `draw_chips`: `count` of 1 or 10. Each roll spends one game ticket, chooses a template uniformly and independently rolls one of seven weighted qualities from `artifact_grades`. `artifact_loot` describes the actual stack. `convert_tickets` takes only `item_id` (`gem_ticket` or `chip_ticket`).
- `gem_socket`, `gem_remove`, `chip_remove`: `girl_id` and 1-based `count` (position). `gem_equip` and `chip_equip` also take `item_id`; `chip_lock` adds `enabled`. A stackable chip ID encodes its family and position, e.g. `aegis_3`. A lock protects the installed chip from both removal and replacement.
- `gem_level`, `chip_level`, `gem_rank`, `chip_rank`: `girl_id`, 1-based `count`, and exact equipped `item_id`. Level 1–120 increases base stats; rank 1–12 advances effects, subject to level gates. Old bare and `@N` stack keys remain valid and distinct; operations return canonical new keys where needed. Do not parse the labels, merge qualities or strip a suffix on removal. Legacy refine intents retain their compatibility path; new clients use the two explicit axes.
- `card_breakthrough`, `effort_promote`: only `girl_id`. Each breakthrough checks current level (not stars or peak level) against `grades[].plus_levels` or the next grade's entry `level`. With `n` previously completed steps including colour changes, charge `ink_base + ink_step*n + n*(n+1)/2` ink and `floor((n+1)/seal_interval)` seals together. Same-colour steps increment `training.plus`; crossing increments `training.grade` and resets only `training.plus` to zero. Neither step changes the card's level, XP or peak level. `effort_train` / `effort_train_max`: `girl_id` and `choice` of stat. Twelve stages have five bars (160 through 33,000 each), ten trainings per card per Beijing day. Migrate old stage progress once, proportionally, without granting materials.
- `card_feed`: `girl_id` and `materials`, a map of selected experience-book IDs and positive integer quantities. No XP or target level input. Validate the full offer, consume small books first, stop at the current captain level (at most Lv.150), and return `feed.{spent,experience,level_before,level_after}`. All card XP sources share the same nonzero Lv.0 threshold and veteran/master curve.
- `skill_point`, `skill_auto`: `girl_id` and `choice` of `normal`, `rage` or `passive`. `skill_buy`: `count` only. The account-wide pool `(points, tick_at)` gains one point per 20 seconds up to 20, using server time. A purchased point costs 120 game gold; automatic leveling never purchases points. Each learned step costs one point plus `30 + 8 × learned_rank` gold; effective skill rank is capped at the current card level, including zero.
- `carry_craft`: `item_id`; `carry_equip`: `girl_id`, `item_id`; `carry_remove`: `girl_id`. `carry_inventory` counts unequipped copies, `own.carry` is the one worn item. Replacement returns exactly one old copy. Only distinct items from the same set worn by deployed cards unlock its wearer passives.
- On upgrade, absent cultivation fields mean untrained/empty, with no compensation grants. Standard victorious runs issued under loot v5+ give armor materials, v6+ also gives route badges/training drops, v7+ also supplies new experience books and eligible vigil/contract progress. Retreats, special modes and old settlements do not retroactively mint them. Unversioned PvP retains pre-workshop combat; power v5 retains the original pairs; v6 retains its larger pair catalog and original gear; v7 freezes crews, held items and the original effort/skill/refinement scaling. Power v8 adds advanced training, graded artifacts, vigil formations and the active leader relic. Existing battle reports are not recalculated.

Run the existing power-index initialization on upgrade and ship all embedded catalogs with the Go game extension. Update the game backend and static files together; static-only changes do not require an image rebuild but cannot enable new authoritative actions.

### Vigil, Cases And Dispatch

`profile.vigil`, `profile.cases` and `profile.dispatch` publish rules, not entitlements. All writes use the existing authenticated action transaction, shared player lock and idempotency record. No new wallet, workers or tables. For every action below, `request_id` is still mandatory; the client must not submit a win, score, budget or reward.

| Intent | Parameters beyond action / request ID | Server check |
| --- | --- | --- |
| `vigil_bind`, `vigil_ascend`, `vigil_refocus`, `vigil_ground`, `vigil_stance` | `girl_id`; path `choice` for binding, aspect `choice` only at the second rite or refocus, `enabled` only for stance | Once bound, the path persists. Freeze witness and stance at run issue; verify the saved win, daily limit, currency, clues and strain when applicable. |
| `case_open`, `case_step`, `case_close` | Open: `item_id` (case template), optional owned `girl_id`. Step/close: issued run `item_id`, expected `count` (next act / completed acts), `choice` | The server freezes leader specialty, advances clues/pressure, records one outcome per run, limits two different cases and ten evidence units per day. `casebook.endings` survives the daily reset. |
| `relic_bind`, `relic_wear`, `relic_store` | First two: `item_id`; store: no extra fields | Require the exact archived ending and pay its evidence once. Only the first deployed unit gets both benefit and cost. |
| `clock_align` | `item_id` = issued Beijing date, next guess `count`, three-symbol `choice` | Match server-generated positional clues; reject duplicates, stale attempts and already-settled awards. |
| `contract_sign`, `dispatch_contract_claim` | Issued date in `item_id`; sign additionally takes owned leader `girl_id` and clause `choice` | Mutually exclusive signature. Only matching standard runs issued **after signing**, settled the same game day, advance the saved clause. Manual claim once. |
| `route_step`, `route_bank` | Issued date, next row / current depth as `count`; only step takes lane `choice` | Check adjacency and fuel before the move, bank only carried cargo, or bank automatically on arrival. No parcel on loss. |
| `mosaic_place` | Issued ISO `week` as `item_id`, expected page `count`, and `choice` e.g. `aab-acb-ccb` | Validate 3 connected triominoes, 1 core each, exactly 1 straight; three one-time pages per Beijing week. Invalid layouts do not spend anything. |
| `mirror_trial` | Issued date, expected try `count`, two distinct loaner IDs in `squad`, and tactic `choice` | Build a temporary standard roster, run the authoritative battle simulator with the date's conditions, verify the objective. Up to three tries; no character, stamina, E or standard dungeon rewards. |
| `cipher_guess`, `cipher_practice` | `item_id` = `day:round` (`round=0` for the daily); guess adds next attempt `count` and four-digit `choice` from `0`–`5`, not feedback | Server scores exact and misplaced symbols without counting a symbol twice. Five guesses on the rewarded daily board; four on each optional unranked practice board. |

Keep the cipher generation seed in the private player-document envelope only. Do not return it, answers or solver state in profiles, action receipts or ranked snapshots. Optional desks hide on older hosts without their corresponding profile rules. Do not enable new clients against an unpatched action handler.

## Guild Progression

Guild ownership remains in the leader's authoritative player document. All affected player locks are acquired in ascending user ID order. `guild_progress` belongs to the account: coins, weekly stage claims/purchases/participation, daily attempts and activity claims survive leaving or being kicked. Level thresholds are 0/1800/6000/14000/28000/50000/82000/126000/184000/260000 XP; capacity is `10 + 5 * (level - 1)`. Each harbor cargo lane accepts `member_limit * 7` contributions per week.

`growth_version=2` maps legacy XP into the same level and proportional progress within that level. Raid migration preserves completed stage bits and the remaining-HP proportion of the active boss; rounding leaves an unfinished boss alive. It does not repeat claimed rewards or invent personal damage attribution. The first three stage claim bits keep their meanings; five-stage claims and participation allow masks 0–31. New activity ledgers start empty.

The calibrated raid rules use `raid_version=3`. Existing version-2 five-boss documents (HP 300000/900000/2400000/5000000/9000000) migrate by remaining-life proportion; version-0/1 documents retain their supported migration path. Completed stages stay completed, an unfinished boss retains positive HP, and previously recorded total/personal damage, ranking contributions, attempts and claim markers are not multiplied. An already completed fifth stage stays complete. Fresh creation and weekly reset read the first boss HP from the same rule catalog; total damage bounds use the sum of the five rule HP values.

| Boss | Guild level | HP | Attack | Defense |
| --- | ---: | ---: | ---: | ---: |
| 暗礁守卫 | 1 | 1,300,000 | 550 | 80 |
| 深潮统领 | 3 | 8,500,000 | 1,800 | 160 |
| 深渊主宰 | 5 | 42,000,000 | 5,000 | 280 |
| 永夜司钟 | 7 | 135,000,000 | 10,000 | 450 |
| 终焉潮皇 | 9 | 360,000,000 | 18,000 | 700 |

`guild_attack` costs 10 stamina with two attempts per Beijing day and a hard 20-round limit. Its `guild_battle` contains the server report and credited damage. Playback and exit make no second settlement request. Five sequential bosses unlock at guild levels 1/3/5/7/9; authoritative HP, mechanisms, descriptions and reward resources are returned in `guild.raid_stages`. Every accepted attack grants 30 guild XP, 25 guild coins and 3 `training_ink`, including attempts that do not defeat the boss. First stage clears add 200/400/700/1100/1600 shared XP. `guild_claim` accepts optional `count=1..5`; a member must have participated in that stage in this guild, and can claim the defeated stage only once per account per week across guilds. Rewards use each stage's `resources` map plus `coins`; old `token`, `gold` and singular material fields are compatibility fields and are no longer the reward source.

The guild raid and damage boards reset at Beijing Monday 00:00, using the server ISO week. Reads show the reset view immediately; the locked guild mutation persists the new week's state. Guild XP, roles, coins and inventory persist. `guild.raid_damage_ranking` returns `{week, stage, stage_name, all_cleared, entries}` for the current boss. Each entry has `{rank, user_id, name, damage, attempts, member}`. Damage comes only from committed server reports; receipt replays do not increment it. Entries sort by damage descending and user ID ascending on ties. The next boss has a separate board; after all bosses are cleared, the fifth board remains visible. Departed members retain their recorded name and damage with `member=false`. Legacy guilds start the board with new attacks, without assigning their earlier unattributed damage to members.

`guild.activity` contains daily `tasks`, shared weekly `missions`, XP `sources` and the account's weekly `contribution`. Daily claims use `guild_checkin` (no extra fields) or `guild_activity_claim` (`item_id=sortie|training|arena`). Their authoritative goals and rewards are:

| Source | Server-observed goal | Guild XP | Personal coins |
| --- | --- | --- | --- |
| Check-in | One daily claim | 20 | 20 |
| Sortie | Win three stamina-costing dungeon battles | 60 | 45 |
| Training | Three successful supported material-spending cultivation actions | 40 | 30 |
| Arena | Two ranked challenges, either result | 50 | 35 |
| Harbor supply | One daily cargo delivery | 40 | 35 |
| Raid | Each accepted raid, at most twice daily | 30 | 25 |

Daily goals are counted from server outcomes and paid actions, never from client counters. Shared weekly goals are 20 deliveries, 30 raids and 50 claimed non-check-in activity tasks; completion grants 800/1200/1800 shared XP once. A member with a contribution in the current guild can use `guild_mission_claim` (`item_id=supply|raid|active`) to collect that goal's resources and coins. The account's weekly mission claim mask survives changing guilds; claiming does not award the shared XP again.

`guild_shop` takes an issued SKU in `item_id`, with per-account weekly caps. SKUs are `tokens`, `ore`, `crystal`, `stardust`, `manual`, `ink`, `seal` and `core`; render costs, amounts and eligibility from `guild.shop`. `guild_role` takes target `opponent_id` and `item_id=deputy|elder|member`; only the leader can appoint, with maxima of two deputies and four elders. Leaders and deputies can manage listing.

`guild_kick` takes only target `opponent_id` besides the regular action/request ID. The leader may remove any other member; deputies may remove members and elders, but neither the leader nor another deputy. Nobody can kick themselves. `members[].can_kick` describes the requesting user's permission. The operation locks leader, actor and target rows in sorted order, rechecks both memberships and roles, removes the roster entry and clears the target's `Guild` in one transaction. The target's account progression and claim markers remain. A stale request cannot clear membership in a different guild, and replaying a successful receipt does not kick someone again after rejoining. All guild intents reject unrelated parameters.

## Ranked Arena

Install the four `GachaLadder*` tables and the session-scoped read routes from `integration/`. After the host account tables exist, `InitializeGachaLadder` seeds 100 NPCs on an empty ladder and retains existing seats on subsequent runs. Run the host worker in `integration/gacha-ladder-worker.go.example` after migration. It settles at Beijing 21:00; a missed tick is caught up before the next successful rank mutation. Neither ranked challenges nor their claims touch E balances or the E matchmaking allowance.

`ladder_join` opts in to asynchronous defense and saves the current squad. `ladder_defense` refreshes the snapshot. `ladder_leave` removes the active seat, but preserves earned claims, battle history, daily usage and cooldown. Rejoining appends to the tail. `ladder_challenge` verifies `opponent_id` and `count` (expected position); a stale quote changes no balance, rank or attempt. `ladder_best_claim` takes `item_id=best:100|best:50|best:20|best:10|best:3|best:1`, credits tokens immediately, and commits inventory, the claimed marker and UUID receipt together. Render its availability from `best_rewards[].can_claim`; each tier also returns `claimed` and `delivered_at`. The legacy date-based `ladder_claim` only synchronizes daily mail and does not directly grant a reward.

Under the SQL gate, settlement removes disabled and missing accounts and compacts positive ranks **before** pricing any new entitlement. The worker and request-time catch-up use the same order, so a leaderboard read is not required for promotion to count. During downtime, overdue unissued entitlements use the eligible ranking observed when settlement resumes; the host does not supply historical account status. Existing mail is not rewritten. A valid last-minute challenge can change the cutoff rank; a challenge processed after the cutoff settles the due reward before swapping ranks. Free ranked play has a rank neighborhood, not a power-difference restriction; per-match consent and symmetric power checks for E play still apply.

## Validation In A Host

Run these commands from your **own compatible host checkout**, inside the shared capped test container:

```sh
GOMAXPROCS=1 go test -p 1 -parallel 1 ./model ./controller -run TestGacha -count=1
```

`model/gacha_game_test.go` accepts `TEST_MYSQL_DSN` and `TEST_POSTGRES_DSN` for your own disposable databases. Set `GACHA_REQUIRE_DB_MATRIX=1` when validating all three engines so missing configured database targets fail instead of silently skipping. Never point these variables at production data.

The root game package has no host `go.mod`, complete database bootstrap or host image. Running `go test ./...` directly in this directory is therefore not a supported standalone-server test. Browser mocks and source audits do not substitute for host integration and database tests.

Ranked reward migration adds `best_reward_rank` to entries and `kind`, `token`, `delivered_at` to the existing ladder reward ledger. Run the integration migration before serving the new model. Historical rows use stable `best:<rank>` keys and represent manual-claim entitlements; daily rows retain their date keys and represent automatic mail. Legacy claimed rewards are not redelivered. A historical row already delivered to mail remains mail-only, including an aggregated attachment, so it cannot also be claimed directly. Pending daily rows retain their original materials. Best milestones grant 200/400/600/1000/1500/2500 tokens at 100/50/20/10/3/1; daily tiers add 300/240/180/120/80/50 tokens at 1/3/10/50/100/remainder. Game tokens do not touch host wallet balances.

The ranked worker records durable daily entitlements. Loading a player delivers only pending daily outbox entries to the shared mailbox and persists inbox changes with delivery marks in the same transaction, including action-replay paths. Full mailboxes aggregate automatic rewards; if the aggregate is also full, pending daily entries remain for later delivery without blocking login. New historical milestones are never automatically mailed. Deleting shared mail or leaving/rejoining the ladder does not reset reward eligibility. Validate fresh, upgrade and replay behavior against the host's SQLite, MySQL and PostgreSQL adapters before deployment.

For this update, Go files were formatted and parsed with `gofmt`; a compatible host build and SQLite/MySQL/PostgreSQL execution were unavailable. Added Go regression cases are not reported as executed. Frontend and mock checks do not close that integration gap.
