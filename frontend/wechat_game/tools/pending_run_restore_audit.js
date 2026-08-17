/* Verify startup-style Run recovery and terminal checkpoint cleanup. */
const assert = require('assert');

const memory = Object.create(null);
global.wx = {
  getStorageSync(key) { return memory[key]; },
  setStorageSync(key, value) { memory[key] = value; },
  removeStorageSync(key) { delete memory[key]; },
  request(options) {
    const path = String(options.url || '').replace(/^https:\/\/calc-api\.pdurl\.cn/, '');
    const success = (statusCode, data) => options.success({ statusCode, data: JSON.stringify(data) });
    if (path.endsWith('/campaign/runs/campaign-active')) {
      success(200, {
        code: 0,
        message: 'success',
        data: {
          run_id: 'campaign-active', status: 'running', level_id: 2, question_index: 1, score: 35, mistakes: 1, hints_used: 0, best_combo: 2,
          puzzles: [{ puzzle_id: 'c1', numbers: [1, 2, 4, 5] }, { puzzle_id: 'c2', numbers: [3, 3, 4, 6] }],
          attempts: [{ puzzle_id: 'c1', question_index: 0, solved: true, score: 35 }],
        },
      });
      return;
    }
    if (path.endsWith('/daily/runs/daily-finished')) {
      success(200, { code: 0, message: 'success', data: { run_id: 'daily-finished', status: 'finished', completed: true } });
      return;
    }
    if (path.endsWith('/endless/runs/endless-expired')) {
      success(404, { code: 40404, message: 'run expired', data: null });
      return;
    }
    options.fail({ errMsg: `unexpected request ${path}` });
  },
};
memory.twenty_four_auth = { access_token: 'access', refresh_token: 'refresh', token_type: 'Bearer' };

const storage = require('../src/services/storage.js');
const { GameApp } = require('../src/app/game_app.js');

function check(condition, message) { assert.ok(condition, message); }

async function run() {
  storage.setAccount('restore-account');
  const progress = storage.load();
  storage.savePendingRun({ mode: 'campaign', run_id: 'campaign-active', level_id: 2, question_index: 0, attempts: [] }, progress);
  storage.savePendingRun({ mode: 'daily', run_id: 'daily-finished', date_key: '2026-08-18', attempts: [] });
  storage.savePendingRun({ mode: 'endless', run_id: 'endless-expired', question_index: 3, attempts: [{ question_index: 0 }] });

  const app = Object.create(GameApp.prototype);
  app.backendAuth = { status: 'ready' };
  app.friendRoomFromInvite = false;
  app.mode = 'campaign';
  app.screen = 'home';
  app.progress = storage.load();
  app.feedback = {};
  app.messages = [];
  app.triggerFeedback = (type, text) => app.messages.push({ type, text });
  app.applyResumedRun = (record) => { app.resumed = record; return true; };

  await app.restorePendingRuns();
  check(app.resumed && app.resumed.pending.mode === 'campaign', '启动恢复没有选择 active campaign Run');
  check(!storage.getPendingRun('daily'), 'finished Run 没有清理 pending');
  check(!storage.getPendingRun('endless'), 'expired Run 没有清理 pending');
  check(storage.getPendingRun('campaign').run_id === 'campaign-active', 'active Run 被错误清理');
  console.log('PENDING_RUN_RESTORE_OK');
}

run().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
