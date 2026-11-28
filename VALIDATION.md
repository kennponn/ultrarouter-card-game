# Game-Only Export Validation

## 玩家经济验收 · 2026-11-28

**结论：** 按下述竞技场第 50 名成熟账号路线，每天平均收入 **12,233 金币、500.2 代币**，培养清单需要 **11,994 金币、496 代币**。**62.7%** 的模拟周能付清整张清单；若推迟一周中一次三锁洗练，**99.6%** 能付清其余项目。漏打竞技场最缺代币，漏打护商最缺金币。压力来自冲榜培养，不要求新手付站点 E，也不是每个账号必须打卡才能玩的硬门槛。

**复算。** `npm run economy:simulate -- --compare` 读取正式关卡、任务、商店、培养和世界池配置，不访问账号或外部服务。每条路线 2,048 个固定种子、连续 7 天，共 **13 条路线、186,368 个模拟玩家日**。用当前公会收入回测最初培养价格，金币/代币周覆盖率中位数为 149.6%/231.7%。公会参战奖提高后，三锁洗练从 195 调至 210；首次学技能仍是 65 金币，无锁洗练仍不花代币。

| 一周内的选择 | 金币覆盖率中位数 | 代币覆盖率中位数 | 兑换宝箱前获得的限定券/周 |
| --- | ---: | ---: | ---: |
| 全勤培养，竞技场第 50 | 101.9% | 100.9% | 24 |
| 保住第 50 名但不打五场 | 87.6% | 87.8% | 17 |
| 不参加排名竞技 | 85.6% | 62.8% | 3 |
| 不打护商金币本，体力改刷关 | 74.7% | 102.1% | 24 |
| 跳过港口活动与集章册 | 100.6% | 97.5% | 14 |
| 不参加公会 | 106.1% | 76.5% | 24 |
| 不打远征 | 101.6% | 88.7% | 24 |
| 竞技场第 3，追更高技能与四锁属性 | 97.4% | 102.4% | 38 |
| 正常养成，但三个币店都不换体力包 | 70.6% | 85.7% | 24 |
| 不买奇物、强化石、培养石和经验书，优先体力 | 120.0% | 109.3% | 24 |
| 已有寻宝女王，且每场普通刷关都触发满额金币加成 | 121.1% | 100.9% | 24 |
| 活跃公会额外完成三档共建周目标和航标宝箱 | 102.6% | 112.4% | 24 |
| 每日三档代币商店体力买满，剩余体力优先刷噩梦裂隙 | 144.0% | 84.6% | 24 |

**模拟口径。** 假设已解锁资源本、双队试炼和公会；每日 09/14/21 点上线，夜间自然回体受 120 上限限制。不把主线首通、首次晋榜、礼包码、站点 E、旧存款或不确定的公会首领首通计作日薪。每周完成 10 层财运远征、周首领和双队；每天打六种资源本各三次、两次公会讨伐、五次排名挑战（假定胜率 55%），完成签到、以材料共建及相应公会日常。余力先打普通裂隙，再打低耗主线，假定刷关胜率 97%；双倍卡优先留给远征。

**玩家怎么买。** 四种专属币分别记账，不能互换。副本币优先补实际缺少的洗练剂；培养路线还会选经验书、努力培养石、奇物和强化石，公会币按实际余额和库存换代币，三店剩余币才买体力。基准路线平均每天多拿 **146.4 体力、131.4 公会商店代币**。两池分别先领免费单抽，再用本池所得券；仍缺张数才用星锻矿换对应券。基准路线平均用矿换 3.6 张/日，两池各十抽可不花游戏代币。如果放弃培养采购、尽量买体力，平均每天多拿的体力可提高至 230.2；不会同时拿到那些培养物品。

**买满代币体力的对照。** 不额外付费刷新，三个时段共花 180 代币买 90 体力；做完日常，把余力优先刷 `n1`（24 体力，基础掉落 672–960 金币），不足一场再刷 `s1`。这一成熟账号路线平均每日收入 **17,277 金币、572.2 代币**；同一培养清单加购体力后需要 **11,994 金币、676 代币**。能存下金币，但每日代币平均缺约 104；没有把换来的体力当成免费资源。不能据此推断所有服、所有阵容或囤积体力的玩家都达不到更高收入。

清单按一天五次技能升级（当前技能约 40 级）、十次第三段努力训练、五次普通单抽、四次洗练（分别锁 1/2/3/3 项），以及一周三次突破。排名前三路线练约 47 级技能、包含四锁洗练。覆盖率是周收入/目标消耗；两钱包可负担率允许按真实商店价格和库存补缺少的金币，不允许反向换代币。商店按每日三段库存和跨日钱包结算，不精确模拟每次点击顺序。模型也不模拟在线匹配、真实战斗和全部培养材料的期初储备；它不代表新手或所有阵容，更不能证明每个玩家都必须做尽每一种玩法。

**强者世界。** 用同一份正式曲线另做了 1,171,456 次模拟抽取（每种条件 8,192 份周样本，比较时保持种子一致）。有 24 张限定券、及时领箱时可抽 26 次：未上榜平均 24.30 片，第 20 名平均 27.30 片。日竞技第一并集齐日常和活动可攒 45 张，含返券抽 49 次；若独立的世界榜也是第一，总计平均 83.94 片，约 90% 的样本当周达到 80 片，而非保送整卡。周末新得的券留给下次开放；这不是新服首周保底。

**本机实际验收与边界。** 97 项玩家界面断言通过，另做了强者世界、跨周领奖、商店售罄与开包、任务积分、扫荡、两池免费抽、培养付费、排名重试及账号切换等定向 DOM 检查。枚举了 512 种任务完成组合和 4,096 个宝箱资格/已领判断。重复关卡即使按最高双倍代币计算，代币买体力也不能回本；副本币礼包同样不能自我补足。防守性审阅检查了会话身份、操作字段白名单、绑定参数、服务端价格/掉落、库存上界和 UUID 回执；没有对生产服务进行攻击测试。

**手机触屏操作。** 用独立 Edge 156 配置在 390×844、DPR 2 下加载真实前端和隔离的虚构回执。0 张限定券时点单抽、十连都不提交；榜单展示两位玩家并标出自己；点击 3 分宝箱只发一次请求，确认后变为「已领」。点击竞技场商店显示测试账户的 110 竞技场币及其他商店入口。世界页与商店页均无横向溢出，无 JavaScript 运行错误。这是浏览器交互验收，不是线上账号或数据库验证。

**公会加奖后的复验。** 四个专项文件（公会成长、公会界面、强者世界、经济）共 24 项通过，另有 5 项锁定、回收与培养筛选用例通过。一键 mock 的实时时钟与虚构开放日均通过 HTTP 验收，逐档首通的币和所有材料与预览相符。Edge 实测主页进入公会打一场、查结算及个人伤害榜，也检查了召唤入口与角色工坊的四种屏幕尺寸。截图和验收步骤见 [本地 PR 说明](docs/pr-guild-rewards/README.md)。本轮未运行正式 Go 或数据库回归，也未复跑全部副本的真实战斗矩阵。

上一轮记录：港口活动、调查、值守、奇物、公会协作的五项定向检查和 20 项 Python 静态预览与发布边界测试通过（首次有一次本机回环连接中断）。发布扫描通过 303 个文件，4 个匹配均为已审核的测试数据或标签；没有加载私人环境标记。

已补充真实宿主的 Go 回归：两账号库存隔离、临界时间补货、重复领取、失败回滚、21 名玩家跨周结算、旧邮件升级、无资金时不计进度等。**本机没有 Docker、兼容宿主模块和一次性 MySQL/PostgreSQL 实例，未运行 Go 编译或三库矩阵；没有部署。** 上线前在隔离宿主用 `sh scripts/build-half-cpu.sh --run` 启动测试容器，设 `GACHA_REQUIRE_DB_MATRIX=1`，串行执行 `GOMAXPROCS=1 go test -p 1 -parallel 1 ./model ./controller -run TestGacha -count=1`。不要把 DOM 回执测试或经济抽样当作实际数据库并发已经通过的证据。

## Consolidated Review · 2026-11-27

PR #2 contains the fixes and history from PRs #1–#3. The [consolidated report](docs/pr-2-evidence/README.md) records the pinned baseline, Git tree comparison, three real database engines (15/15 focused cases), same-file SQLite upgrade and mail checks, 100 UI assertions, 129 Node tests, 19 Python tests, the release audit, and nine browser screenshots. The dated reports below describe their original revisions, not the consolidated result.

## Captain, Mailbox, Dungeons And Rankings · 2026-11-27

- **24 focused Node DOM tests and 93 existing server-UI assertions passed.** Coverage includes visible captain XP and the avatar dossier, Beijing meal boundaries, five daily tasks, three dungeon difficulties, star chests, dated sweep intents, fourteen event previews, guild cooperation, five ranking categories and the separate fictional ranking demo. Mail tests verify intent-only requests, no optimistic grants, duplicate-click suppression and late-session responses. Node test files ran serially with one V8 pool worker; the full `npm test` command was not rerun for this revision.
- **19 Python preview and release-boundary tests passed.** The static preview serves the game assets and rejects gameplay writes; private host source, deployment files and local data remain outside the release boundary.
- **102 Edge DOM and input checks passed:** 51 at 1,366 × 900 and 51 at 390 × 844. The actual HTML and scripts display captain XP 120/440, the avatar dossier, 8/3/2 chapter stages, three independent chest states, six resource dungeons, eight fictional ranking rows with four portraits each, team details, mailbox attachments and the gift-code form. There were no horizontal overflows, console/runtime errors or same-origin HTTP failures. Forty-two distinct game image assets returned HTTP 200.
- Edge wrote **12 screenshots** with the game's own artwork and fictional data for [the update announcement](docs/pr-harbor-voyages/README.md). Screenshot files were not opened, decoded or visually inspected by the agent; only DOM geometry, resource status and file metadata were checked. The isolated loopback fixture is read-only and is not part of the PR or release archive.
- The dungeon renderer and floating controls follow the supplied frontend reference source; [the layout comparison](docs/pr-harbor-voyages/dungeon-layout.md) maps the implementation. The local reference client previously completed ordinary sign-in, server selection, role login and sync using code and protocol inspection. No original-client image analysis, credentials or session data are included.
- **25 changed Go files passed syntax parsing and normalized `gofmt` checks.** All 12 embedded JSON catalogs parsed, and the HTML assets and announcement links resolve to existing files. A text-only credential-pattern scan covered 140 release/source-document files with four unchanged, hash-reviewed synthetic matches and no unreviewed findings; it did not read image contents. The scan used the built-in patterns, with no private-marker configuration loaded.
- Go model and host-matrix regressions cover stamina-only captain XP plus daily mail, failed/abandoned/replayed runs, reward delivery and claim capacity, account isolation, deleted gift-code receipts, midnight/full-mailbox retention, star ranking backfill and a mailbox larger than 64 KiB. **Go compilation and SQLite/MySQL/PostgreSQL execution require the compatible host module in the capped container and were not run on this Windows checkout.** No deployment was performed.

## Workshop, Investigations And Seven Dispatch Desks — 2026-11-26

- `npm test` passed: **93 server-UI checks and 104 Node tests/subtests**, no failures, skips or cancellations. The suites ran serially on one logical CPU, separately from the browser. Coverage includes Lv.0 ownership, the Lv.150 XP cap, material-only feeding, skill recovery and auto-spend, 12 effort stages, seven artifact qualities and dual growth axes, vigil, cases, all seven desks, ambiguous network results and receipt replay.
- Mobile (390 × 844) and desktop (1,366 × 900) Edge runs used isolated, fictional in-memory accounts and the actual frontend with external and bitmap requests blocked. Checks covered timed recovery, 119→120 / 11→12 item growth, six-branch rite selection, case outcomes, and the seven desk interfaces. The mirror check selected loaners and a tactic without altering the real squad; the browser fixture **did not simulate the official battle**. The PR walkthrough links 35 screenshot crops. PNGs were written, not opened or read back.
- The change touches 38 Go sources. `gofmt` was not available in this Windows checkout, so formatting was not re-run here. The text-only release allowlist check covered 200 files: four previously reviewed fixture matches and no new findings. All ten embedded game catalogs parsed; the dispatch recipes, loans, ending prerequisites and reward IDs also cross-checked against the catalogs. No media scan or release archive was generated.
- Go regression cases include frozen v7 teams, same-day and stale intents, puzzle solvability, real loaner battle simulation, reward capacity rollback and receipt replay in the host database matrix. **Go compilation, Go tests and the SQLite/MySQL/PostgreSQL matrix were not run here:** this game-only checkout lacks the compatible host module and Docker runtime. Run them in the shared capped container before a backend merge or deployment. No production access, deployment or PR submission occurred.

## Long-Term Card Cultivation — 2026-11-26

- `npm test`: 119 UI assertions and 85 Node tests/subtests passed, serially on one logical CPU. Coverage includes same-colour plus levels versus colour promotion, all-stat effort gates and carry-over, skill budgets, coloured gear, over-limit costs, refinable assets and their exact return stacks, wearable recipes and three-member ownership crews. Rejected or missing responses do not change the displayed inventory.
- Headless Edge exercised the actual frontend with isolated, fictional in-memory accounts, all external/media requests blocked. Four mobile and ten desktop PNGs were written, not read back. The checks advanced white +2 to green +0, retained effort after promotion, exhausted the shared skill budget, removed and re-equipped +6 items, activated an undeployed crew, and checked each 12→15 equipment payment. The final 15-star button was hit-tested with and without reduced motion; a real pointer click with reduced motion charged 450 rivets and 160 route badges and disabled further upgrades. See `docs/pr-cultivation-v7/README.md` in the Git checkout for the nine walkthroughs; these are UI/receipt checks, not production battle or database tests.
- All 25 changed/new Go files parsed and matched `gofmt` after normalizing line endings. Regression cases were added for authoritative costs, capacity, repeat requests, battle effects and historical snapshots. Go compilation and the SQLite/MySQL/PostgreSQL host matrix were **not executed**: the export does not contain the host Go module and this environment has no Docker runtime. Run them in a disposable compatible host before merging/deploying the backend.
- A text-only release check covered 82 allowlisted files, including the six embedded catalogs; the four previously reviewed synthetic fixture matches were unchanged, with no unreviewed findings. Private guides, the mock service and PR screenshots are not in the game release allowlist. No media contents were read and no deployment, archive or complete media audit was performed.

## Character Dossier And Five-Slot Gear — 2026-11-26

- `npm test` passed: 119 UI assertions and 76 Node tests/subtests, including 20 workshop cases. Node ran serially on one logical CPU. Checks cover roster filters and wearer-specific navigation, five illustrated equipment slots, the maximum affordable upgrade, star gates and server-only charges, all three equipped-set thresholds, the seven quality emblems and five gem cuts, and old-host fallback.
- The catalog and UI checks cover all 28 cards, the 2/4/6 relationship limits, ownership without deployment, the per-stat bonus cap, role-specific 3/6/9/12-star milestones, personal-shard reserves and old-dust recycling. They also check that stage previews use the four server drop tables and that reward icons only decorate the quantities in a returned receipt.
- Headless Edge with images blocked exercised the new dossier at 320×720, 390×844 and 1366×900. The selected character and five individually hit-tested slots all fit on the first screen (the equipment row ended at y=354, 633 and 625 respectively). Checks selected all five slots, paid for a single upgrade, auto-upgraded to +10, starred a headpiece to activate the two-piece bonus, exchanged old dust, and returned from both draw pools to the original wearer/slot or bag. Pages and dialogs had no horizontal overflow, page errors or image responses. These were isolated fictional in-memory sessions; no screenshots or real player accounts were used.
- The 19 changed/new Go files passed `gofmt` parsing and normalized formatting checks. Game-only host tests were added for equipment costs, retries, drop versioning, PvE/PvP bonuses, v5 frozen defenses and all five role routes. Go compilation and SQLite/MySQL/PostgreSQL integration were **not run**: this export does not include the host module, containers or disposable databases. There was no deployment.
- A text-only scan covered 70 allowlisted source/metadata files, with four unchanged, hash-reviewed synthetic fixture matches and no unreviewed credential-pattern findings. No private deployment-marker scan, media read, full media audit or release packaging was performed.

## Card Workshop — 2026-11-26

- `npm test` passed: 119 existing UI checks and 68 Node tests/subtests, including twelve cultivation cases. These cover the 12-star cap, reserved shards, immediate collection bonuses after unlocking an undeployed card, fixed weapons, cumulative gem/chip sets, individual-value rounding and locks, intent-only requests, ticket budgets, server-only receipts, and returning from a draw to the selected equipment slot. The Node suites ran serially on one logical CPU.
- In headless Edge at 390×844 and 1366×900, the roster, workshop panels, bag and item draws fit horizontally. Mobile interaction checks exercised 11→12 stars, opening a fourth gem socket, activating a three-piece set, replacing/removing items, chip protection, both ten-draw pools and ticket exchange. The individual-value radar and all six lock controls rendered at both widths. Locking attack at 31 preserved it through a three-ore reroll; an untrained card could not lock a stat before its first two-ore roll. No page errors were reported. These runs blocked image requests and used no screenshots.
- The Edge runs used a separate loopback process with fictional in-memory fixtures. They verify controls and receipt handling, not the real battle simulation, odds distribution or database concurrency. The normal published static preview remains read-only.
- Changed Go sources passed syntax/format checks with `gofmt`. Go tests were added to the host's existing matrix for migrated three-socket saves, material and ticket limits, protected rolls, transaction retries, frozen defenses and replayed requests. The credential-pattern scan of 64 allowlisted text files produced no findings beyond the previously reviewed fixture matches. **Not executed here:** Go compilation, SQLite/MySQL/PostgreSQL integration, or production deployment; this export has no host module, Docker or disposable databases. No full media audit or release package was run for this change.

Validation date: 2026-11-25. This export contains only the card game; no complete host application or production deployment is included.

## Checks Actually Run

- Windows, Node.js 24.22.3: 93 browser-interface assertions passed.
- Node's test runner: 56 tests/subtests passed, zero failed, covering browser login gates, server-owned state, intent-only requests, expiry/settlement, readability and retained upstream attribution.
- Python 3.13.2: 19 preview/release tests passed, including anonymous gameplay rejection, no local wallet writes, static-source/environment/data blocking, path traversal, Host/Origin checks, private-marker rejection, complete-host exclusion and ZIP hash manifests.
- Python/JSON syntax checks, shared CPU-wrapper shell syntax and game-only CI YAML/boundaries passed. The public CI workflow was inspected locally, not claimed to have run on GitHub.
- The release source audit checked 148 selected game files against 14 known private deployment markers. Four synthetic fixture matches are retained by exact hash across three reviewed files; credential values are not recorded in the review manifest.

Local browser test processes were started serially with affinity `0F` on an eight-CPU machine, limiting their process tree to four CPUs. Node used one V8 pool worker and sequential test-file execution. No production build or service CPU setting was changed.

## Game And Host Boundary

The 14 game backend source/catalog/test files are byte-identical to the previously validated game implementation. Earlier full-host validation exercised the game using real SQLite, MySQL and PostgreSQL; the full host, its images and broader platform tests are intentionally not distributed in this game-only package.

This export did **not** build or run a standalone Go game server. The existing Go integration requires the host session, wallet, cache and database contracts in `backend/INTEGRATION.md`. Browser mocks, a source scan and matching hashes are not a substitute for validating an adapter to a different host.

The original local game files retain their recorded hashes. The earlier oversized full-site export was preserved privately instead of deleting its changes. This correction used local files only and did not connect to or modify production services.

## Publication And Artwork

The generated source ZIP includes a per-file SHA-256 manifest and excludes host-platform code, production connections, environment/data files, logs, backups, installed dependencies and Git history. Known private markers are held outside the game-source directory and are not packaged.

The game's 28 character provenance entries and 95 WebP files are retained. Artwork permission remains a separate question documented in `ASSETS.md`; source validation does not certify an unconfirmed image license. At the 2026-11-25 validation snapshot, no public repository had been uploaded. Publishing this source does not deploy a production service. GitHub Actions results are reported by the workflow itself, not by the earlier local validation snapshot.
