# 上线后稳定性与题目质量防护 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在不修改 backend 的前提下，为微信小游戏前端增加服务端题目质量拦截、线上诊断记录和结算幂等保护。

**Architecture:** 新增无副作用的 `question_quality` 校验模块和本地 `telemetry/feedback` 服务。模式控制器在接收题目合同时校验题目，GameApp 记录事件并为结算建立幂等键；现有 storage、api_client、GameApp 和 mode_controller 架构保持不变。

**Tech Stack:** 微信小游戏 Canvas、原生 JavaScript/CommonJS、现有 `puzzle_generator.js`、`storage.js`、Node.js 审计脚本。

**Spec:** `docs/superpowers/specs/2026-09-10-post-launch-stability-question-quality-design.md`

## Global Constraints

- 只修改 `frontend/wechat_game`，不修改 `backend`、`_archive_godot` 或 `frontend/taptap_game`。
- 不新增 mock 数据，不在正式环境回退到本地奖励、本地结算或本地随机题。
- 保留 `src/app/game_app.js`、`src/input/touch_geometry.js` 和现有未提交修改。
- 所有现有后端响应仍通过 `api_client.js` 的 `code/message/data` 包装解析。
- 不把 AppSecret、头像、昵称或完整解法上传到 telemetry。

---

### Task 1: 建立题目质量校验模块

**Files:**
- Create: `frontend/wechat_game/src/core/question_quality.js`
- Create: `frontend/wechat_game/tools/question_quality_unit_audit.js`
- Modify: `frontend/wechat_game/README.md`

**Interfaces:**
- Consumes: `puzzle_generator.solveDetailed`, `puzzle_generator.executeSteps`,题目合同和 rules。
- Produces: `numberKey(numbers)`, `validatePuzzle(record, options)`, `validatePuzzleList(records, options)`。

- [ ] **Step 1: 写失败测试数据和断言**

在 `question_quality_unit_audit.js` 中加载 `question_quality.js` 与 `puzzle_generator.js`，断言：

```js
const valid = quality.validatePuzzle({ numbers: [3, 5, 5, 9], target: 24 });
assert.strictEqual(valid.ok, true);
assert.strictEqual(valid.key, '3,5,5,9');
assert.strictEqual(quality.validatePuzzle({ numbers: [1, 1, 1, 1], target: 24 }).ok, false);
assert.strictEqual(quality.validatePuzzle({ numbers: [3, 3, 8, 8], target: 24 }).ok, false);
```

- [ ] **Step 2: 运行审计确认模块尚不存在或断言失败**

Run: `node tools/question_quality_unit_audit.js`

Expected: FAIL with missing module or failed assertion。

- [ ] **Step 3: 实现纯函数校验**

在 `question_quality.js` 中实现以下行为：

```js
function numberKey(numbers) { return (Array.isArray(numbers) ? numbers : []).map(Number).join(','); }
function validatePuzzle(record, options = {}) { /* 返回 {ok,key,puzzleId,reason,solutionSteps} */ }
function validatePuzzleList(records, options = {}) { /* 返回 {ok,records,issues,keys} */ }
```

校验 4 个整数、`target === 24`、1～13 范围（可由 `minDigit/maxDigit` 覆盖）、可选服务端 `solutionSteps` 可执行、完整求解至少一条、规则禁止负数/小数时严格执行。返回值必须是新对象，不修改传入 record。

- [ ] **Step 4: 运行单元审计确认通过**

Run: `node tools/question_quality_unit_audit.js`

Expected: `QUESTION_QUALITY_UNIT_OK`。

- [ ] **Step 5: 在 README 记录调用边界**

加入“题目质量校验”小节，说明服务端题目进入 `this.puzzles` 前校验、失败不使用本地题目替代，以及本地审计命令。

### Task 2: 加入本地线上事件记录和反馈服务

**Files:**
- Create: `frontend/wechat_game/src/services/telemetry.js`
- Create: `frontend/wechat_game/src/services/feedback_service.js`
- Modify: `frontend/wechat_game/src/services/diagnostics.js`
- Modify: `frontend/wechat_game/src/services/storage.js`
- Create: `frontend/wechat_game/tools/telemetry_audit.js`

**Interfaces:**
- Consumes: `storage.appendErrorLog`, `storage.getErrorLogs`, `storage.getDiagnostics`。
- Produces: `telemetry.record(event, context)`, `telemetry.recent(limit)`, `telemetry.summary()`, `feedback.buildDiagnosticText(context)`, `feedback.copyDiagnostic(context)`。

- [ ] **Step 1: 写审计断言**

在 `telemetry_audit.js` 中提供最小 `wx` 内存存储，调用 `telemetry.record('question_quality_failed', { mode: 'friend', puzzle_key: '3,3,8,8' })`，断言事件可读取、字段被截断、不会包含 `solution`、`avatar`、`nickname`，并验证 `feedback.buildDiagnosticText` 返回短文本。

- [ ] **Step 2: 运行审计确认失败**

Run: `node tools/telemetry_audit.js`

Expected: FAIL because the new modules do not exist。

- [ ] **Step 3: 实现 telemetry**

使用当前账号 storage 命名空间，事件最多保留 40 条；事件名只允许 `[a-z0-9_:-]`，字符串字段最多 160 字符，总上下文仅保留 mode/screen/stage/statusCode/puzzle_key/run_id/match_id/reason 等白名单字段。写入失败时静默返回 `false`。

- [ ] **Step 4: 实现 feedback**

生成包含小游戏名、时间、模式、页面、最近错误阶段和题目指纹的文本。调用 `wx.setClipboardData` 时兼容成功/失败回调，失败返回 `{ ok:false }`；没有微信 API 时也不能抛异常。

- [ ] **Step 5: 扩展 diagnostics 摘要并运行审计**

让 `diagnostics.report()` 包含最近事件数量和最近质量失败时间，但不包含隐私字段。运行：`node tools/telemetry_audit.js`，Expected: `TELEMETRY_OK`。

### Task 3: 在模式启动边界拦截无解题目

**Files:**
- Modify: `frontend/wechat_game/src/modes/mode_controller.js`
- Modify: `frontend/wechat_game/src/app/game_app.js`
- Modify: `frontend/wechat_game/src/core/question_service.js`
- Create: `frontend/wechat_game/tools/question_contract_audit.js`

**Interfaces:**
- Consumes: Task 1 `question_quality.validatePuzzle` 和 Task 2 `telemetry.record`。
- Produces: 模式启动前拒绝非法题目；`game_app` 中 `validatePuzzleContract(records, mode)` 统一记录失败并设置可重试状态。

- [ ] **Step 1: 写合同审计**

在 `question_contract_audit.js` 中验证：campaign/daily/endless/friend 的有效题目通过；包含 `[3,3,8,8]` 的服务端列表失败；失败列表不应被写入运行时 `puzzles`。

- [ ] **Step 2: 运行审计确认失败**

Run: `node tools/question_contract_audit.js`

Expected: FAIL because the contract guard is not available。

- [ ] **Step 3: 增加 GameApp 统一校验入口**

实现：

```js
validatePuzzleContract(records, mode, options = {}) {
  const result = questionQuality.validatePuzzleList(records, options);
  if (!result.ok) {
    telemetry.record('question_quality_failed', { mode, reason: result.issues[0].reason, puzzle_key: result.issues[0].key });
    this.status = '题目校验失败，请重试';
  }
  return result;
}
```

入口失败时清空 `currentPuzzle/cards/puzzles`，保持现有模式入口或好友房等待页，不调用本地奖励。

- [ ] **Step 4: 接入四种服务端题目路径**

在 `startCampaign`、`startDaily`、`startEndless`、`startFriend` 接收并规范化服务端题目后调用统一入口；只有 `result.ok` 才赋值给 `this.puzzles` 并开始计时。好友对战仍优先使用房间合同，不改变 reconnect。

- [ ] **Step 5: 增加 question_service 的题目摘要元数据**

为现有 `withGenerationMeta` 或对应记录路径补充 `question_key`，服务端已有 `puzzle_id/question_hash` 时原样保留，不修改题目数字。

- [ ] **Step 6: 运行合同审计**

Run: `node tools/question_contract_audit.js`

Expected: `QUESTION_CONTRACT_OK`。

### Task 4: 防止结算重复提交并记录失败原因

**Files:**
- Modify: `frontend/wechat_game/src/app/game_app.js`
- Modify: `frontend/wechat_game/src/services/api_client.js`（仅在现有提交函数边界补充错误标识，不改响应包装）
- Create: `frontend/wechat_game/tools/settlement_idempotency_audit.js`

**Interfaces:**
- Consumes: 现有 `submitCampaignLevelCompletion`、`submitDailyChallengeCompletion`、`submitEndlessRun`、`submitFriendLeaderboard`。
- Produces: `settlementKeys` 和 `settlementState`，同一请求键只提交一次；失败后允许同键重试。

- [ ] **Step 1: 写幂等审计**

测试一个 fake submitter 被同一键调用两次时只执行一次；第一次 rejected 后第二次允许再次执行；不同 run 或 match 必须分别执行。

- [ ] **Step 2: 运行确认失败**

Run: `node tools/settlement_idempotency_audit.js`

Expected: FAIL because helper/state is not implemented。

- [ ] **Step 3: 实现 GameApp 幂等状态**

增加 `this.settlementRequests = { pending: {}, completed: {} }`，实现：

```js
settlementKey(mode, identity, count) { return `${mode}:${identity}:${count}`; }
shouldSubmitSettlement(key) { /* pending/completed 返回 false，其余标记 pending 返回 true */ }
markSettlementCompleted(key) { /* pending 删除，completed 保留 */ }
markSettlementFailed(key) { /* 仅删除 pending，不清理 checkpoint */ }
```

在四个正式提交入口前调用；成功后只以服务端结果更新状态并标记 completed，失败记录 telemetry 并保留原有重试提示。

- [ ] **Step 4: 在退出/切账号时清理运行期锁**

在 `goHome`、`restartMode` 和账号登出后的现有状态清理路径重置 pending 锁，但不要清除已完成键的短期记录，避免迟到响应重复应用。

- [ ] **Step 5: 运行幂等审计**

Run: `node tools/settlement_idempotency_audit.js`

Expected: `SETTLEMENT_IDEMPOTENCY_OK`。

### Task 5: 总体验证与文档收口

**Files:**
- Modify: `frontend/wechat_game/tools/stability_audit.js`
- Modify: `frontend/wechat_game/README.md`

- [ ] **Step 1: 扩展 stability audit**

增加题目失败事件、反馈生成、结算键重复调用三个检查，并保持原有 `STABILITY_OK` 输出。

- [ ] **Step 2: 运行全量 JavaScript 语法检查**

Run: `Get-ChildItem src -Recurse -Filter *.js | ForEach-Object { node --check $_.FullName }`

Expected: 所有命令退出码为 0，无语法错误。

- [ ] **Step 3: 运行现有和新增审计**

Run:

```powershell
node tools/smoke_test.js
node tools/button_audit.js
node tools/stability_audit.js
node tools/question_quality_unit_audit.js
node tools/question_contract_audit.js
node tools/settlement_idempotency_audit.js
node tools/telemetry_audit.js
node tools/layout_audit.js
node tools/question_quality_audit.js
node tools/friend_match_audit.js
node tools/matchmaking_audit.js
node tools/prelaunch_audit.js
node tools/auth_session_audit.js
node tools/run_resume_avatar_logout_audit.js
```

Expected: 所有审计输出对应 `*_OK` 或 `QUESTION_QUALITY_AUDIT OK issues=0`。

- [ ] **Step 4: 检查 diff 和受影响目录**

Run: `git diff --check` 和 `git status --short`，确认只出现 `frontend/wechat_game` 与本轮新增的 docs，不修改 backend。

- [ ] **Step 5: 记录仍需后端配合的事项**

README 明确列出：服务端仍需保证 Run 题目合同可解、题目 hash/ID 稳定、结算接口幂等和排行榜服务端校验；本轮不生成或修改后端代码。
