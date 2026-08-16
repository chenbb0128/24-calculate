const DAILY_QUESTION_COUNT = 3;
const RULE_COUNT = 8;
const dailyPuzzleData = require('./daily_puzzle_data.js');

const SIMPLE_CANDIDATES = [
  [1, 2, 3, 4], [1, 2, 3, 8], [1, 2, 4, 5], [1, 2, 5, 5],
  [1, 3, 5, 6], [2, 3, 4, 6], [2, 3, 6, 8], [3, 4, 6, 9],
  [4, 4, 6, 8], [1, 5, 5, 5], [2, 4, 7, 8], [3, 5, 7, 9],
];
const ADVANCED_CANDIDATES = [
  [4, 6, 10, 12], [3, 7, 11, 13], [2, 8, 10, 13], [5, 6, 11, 13],
  [1, 2, 6, 10], [1, 2, 7, 12], [1, 4, 10, 12], [2, 4, 10, 13],
  [2, 6, 10, 13], [2, 6, 11, 12], [3, 4, 6, 13],
];
// 先保留一组可读的候选；真正入题前仍由 solve + forbiddenOperator 再次验证。
// 不固定为少量数字，避免某些日期因重复数字不足而无法生成三题。
const NO_MULTIPLY_CANDIDATES = SIMPLE_CANDIDATES;

class SeededRandom {
  constructor(seed) { this.seed = (Number(seed) >>> 0) || 1; }
  next() { this.seed = (1664525 * this.seed + 1013904223) >>> 0; return this.seed / 0x100000000; }
  int(min, max) { return Math.floor(this.next() * (max - min + 1)) + min; }
}

function ruleForIndex(index) {
  const rules = [
    { id: 'no_division', title: '今日规则：禁用除法', text: '三题都不能使用 ÷，全部答对可领取完整奖励。', maxDigit: 9, forbiddenOperator: '÷' },
    { id: 'no_undo', title: '今日规则：一步到底', text: '今天不能撤销，每一步都要先想清楚。', maxDigit: 9 },
    { id: 'big_digits', title: '今日规则：进阶数字', text: '题目会出现 10～13，观察数字组合再开始。', maxDigit: 13 },
    { id: 'must_subtract', title: '今日规则：必须减法', text: '每题至少使用一次减法，才算完成挑战。', maxDigit: 9, requiredOperator: '-' },
    { id: 'must_multiply', title: '今日规则：必须乘法', text: '每题至少使用一次乘法，找出能快速合并的数字。', maxDigit: 9, requiredOperator: '×' },
    { id: 'no_multiply', title: '今日规则：禁用乘法', text: '今天不能使用 ×，尝试用加减除完成目标。', maxDigit: 9, forbiddenOperator: '×' },
    { id: 'no_add', title: '今日规则：禁用加法', text: '今天不能使用 +，先观察减法和除法的组合。', maxDigit: 9, forbiddenOperator: '+' },
    { id: 'quick_start', title: '今日规则：快速出手', text: '每题限时更短，连续答对可以获得额外分数。', maxDigit: 13, timeBonus: true },
  ];
  return { ...rules[((Number(index) % RULE_COUNT) + RULE_COUNT) % RULE_COUNT] };
}

function key(numbers) { return numbers.slice().sort((a, b) => a - b).join(','); }

function cycleDayIndex(dateKey) {
  const match = String(dateKey || '').match(/^(\d{4})-(\d{2})-(\d{2})$/);
  if (!match || !dailyPuzzleData || !Array.isArray(dailyPuzzleData.days) || !dailyPuzzleData.days.length) return -1;
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
  const start = new Date(Date.UTC(Number(match[1]), 0, 1));
  if (Number.isNaN(date.getTime())) return -1;
  const raw = Math.floor((date.getTime() - start.getTime()) / 86400000);
  const total = dailyPuzzleData.days.length;
  return ((raw % total) + total) % total;
}

function buildScheduled(generator, dateKey, dateSeed) {
  const index = cycleDayIndex(dateKey);
  const schedule = index >= 0 ? dailyPuzzleData.days[index] : null;
  if (!schedule || !Array.isArray(schedule.puzzles) || schedule.puzzles.length !== DAILY_QUESTION_COUNT) return null;
  const ruleIndex = ((Number(schedule.rule_index) % RULE_COUNT) + RULE_COUNT) % RULE_COUNT;
  const rule = ruleForIndex(ruleIndex);
  const puzzles = [];
  for (let stage = 0; stage < DAILY_QUESTION_COUNT; stage += 1) {
    const numbers = Array.isArray(schedule.puzzles[stage]) ? schedule.puzzles[stage].slice() : [];
    const record = generator.makeVerifiedRecord(
      numbers,
      3000 + stage,
      stage,
      1,
      40,
      rule.requiredOperator || '',
      rule.forbiddenOperator || '',
      false,
    );
    if (!record || !record.numbers || record.solutionSteps.length !== 3 || !generator.isVerifiedRecord(record)) return null;
    record.daily_stage = stage;
    record.daily_stage_name = ['热身题', '进阶题', '高难题'][stage];
    record.daily_rule_id = rule.id;
    puzzles.push(record);
  }
  return {
    date_key: dateKey,
    seed: Number(dateSeed),
    title: '每日挑战 · 三题连战',
    rule_id: rule.id,
    rule_title: rule.title,
    rule_text: rule.text,
    rule_index: ruleIndex,
    time_bonus: Boolean(rule.timeBonus),
    required_operator: rule.requiredOperator || '',
    forbidden_operator: rule.forbiddenOperator || '',
    max_digit: 13,
    allow_negative_intermediate: false,
    question_count: DAILY_QUESTION_COUNT,
    time_limit: rule.timeBonus ? 105 : 150,
    hint_count: rule.id === 'no_undo' ? 2 : 1,
    allow_hint: true,
    puzzles,
  };
}

function build(generator, dateKey, dateSeed) {
  const scheduled = buildScheduled(generator, dateKey, dateSeed);
  if (scheduled) return scheduled;
  const ruleIndex = ((Number(dateSeed) % RULE_COUNT) + RULE_COUNT) % RULE_COUNT;
  const rule = ruleForIndex(ruleIndex);
  const candidates = rule.id === 'no_multiply' ? NO_MULTIPLY_CANDIDATES : (rule.maxDigit > 9 ? ADVANCED_CANDIDATES : SIMPLE_CANDIDATES);
  const puzzles = [];
  const seen = new Set();
  const random = new SeededRandom(Number(dateSeed) * 97 + 1);
  for (let stage = 0; stage < DAILY_QUESTION_COUNT; stage += 1) {
    // 规则过滤后再统计解法数量；极少解题型允许只有 1 个合法解，避免某天生成失败。
    const minSolutions = 1;
    const maxSolutions = stage === 2 ? 40 : 40;
    let record = null;
    const start = (Number(dateSeed) + stage * 7) % candidates.length;
    for (let offset = 0; offset < candidates.length && !record; offset += 1) {
      const numbers = candidates[(start + offset) % candidates.length].slice();
      if (seen.has(key(numbers))) continue;
      const candidate = generator.makeVerifiedRecord(numbers, 3000 + stage, stage, minSolutions, maxSolutions, rule.requiredOperator || '', rule.forbiddenOperator || '', false);
      if (candidate && candidate.numbers && candidate.solutionSteps && candidate.solutionSteps.length === 3 && generator.isVerifiedRecord(candidate)) { record = candidate; seen.add(key(numbers)); }
    }
    let attempts = 0;
    while (!record && attempts < 1200) {
      attempts += 1;
      const numbers = [0, 0, 0, 0].map(() => random.int(1, rule.maxDigit));
      if (seen.has(key(numbers))) continue;
      const candidate = generator.makeVerifiedRecord(numbers, 3000 + stage, stage, minSolutions, maxSolutions, rule.requiredOperator || '', rule.forbiddenOperator || '', false);
      if (candidate && candidate.numbers && candidate.solutionSteps && candidate.solutionSteps.length === 3 && generator.isVerifiedRecord(candidate)) { record = candidate; seen.add(key(numbers)); }
    }
    if (!record) return {};
    record.daily_stage = stage;
    record.daily_stage_name = ['热身题', '进阶题', '高难题'][stage];
    record.daily_rule_id = rule.id;
    puzzles.push(record);
  }
  return {
    date_key: dateKey,
    seed: Number(dateSeed),
    title: '每日挑战 · 三题连战',
    rule_id: rule.id,
    rule_title: rule.title,
    rule_text: rule.text,
    rule_index: ruleIndex,
    time_bonus: Boolean(rule.timeBonus),
    required_operator: rule.requiredOperator || '',
    forbidden_operator: rule.forbiddenOperator || '',
    max_digit: Number(rule.maxDigit),
    allow_negative_intermediate: false,
    question_count: DAILY_QUESTION_COUNT,
    time_limit: rule.timeBonus ? 105 : 150,
    hint_count: rule.id === 'no_undo' ? 2 : 1,
    allow_hint: true,
    puzzles,
  };
}

module.exports = { DAILY_QUESTION_COUNT, RULE_COUNT, rulePreview: (seed) => ruleForIndex(Number(seed) % RULE_COUNT), build, ruleForIndex };
