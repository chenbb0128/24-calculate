/*
 * Question quality audit for every game mode.
 *
 * This is a read-only development tool. It exercises the same QuestionService
 * and PuzzleGenerator used by the client, so a green report means the runtime
 * question path is also being checked.
 *
 * Run from frontend/wechat_game:
 *   node tools/question_quality_audit.js
 */
const questionCore = require('../src/core/question_service.js');
const puzzle = require('../src/core/puzzle_generator.js');
const daily = require('../src/core/daily_challenge.js');
const endless = require('../src/core/endless_mode.js');
const friend = require('../src/core/friend_match_service.js');
const levels = require('../src/core/level_catalog.js').all();
const dailyData = require('../src/core/daily_puzzle_data.js');

const YEAR = 2026;
const CAMPAIGN_PHASES = [0, 1, 2, 3];

function key(numbers) {
  return puzzle.numberKey(numbers);
}

function campaignKey(numbers) {
  return (Array.isArray(numbers) ? numbers : []).join(',');
}

function round(value, digits = 1) {
  const factor = 10 ** digits;
  return Math.round(Number(value) * factor) / factor;
}

function dateKey(year, dayOfYear) {
  const date = new Date(Date.UTC(year, 0, 1 + dayOfYear));
  return date.toISOString().slice(0, 10);
}

function dateSeed(date) {
  return Number(String(date).replace(/-/g, ''));
}

function problem(scope, message, details = {}) {
  return { scope, message, ...details };
}

function checkRule(record, rule) {
  if (!record || !rule) return false;
  const rules = {
    requiredOperator: rule.requiredOperator || rule.required_operator || '',
    forbiddenOperator: rule.forbiddenOperator || rule.forbidden_operator || '',
    allowNegativeIntermediate: false,
  };
  if (!puzzle.executeSteps(record.numbers, record.solutionSteps, rules)) return false;
  if (rule.requiresLarge && !record.numbers.some((value) => Number(value) >= 10)) return false;
  return true;
}

function hasRuleCompatibleSolution(numbers, rule) {
  const config = {
    requiredOperator: rule.requiredOperator || rule.required_operator || '',
    forbiddenOperator: rule.forbiddenOperator || rule.forbidden_operator || '',
    allowNegativeIntermediate: false,
  };
  return puzzle.solveDetailed(numbers).some((solution) => puzzle.executeSteps(numbers, solution.steps, config));
}

function auditCampaign(service) {
  const issues = [];
  const allKeys = new Set();
  const phaseStats = Object.fromEntries(CAMPAIGN_PHASES.map((phase) => [phase, {
    levels: 0,
    questions: 0,
    difficultyTotal: 0,
    min: Infinity,
    max: -Infinity,
  }]));
  const levelStats = [];
  let total = 0;
  let verified = 0;

  for (let levelIndex = 0; levelIndex < levels.length; levelIndex += 1) {
    const config = levels[levelIndex];
    const expected = Number(config.questionCount || config.question_count || 3);
    const records = service.getCampaignLevel(levelIndex, { config });
    const phase = Number(config.difficultyPhase ?? config.difficulty_phase ?? 0);
    const stats = phaseStats[phase] || (phaseStats[phase] = {
      levels: 0, questions: 0, difficultyTotal: 0, min: Infinity, max: -Infinity,
    });
    stats.levels += 1;

    if (!Array.isArray(records) || records.length !== expected) {
      issues.push(problem('campaign', '关卡题目数量不符合配置', {
        level: levelIndex + 1, expected, actual: Array.isArray(records) ? records.length : 0,
      }));
      continue;
    }

    const localKeys = new Set();
    const scores = [];
    records.forEach((record, questionIndex) => {
      total += 1;
      const numberKey = campaignKey(record.numbers);
      const usable = service.isVerified(record, config)
        && record.solutionSteps.length === 3
        && puzzle.executeSteps(record.numbers, record.solutionSteps, config);
      if (usable) verified += 1;
      else issues.push(problem('campaign', '题目未通过统一验证', { level: levelIndex + 1, question: questionIndex + 1, numberKey }));
      if (localKeys.has(numberKey)) issues.push(problem('campaign', '同一关出现重复题目', { level: levelIndex + 1, numberKey }));
      localKeys.add(numberKey);
      if (allKeys.has(numberKey)) issues.push(problem('campaign', '闯关题目全局重复', { level: levelIndex + 1, question: questionIndex + 1, numberKey }));
      allKeys.add(numberKey);
      const score = Number(record.difficultyScore || record.difficulty_score || 0);
      scores.push(score);
      stats.questions += 1;
      stats.difficultyTotal += score;
      stats.min = Math.min(stats.min, score);
      stats.max = Math.max(stats.max, score);
      if (levelIndex < 20) {
        const kinds = new Set(record.solutionSteps.map((step) => step.operator));
        if (kinds.size < 2) issues.push(problem('campaign', '前 20 关出现单一运算符解法', { level: levelIndex + 1, question: questionIndex + 1, numberKey }));
      }
    });
    levelStats.push({ level: levelIndex + 1, phase, averageDifficulty: round(scores.reduce((sum, value) => sum + value, 0) / scores.length) });
  }

  const phases = {};
  CAMPAIGN_PHASES.forEach((phase) => {
    const stats = phaseStats[phase];
    phases[phase] = {
      levels: stats.levels,
      questions: stats.questions,
      averageDifficulty: stats.questions ? round(stats.difficultyTotal / stats.questions) : 0,
      minDifficulty: stats.min === Infinity ? 0 : stats.min,
      maxDifficulty: stats.max === -Infinity ? 0 : stats.max,
    };
  });
  for (let index = 1; index < CAMPAIGN_PHASES.length; index += 1) {
    const previous = phases[CAMPAIGN_PHASES[index - 1]].averageDifficulty;
    const current = phases[CAMPAIGN_PHASES[index]].averageDifficulty;
    if (current < previous) issues.push(problem('campaign', '阶段平均难度下降', { from: CAMPAIGN_PHASES[index - 1], to: CAMPAIGN_PHASES[index], previous, current }));
  }

  return {
    total,
    unique: allKeys.size,
    duplicate: total - allKeys.size,
    verified,
    phases,
    firstLevels: levelStats.slice(0, 5),
    lastLevels: levelStats.slice(-5),
    issues,
  };
}

function auditDaily(service) {
  const issues = [];
  const allKeys = new Set();
  const ruleCoverage = {};
  let total = 0;
  let verified = 0;
  let stable = 0;
  const days = 365;
  const sampleDays = new Set([0, 1, 30, 59, 90, 120, 150, 180, 210, 240, 270, 300, 330, 364]);
  const fullDaily = process.argv.includes('--full');
  const solverDays = fullDaily
    ? new Set(Array.from({ length: days }, (_, index) => index))
    : new Set(Array.from({ length: days }, (_, index) => index).filter((index) => index % 7 === 0));
  const schedules = dailyData && Array.isArray(dailyData.days) ? dailyData.days : [];
  if (schedules.length !== days) issues.push(problem('daily', '每日静态轮换天数不是 365', { actual: schedules.length }));

  for (let day = 0; day < days; day += 1) {
    const date = dateKey(YEAR, day);
    const seed = dateSeed(date);
    const schedule = schedules[day];
    if (!schedule || !Array.isArray(schedule.puzzles) || schedule.puzzles.length !== daily.DAILY_QUESTION_COUNT) {
      issues.push(problem('daily', '每日题目数量或生成结果异常', { date }));
      continue;
    }
    const rule = daily.ruleForIndex(schedule.rule_index);
    ruleCoverage[rule.id] = (ruleCoverage[rule.id] || 0) + 1;
    const localKeys = new Set();
    schedule.puzzles.forEach((numbers, index) => {
      total += 1;
      const numberKey = key(numbers);
      const structurallyValid = Array.isArray(numbers)
        && numbers.length === 4
        && numbers.every((value) => Number.isInteger(Number(value)) && Number(value) >= 1 && Number(value) <= 13)
        && (rule.id !== 'big_digits' || numbers.some((value) => Number(value) >= 10));
      const valid = structurallyValid && (!solverDays.has(day) || hasRuleCompatibleSolution(numbers, rule));
      if (valid && solverDays.has(day)) verified += 1;
      else if (!structurallyValid || (solverDays.has(day) && !valid)) issues.push(problem('daily', '每日题目未通过规则验证', { date, question: index + 1, rule: rule.id, numberKey }));
      if (localKeys.has(numberKey)) issues.push(problem('daily', '同一天出现重复题目', { date, numberKey }));
      localKeys.add(numberKey);
      if (allKeys.has(numberKey)) issues.push(problem('daily', '年度每日题目重复', { date, question: index + 1, numberKey }));
      allKeys.add(numberKey);
      if (rule.id === 'big_digits' && !numbers.some((value) => Number(value) >= 10)) issues.push(problem('daily', '大数字规则未生效', { date, numberKey }));
    });
    if (sampleDays.has(day)) {
      const result = service.getDailyChallenge(date, seed);
      const again = service.getDailyChallenge(date, seed);
      if (result && JSON.stringify(result) === JSON.stringify(again)) stable += 1;
      else issues.push(problem('daily', '正式题目服务抽样结果不稳定或生成失败', { date }));
      if (result && result.puzzles.map((record) => key(record.numbers)).join('|') !== schedule.puzzles.map(key).join('|')) {
        issues.push(problem('daily', '正式题目服务与静态轮换不一致', { date }));
      }
    }
  }

  return {
    days,
    total,
    unique: allKeys.size,
    duplicate: total - allKeys.size,
    verified,
    structuralQuestions: total,
    solverSampleDays: solverDays.size,
    solverSampleQuestions: solverDays.size * daily.DAILY_QUESTION_COUNT,
    stableDays: stable,
    runtimeSampleDays: sampleDays.size,
    ruleCoverage,
    issues,
  };
}

function auditEndless() {
  const issues = [];
  const service = questionCore.createQuestionService({ levels });
  const runSeed = 24681357;
  const sampleCount = process.argv.includes('--deep') ? 100 : 30;
  const records = [];
  const keys = new Set();
  for (let index = 0; index < sampleCount; index += 1) {
    const record = service.getEndlessQuestion(index, runSeed);
    if (!record) {
      issues.push(problem('endless', '无尽题目生成失败', { question: index + 1, stage: Math.floor(index / 3) }));
      continue;
    }
    const numberKey = key(record.numbers);
    if (keys.has(numberKey)) issues.push(problem('endless', '同一局出现重复题目', { question: index + 1, numberKey }));
    keys.add(numberKey);
    if (!service.isVerified(record) || record.solutionSteps.length !== 3) issues.push(problem('endless', '无尽题目未通过统一验证', { question: index + 1, numberKey }));
    records.push(record);
  }

  const stageStats = {};
  records.forEach((record, index) => {
    const stage = Math.floor(index / 3);
    const entry = stageStats[stage] || { count: 0, totalDifficulty: 0, minDifficulty: Infinity };
    entry.count += 1;
    entry.totalDifficulty += Number(record.difficultyScore || 0);
    entry.minDifficulty = Math.min(entry.minDifficulty, Number(record.difficultyScore || 0));
    stageStats[stage] = entry;
  });
  const firstStages = Object.keys(stageStats).map(Number).sort((a, b) => a - b).map((stage) => ({
    stage,
    averageDifficulty: round(stageStats[stage].totalDifficulty / stageStats[stage].count),
    minDifficulty: stageStats[stage].minDifficulty,
  }));
  for (let index = 1; index < firstStages.length; index += 1) {
    const previous = firstStages[index - 1];
    const current = firstStages[index];
    // The endless director intentionally reaches a difficulty plateau once
    // the 1..13 integer space is saturated. Before the plateau, the minimum
    // must rise; after it, keep the floor and reject only a material average
    // drop (single-sample variance is normal).
    if (current.stage < 8 && current.minDifficulty < previous.minDifficulty) {
      issues.push(problem('endless', '后续阶段最低难度下降', { from: previous, to: current }));
    }
    if (current.stage >= 8 && current.minDifficulty < 12) {
      issues.push(problem('endless', '无尽高压阶段低于难度平台', { stage: current.stage, minDifficulty: current.minDifficulty }));
    }
    if (current.stage >= 8 && current.averageDifficulty + 0.75 < previous.averageDifficulty) {
      issues.push(problem('endless', '无尽高压阶段平均难度明显下降', { from: previous, to: current }));
    }
  }

  const otherService = questionCore.createQuestionService({ levels });
  const otherRunKeys = new Set();
  for (let index = 0; index < 10; index += 1) {
    const record = otherService.getEndlessQuestion(index, runSeed + 1);
    if (record) otherRunKeys.add(key(record.numbers));
  }
  const sameFirst = key(service.getEndlessQuestion(0, runSeed).numbers) === key(otherService.getEndlessQuestion(0, runSeed).numbers);
  if (!sameFirst) issues.push(problem('endless', '同一 run seed 的首题不稳定'));
  const differentRunOverlap = [...keys].filter((numberKey) => otherRunKeys.has(numberKey)).length;

  return {
    sampleCount,
    generated: records.length,
    unique: keys.size,
    duplicate: records.length - keys.size,
    differentRunSampleCount: otherRunKeys.size,
    differentRunOverlap,
    stageSamples: firstStages.slice(0, 8),
    issues,
  };
}

function auditFriend() {
  const issues = [];
  const service = questionCore.createQuestionService({ levels });
  const seed = 135792468;
  const sameA = service.getFriendQuestions(seed, { count: friend.QUESTION_COUNT });
  const sameB = questionCore.createQuestionService({ levels }).getFriendQuestions(seed, { count: friend.QUESTION_COUNT });
  const other = questionCore.createQuestionService({ levels }).getFriendQuestions(seed + 1, { count: friend.QUESTION_COUNT });
  const fingerprint = (records) => JSON.stringify((records || []).map((record) => ({ numbers: record.numbers, steps: record.solutionSteps })));
  if (!Array.isArray(sameA) || sameA.length !== friend.QUESTION_COUNT) issues.push(problem('friend', '好友模式题目数量异常', { actual: Array.isArray(sameA) ? sameA.length : 0 }));
  if (fingerprint(sameA) !== fingerprint(sameB)) issues.push(problem('friend', '相同房间种子没有生成完全相同题目'));
  if (sameA.some((record) => !service.isVerified(record))) issues.push(problem('friend', '好友模式存在未验证题目'));
  const unique = new Set(sameA.map((record) => key(record.numbers)));
  if (unique.size !== sameA.length) issues.push(problem('friend', '同一房间出现重复题目'));
  const overlapWithOtherSeed = sameA.filter((record) => other.some((candidate) => key(candidate.numbers) === key(record.numbers))).length;
  return {
    questionCount: friend.QUESTION_COUNT,
    unique: unique.size,
    differentSeedOverlap: overlapWithOtherSeed,
    sameSeedStable: fingerprint(sameA) === fingerprint(sameB),
    issues,
  };
}

function printSection(name, report) {
  const { issues = [], ...summary } = report;
  console.log(`\n[${name}]`);
  console.log(JSON.stringify(summary, null, 2));
  if (issues.length) {
    console.log(`ISSUES ${issues.length}`);
    issues.slice(0, 20).forEach((item) => console.log(`- ${JSON.stringify(item)}`));
    if (issues.length > 20) console.log(`- ... 其余 ${issues.length - 20} 条省略`);
  } else {
    console.log('ISSUES 0');
  }
}

function main() {
  const campaignService = questionCore.createQuestionService({ levels });
  const campaign = auditCampaign(campaignService);
  const dailyReport = auditDaily(questionCore.createQuestionService({ levels }));
  const endlessReport = auditEndless();
  const friendReport = auditFriend();
  [
    ['CAMPAIGN', campaign],
    ['DAILY', dailyReport],
    ['ENDLESS', endlessReport],
    ['FRIEND', friendReport],
  ].forEach(([name, report]) => printSection(name, report));
  const totalIssues = [campaign, dailyReport, endlessReport, friendReport]
    .reduce((sum, report) => sum + report.issues.length, 0);
  console.log(`\nQUESTION_QUALITY_AUDIT ${totalIssues ? 'FAILED' : 'OK'} issues=${totalIssues}`);
  if (totalIssues) process.exitCode = 1;
}

main();
