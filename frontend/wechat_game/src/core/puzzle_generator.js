const OPERATORS = ['+', '-', '×', '÷'];
const MAX_SOLUTIONS = 40;

class SeededRandom {
  constructor(seed) {
    this.seed = (Number(seed) >>> 0) || 1;
  }

  next() {
    this.seed = (1664525 * this.seed + 1013904223) >>> 0;
    return this.seed / 0x100000000;
  }

  int(min, max) {
    return Math.floor(this.next() * (max - min + 1)) + min;
  }
}

function numberKey(numbers) {
  return numbers.slice().sort((a, b) => a - b).join(',');
}

function difficultyScore(numbers, solutions = [], detailed = null) {
  const values = numbers.map((value) => Number(value));
  const maxDigit = Math.max(...values);
  const uniqueCount = new Set(values).size;
  const solutionCount = Math.max(1, solutions.length);
  const steps = detailed && Array.isArray(detailed.steps) ? detailed.steps : [];
  const operators = new Set(steps.map((step) => step.operator));
  const hasLargeDigit = maxDigit >= 10;
  const hasDivision = steps.some((step) => step.operator === '÷');
  const hasSubtract = steps.some((step) => step.operator === '-');
  const duplicatePenalty = uniqueCount < 4 ? -1 : 0;
  let score = 1;
  score += hasLargeDigit ? 2 : 0;
  score += Math.max(0, 6 - solutionCount) * 1.2;
  score += Math.max(0, operators.size - 1) * 1.5;
  score += hasDivision ? 1.2 : 0;
  score += hasSubtract ? 0.8 : 0;
  score += steps.filter((step) => step.operator === '÷' || step.operator === '-').length * 0.45;
  score += duplicatePenalty;
  return Math.max(1, Math.min(20, Math.round(score)));
}

function difficultyTier(score) {
  const value = Number(score) || 1;
  if (value <= 5) return '简单';
  if (value <= 9) return '进阶';
  if (value <= 14) return '困难';
  return '大师';
}

function addCandidate(result, seen, value, expression) {
  if (!Number.isInteger(value) || Math.abs(value) > 10000) return;
  if (result.length >= MAX_SOLUTIONS) return;
  if (!seen.has(expression)) {
    seen.add(expression);
    result.push(expression);
  }
}

function solveDetailed(numbers) {
  const solutions = [];
  const seenExpressions = new Set();

  function search(items) {
    if (solutions.length >= MAX_SOLUTIONS) return;
    if (items.length === 1) {
      if (items[0].value === 24 && !seenExpressions.has(items[0].expression)) {
        seenExpressions.add(items[0].expression);
        solutions.push({ expression: items[0].expression, steps: items[0].steps });
      }
      return;
    }

    for (let i = 0; i < items.length; i += 1) {
      for (let j = i + 1; j < items.length; j += 1) {
        const left = items[i];
        const right = items[j];
        const rest = items.filter((_, index) => index !== i && index !== j);
        const candidates = [
          [left.value + right.value, '+', left, right],
          [left.value * right.value, '×', left, right],
          [left.value - right.value, '-', left, right],
          [right.value - left.value, '-', right, left],
        ];
        if (right.value !== 0 && left.value % right.value === 0) {
          candidates.push([left.value / right.value, '÷', left, right]);
        }
        if (left.value !== 0 && right.value % left.value === 0) {
          candidates.push([right.value / left.value, '÷', right, left]);
        }
        for (const [value, operator, a, b] of candidates) {
          const step = {
            firstIndices: a.indices.slice(), secondIndices: b.indices.slice(),
            first: a.value, second: b.value, operator,
          };
          const next = rest.concat([{
            value,
            expression: `(${a.expression} ${operator} ${b.expression})`,
            indices: a.indices.concat(b.indices),
            steps: a.steps.concat(b.steps, [step]),
          }]);
          search(next);
        }
      }
    }
  }

  search(numbers.map((value, index) => ({
    value: Number(value), expression: String(value), indices: [index], steps: [],
  })));
  return solutions;
}

function solve(numbers) {
  return solveDetailed(numbers).map((item) => item.expression);
}

function firstStep(expression, steps = []) {
  if (Array.isArray(steps) && steps.length) {
    const step = steps[0];
    return {
      first: Number(step.first), operator: step.operator, second: Number(step.second),
      firstIndex: Number(step.firstIndices[0]), secondIndex: Number(step.secondIndices[0]),
      firstIndices: step.firstIndices.slice(), secondIndices: step.secondIndices.slice(),
    };
  }
  const text = String(expression || '');
  const match = text.match(/\((\d+)\s([+\-×÷])\s(\d+)\)/);
  if (!match) return null;
  return { first: Number(match[1]), operator: match[2], second: Number(match[3]) };
}

function applyStep(valueA, valueB, operator) {
  if (operator === '+') return valueA + valueB;
  if (operator === '-') return valueA - valueB;
  if (operator === '×') return valueA * valueB;
  if (operator === '÷' && valueB !== 0 && valueA % valueB === 0) return valueA / valueB;
  return null;
}

function executeSteps(numbers, steps, config = {}) {
  if (!Array.isArray(numbers) || numbers.length !== 4 || !Array.isArray(steps) || steps.length !== 3) return null;
  const items = numbers.map((value, index) => ({ value: Number(value), indices: [index] }));
  const required = config.requiredOperator || config.required_operator || '';
  const forbidden = config.forbiddenOperator || config.forbidden_operator || '';
  const usedOperators = [];
  const sameGroup = (a, b) => a.length === b.length && a.every((value) => b.includes(value));
  for (const step of steps) {
    const firstIndices = Array.isArray(step.firstIndices) ? step.firstIndices : [step.firstIndex];
    const secondIndices = Array.isArray(step.secondIndices) ? step.secondIndices : [step.secondIndex];
    const firstAt = items.findIndex((item) => sameGroup(item.indices, firstIndices));
    const secondAt = items.findIndex((item) => sameGroup(item.indices, secondIndices));
    if (firstAt < 0 || secondAt < 0 || firstAt === secondAt) return null;
    const operator = step.operator;
    if (forbidden && operator === forbidden) return null;
    const value = applyStep(items[firstAt].value, items[secondAt].value, operator);
    if (!Number.isInteger(value)) return null;
    if (config.allowNegativeIntermediate === false || config.allow_negative_intermediate === false) {
      if (value < 0) return null;
    }
    usedOperators.push(operator);
    const merged = { value, indices: items[firstAt].indices.concat(items[secondAt].indices) };
    const next = items.filter((_, index) => index !== firstAt && index !== secondAt);
    next.push(merged);
    items.splice(0, items.length, ...next);
  }
  if (required && !usedOperators.includes(required)) return null;
  if (items.length !== 1 || items[0].value !== 24 || items[0].indices.length !== 4) return null;
  return { value: items[0].value, operators: usedOperators };
}

function isVerifiedRecord(record) {
  return Boolean(record && executeSteps(record.numbers, record.solutionSteps, record.rules || {}));
}

function matchesRules(solutions, config) {
  return compatibleSolutions(solutions, config).length > 0;
}

function ruleCompatible(solution, config) {
  const required = config.requiredOperator || config.required_operator || '';
  const forbidden = config.forbiddenOperator || config.forbidden_operator || '';
  if (required && !String(solution).includes(required)) return false;
  if (forbidden && String(solution).includes(forbidden)) return false;
  return true;
}

function compatibleSolutions(solutions, config) {
  return (solutions || []).filter((solution) => ruleCompatible(solution, config));
}

function candidatePool(config) {
  const simple = [
    [1, 1, 1, 8], [1, 1, 2, 6], [1, 1, 2, 7], [1, 1, 2, 9],
    [1, 1, 3, 4], [1, 1, 3, 5], [1, 1, 4, 4], [1, 1, 4, 9],
    [1, 1, 5, 7], [1, 1, 5, 8], [1, 2, 2, 4], [1, 2, 2, 5],
    [1, 2, 2, 7], [1, 2, 2, 8], [1, 2, 2, 9], [1, 2, 3, 3],
    [1, 2, 4, 5], [1, 2, 5, 5], [1, 3, 3, 3], [1, 3, 5, 6],
    [2, 2, 3, 8], [2, 3, 4, 6], [2, 3, 4, 9], [2, 3, 6, 8],
    [3, 3, 4, 6], [3, 4, 6, 9], [4, 4, 6, 8], [5, 5, 5, 5],
  ];
  const advanced = [
    [1, 1, 1, 11], [1, 1, 1, 13], [1, 1, 3, 13], [1, 1, 4, 12],
    [1, 2, 3, 13], [1, 2, 4, 11], [1, 2, 4, 13], [1, 2, 5, 13],
    [1, 2, 6, 10], [1, 2, 6, 11], [1, 2, 6, 13], [1, 2, 7, 11],
    [1, 2, 7, 12], [1, 3, 5, 11], [1, 3, 6, 11], [1, 3, 7, 12],
    [1, 3, 8, 10], [1, 3, 8, 11], [1, 3, 12, 13], [1, 4, 5, 13],
    [1, 4, 10, 12], [1, 6, 10, 12], [1, 6, 10, 13], [1, 6, 11, 12],
    [1, 6, 12, 13], [2, 3, 8, 13], [2, 3, 9, 13], [2, 4, 10, 13],
    [2, 6, 10, 13], [2, 6, 11, 12], [2, 8, 10, 13], [3, 4, 6, 13],
  ];
  return Number(config.maxDigit || config.max_digit || 9) > 9 ? advanced : simple;
}

function makeRecord(numbers, solutions, levelIndex, questionIndex, seed, config) {
  const detailed = solveDetailed(numbers);
  const compatible = detailed.filter((item) => ruleCompatible(item.expression, config) && executeSteps(numbers, item.steps, config));
  const validDetailed = compatible[0];
  const validSolution = validDetailed ? validDetailed.expression : (solutions.find((solution) => ruleCompatible(solution, config)) || solutions[0]);
  return makeRecordFromDetailed(numbers, compatible, validSolution, levelIndex, questionIndex, seed, config);
}

function makeRecordFromDetailed(numbers, detailed, validSolution, levelIndex, questionIndex, seed, config) {
  const compatible = Array.isArray(detailed) ? detailed : [];
  const validDetailed = compatible[0];
  if (!validDetailed || !validSolution) return null;
  const requiredOperator = config.requiredOperator || config.required_operator || '';
  const forbiddenOperator = config.forbiddenOperator || config.forbidden_operator || '';
  const allowNegativeIntermediate = config.allowNegativeIntermediate !== false && config.allow_negative_intermediate !== false;
  const record = {
    version: 1,
    puzzleId: `L${String(levelIndex + 1).padStart(3, '0')}-Q${questionIndex + 1}`,
    seed,
    numbers: numbers.slice(),
    target: 24,
    solution: validSolution,
    solutionCount: compatible.length,
    solutionSteps: validDetailed ? validDetailed.steps : [],
    firstStep: firstStep(validSolution, validDetailed && validDetailed.steps),
    rules: {
      useEachNumberOnce: true,
      integerIntermediateResults: true,
      allowedOperators: OPERATORS.slice(),
      requiredOperator,
      forbiddenOperator,
      allowNegativeIntermediate,
    },
  };
  record.difficultyScore = difficultyScore(numbers, compatible.map((item) => item.expression), validDetailed);
  record.difficultyTier = difficultyTier(record.difficultyScore);
  record.difficulty_score = record.difficultyScore;
  record.difficulty_tier = record.difficultyTier;
  return record;
}

function isRecordUsable(record) {
  return Boolean(record && record.numbers && record.solutionSteps && executeSteps(record.numbers, record.solutionSteps, record.rules));
}

function generatePuzzleSet(config, levelIndex, count, seed, globalUsedKeys = null) {
  const random = new SeededRandom(seed + levelIndex * 7919);
  const result = [];
  const used = globalUsedKeys instanceof Set ? globalUsedKeys : new Set();
  let candidates = candidatePool(config);
  // Friend rounds opt into a seeded shuffle so a new round does not repeatedly
  // take the same prefix of the deterministic candidate pool. Other modes keep
  // the original order and therefore retain their fixed-question guarantees.
  if (config && (config.shuffleCandidates || config.shuffle_candidates)) {
    candidates = candidates.map((numbers) => numbers.slice());
    for (let index = candidates.length - 1; index > 0; index -= 1) {
      const swapIndex = random.int(0, index);
      const value = candidates[index];
      candidates[index] = candidates[swapIndex];
      candidates[swapIndex] = value;
    }
  }
  const start = config && (config.shuffleCandidates || config.shuffle_candidates)
    ? random.int(0, Math.max(0, candidates.length - 1))
    : (levelIndex * 7) % candidates.length;

  function tryNumbers(numbers) {
    const key = numberKey(numbers);
    if (used.has(key)) return false;
    const solutions = compatibleSolutions(solve(numbers), config);
    const min = Number(config.minSolutions || config.min_solutions || 1);
    const max = Number(config.maxSolutions || config.max_solutions || 999999);
    if (solutions.length < min || solutions.length > max) return false;
    const record = makeRecord(numbers, solutions, levelIndex, result.length, seed, config);
    const minDifficulty = Number(config.minDifficulty || config.min_difficulty || 0);
    const maxDifficulty = Number(config.maxDifficulty || config.max_difficulty || 999999);
    if (record.difficultyScore < minDifficulty || record.difficultyScore > maxDifficulty) return false;
    if (!isRecordUsable(record)) return false;
    used.add(key);
    result.push(record);
    return true;
  }

  for (let offset = 0; offset < candidates.length && result.length < count; offset += 1) {
    tryNumbers(candidates[(start + offset) % candidates.length].slice());
  }

  let attempts = 0;
  while (result.length < count && attempts < 800) {
    attempts += 1;
    const numbers = [];
    const minDigit = Number(config.minDigit || config.min_digit || 1);
    const maxDigit = Number(config.maxDigit || config.max_digit || 9);
    for (let i = 0; i < 4; i += 1) numbers.push(random.int(minDigit, maxDigit));
    tryNumbers(numbers);
  }
  return result;
}

const CAMPAIGN_DIGIT_MAX = 9;
const CAMPAIGN_COMBINATION_TOTAL = 495;

function campaignNumberKey(numbers) {
  return (Array.isArray(numbers) ? numbers : []).join(',');
}

function campaignPermutations(values) {
  const result = [];
  function visit(items, path) {
    if (!items.length) {
      result.push(path.slice());
      return;
    }
    const used = new Set();
    items.forEach((value, index) => {
      if (used.has(value)) return;
      used.add(value);
      visit(items.slice(0, index).concat(items.slice(index + 1)), path.concat(value));
    });
  }
  visit((Array.isArray(values) ? values : []).slice(), []);
  return result;
}

function makeCampaignCandidate(numbers) {
  const detailed = solveDetailed(numbers);
  if (!detailed.length) return null;
  const key = campaignNumberKey(numbers);
  const expressions = detailed.map((item) => item.expression);
  const score = difficultyScore(numbers, expressions, detailed[0]);
  return { numbers: numbers.slice(), key, detailed, score };
}

function generateCampaignPuzzleBank(levels, seedBase = 240000) {
  return buildCampaignPuzzleBankFromCandidates(levels, seedBase, buildCampaignCandidates());
}

function expressionFromSteps(numbers, steps) {
  if (!Array.isArray(numbers) || numbers.length !== 4 || !Array.isArray(steps) || steps.length !== 3) return '';
  const items = numbers.map((value, index) => ({ value: Number(value), expression: String(value), indices: [index] }));
  const sameGroup = (a, b) => a.length === b.length && a.every((value) => b.includes(value));
  for (const step of steps) {
    const firstAt = items.findIndex((item) => sameGroup(item.indices, step.firstIndices));
    const secondAt = items.findIndex((item) => sameGroup(item.indices, step.secondIndices));
    if (firstAt < 0 || secondAt < 0 || firstAt === secondAt) return '';
    const value = applyStep(items[firstAt].value, items[secondAt].value, step.operator);
    if (!Number.isInteger(value)) return '';
    const first = items[firstAt];
    const second = items[secondAt];
    const merged = {
      value,
      expression: `(${first.expression} ${step.operator} ${second.expression})`,
      indices: first.indices.concat(second.indices),
    };
    const next = items.filter((_, index) => index !== firstAt && index !== secondAt);
    next.push(merged);
    items.splice(0, items.length, ...next);
  }
  return items.length === 1 && items[0].value === 24 ? items[0].expression : '';
}

function materializeCampaignRecord(entry, levelIndex, questionIndex, seed, config) {
  if (!Array.isArray(entry) || entry.length < 4 || !Array.isArray(entry[0]) || !Array.isArray(entry[1])) return null;
  const numbers = entry[0].map((value) => Number(value));
  if (numbers.length !== 4 || numbers.some((value) => !Number.isInteger(value))) return null;
  const solutionSteps = entry[1].map((step) => {
    if (!Array.isArray(step) || step.length < 5) return null;
    return {
      firstIndices: Array.isArray(step[0]) ? step[0].map(Number) : [],
      secondIndices: Array.isArray(step[1]) ? step[1].map(Number) : [],
      first: Number(step[2]),
      second: Number(step[3]),
      operator: String(step[4]),
    };
  });
  if (solutionSteps.some((step) => !step) || solutionSteps.length !== 3) return null;
  const solution = expressionFromSteps(numbers, solutionSteps);
  if (!solution) return null;
  const allowNegativeIntermediate = config.allowNegativeIntermediate !== false && config.allow_negative_intermediate !== false;
  const record = {
    version: 1,
    puzzleId: `L${String(levelIndex + 1).padStart(3, '0')}-Q${questionIndex + 1}`,
    seed,
    numbers,
    target: 24,
    solution,
    solutionCount: Math.max(1, Number(entry[2]) || 1),
    solutionSteps,
    firstStep: firstStep(solution, solutionSteps),
    rules: {
      useEachNumberOnce: true,
      integerIntermediateResults: true,
      allowedOperators: OPERATORS.slice(),
      requiredOperator: config.requiredOperator || config.required_operator || '',
      forbiddenOperator: config.forbiddenOperator || config.forbidden_operator || '',
      allowNegativeIntermediate,
    },
    difficultyScore: Math.max(1, Math.min(20, Number(entry[3]) || 1)),
  };
  record.difficultyTier = difficultyTier(record.difficultyScore);
  record.difficulty_score = record.difficultyScore;
  record.difficulty_tier = record.difficultyTier;
  return isRecordUsable(record) ? record : null;
}

function loadCampaignPuzzleBankFromData(data, levels, seedBase = 240000) {
  const source = Array.isArray(levels) ? levels : [];
  const entries = data && Array.isArray(data.levels) ? data.levels : null;
  if (!entries || entries.length !== source.length) return null;
  const bank = [];
  const keys = new Set();
  let total = 0;
  let verifiedCount = 0;
  let difficultyTotal = 0;
  const phaseStats = {};
  for (let levelIndex = 0; levelIndex < source.length; levelIndex += 1) {
    const config = source[levelIndex];
    const levelEntries = entries[levelIndex];
    const expectedCount = Number(config.questionCount || config.question_count || 3);
    if (!Array.isArray(levelEntries) || levelEntries.length !== expectedCount) return null;
    const records = levelEntries.map((entry, questionIndex) => materializeCampaignRecord(
      entry,
      Number(config.index || levelIndex),
      questionIndex,
      Number(seedBase) + Number(config.index || levelIndex) * 9973 + questionIndex,
      config,
    ));
    if (records.some((record) => !record)) return null;
    let addedThisLevel = 0;
    records.forEach((record) => {
      const key = campaignNumberKey(record.numbers);
      if (keys.has(key)) return;
      keys.add(key);
      addedThisLevel += 1;
      total += 1;
      verifiedCount += 1;
      difficultyTotal += Number(record.difficultyScore || 0);
      const phase = Number(config.difficultyPhase ?? config.difficulty_phase ?? 0);
      const entryStats = phaseStats[phase] || { total: 0, count: 0 };
      entryStats.total += Number(record.difficultyScore || 0);
      entryStats.count += 1;
      phaseStats[phase] = entryStats;
    });
    if (addedThisLevel !== records.length) return null;
    bank.push(records);
  }
  const expected = source.reduce((sum, config) => sum + Number(config.questionCount || config.question_count || 3), 0);
  if (total !== expected || verifiedCount !== expected || keys.size !== expected) return null;
  Object.keys(phaseStats).forEach((phase) => {
    phaseStats[phase].average = phaseStats[phase].count
      ? Math.round((phaseStats[phase].total / phaseStats[phase].count) * 10) / 10
      : 0;
  });
  return {
    bank,
    total,
    expected,
    duplicateCount: 0,
    missingCount: 0,
    verifiedCount,
    averageDifficulty: total ? Math.round((difficultyTotal / total) * 10) / 10 : 0,
    uniqueKeys: keys.size,
    candidateCount: 0,
    phaseStats,
    poolDiagnostics: { source: 'static' },
  };
}

function createCampaignPuzzleBankBuilder(levels, seedBase = 240000) {
  const source = Array.isArray(levels) ? levels : [];
  const state = {
    stage: 'candidates',
    first: 1,
    second: 1,
    third: 1,
    fourth: 1,
    processed: 0,
    candidates: [],
    result: null,
  };

  function advanceCombination() {
    state.fourth += 1;
    if (state.fourth <= CAMPAIGN_DIGIT_MAX) return;
    state.third += 1;
    if (state.third <= CAMPAIGN_DIGIT_MAX) {
      state.fourth = state.third;
      return;
    }
    state.second += 1;
    if (state.second <= CAMPAIGN_DIGIT_MAX) {
      state.third = state.second;
      state.fourth = state.third;
      return;
    }
    state.first += 1;
    state.second = state.first;
    state.third = state.second;
    state.fourth = state.third;
  }

  function progressInfo() {
    if (state.stage === 'done') return { progress: 1, stage: state.stage, processed: state.processed, total: CAMPAIGN_COMBINATION_TOTAL };
    if (state.stage === 'finalize') return { progress: 0.94, stage: state.stage, processed: state.processed, total: CAMPAIGN_COMBINATION_TOTAL };
    return {
      progress: Math.min(0.92, (state.processed / CAMPAIGN_COMBINATION_TOTAL) * 0.92),
      stage: state.stage,
      processed: state.processed,
      total: CAMPAIGN_COMBINATION_TOTAL,
    };
  }

  return {
    step(budgetMs = 4) {
      if (state.stage === 'done') return { done: true, result: state.result, ...progressInfo() };
      if (state.stage === 'candidates') {
        const startedAt = Date.now();
        const safeBudget = Math.max(1, Math.min(8, Number(budgetMs) || 4));
        do {
          if (state.first > CAMPAIGN_DIGIT_MAX) break;
          const numbers = [state.first, state.second, state.third, state.fourth];
          const candidate = makeCampaignCandidate(numbers);
          if (candidate) state.candidates.push(candidate);
          state.processed += 1;
          advanceCombination();
        } while (state.first <= CAMPAIGN_DIGIT_MAX && Date.now() - startedAt < safeBudget);
        if (state.first > CAMPAIGN_DIGIT_MAX) state.stage = 'finalize';
        return { done: false, result: null, ...progressInfo() };
      }
      if (state.stage === 'finalize') {
        state.result = buildCampaignPuzzleBankFromCandidates(source, seedBase, state.candidates);
        state.stage = 'done';
        return { done: true, result: state.result, ...progressInfo() };
      }
      return { done: false, result: null, ...progressInfo() };
    },
    getProgress() { return progressInfo(); },
  };
}

function buildCampaignPuzzleBankFromCandidates(levels, seedBase, candidates) {
  const source = Array.isArray(levels) ? levels : [];
  const expected = source.reduce((sum, config) => sum + Number(config.questionCount || config.question_count || 3), 0);
  const usedKeys = new Set();
  const bank = [];
  const random = new SeededRandom(seedBase);
  const allCandidates = Array.isArray(candidates) ? candidates : [];
  const assignedKeys = new Set();
  const phaseRequirements = new Map();
  source.forEach((config) => {
    const phase = Number(config.difficultyPhase ?? config.difficulty_phase ?? 0);
    const count = Number(config.questionCount || config.question_count || 3);
    phaseRequirements.set(phase, (phaseRequirements.get(phase) || 0) + count);
  });

  // 先锁定困难阶段，再锁定大师阶段，最后分配宽松阶段。
  // 困难阶段与大师阶段有部分候选重叠，按这个顺序可以保留足够的高难题。
  const preferredPhaseOrder = [2, 3, 1, 0];
  const phaseOrder = preferredPhaseOrder.filter((phase) => phaseRequirements.has(phase));
  Array.from(phaseRequirements.keys()).forEach((phase) => {
    if (!phaseOrder.includes(phase)) phaseOrder.push(phase);
  });
  const phasePools = new Map();
  const poolDiagnostics = {};
  for (const phase of phaseOrder) {
    const needed = phaseRequirements.get(phase) || 0;
    const candidates = allCandidates.filter((candidate) => {
      if (assignedKeys.has(candidate.key)) return false;
      return candidateFitsPhase(candidate, phase);
    });
    const selected = selectCampaignCandidates(candidates, needed, phase, random);
    selected.forEach((candidate) => assignedKeys.add(candidate.key));
    phasePools.set(phase, selected);
    poolDiagnostics[phase] = {
      available: candidates.length,
      requested: needed,
      selected: selected.length,
    };
  }

  // 只允许在“难度边界”上做小幅回退，数字范围、可解性和全局去重始终是硬规则。
  for (const phase of phaseOrder) {
    const needed = phaseRequirements.get(phase) || 0;
    const selected = phasePools.get(phase) || [];
    if (selected.length >= needed) continue;
    const relaxed = allCandidates.filter((candidate) => {
      if (assignedKeys.has(candidate.key)) return false;
      return candidateFitsPhase(candidate, phase, true);
    });
    const extra = selectCampaignCandidates(relaxed, needed - selected.length, phase, random);
    extra.forEach((candidate) => {
      assignedKeys.add(candidate.key);
      selected.push(candidate);
    });
    poolDiagnostics[phase].fallbackSelected = extra.length;
  }

  const phaseOffsets = new Map();
  source.forEach((config) => {
    const index = Number(config.index || 0);
    const phase = Number(config.difficultyPhase ?? config.difficulty_phase ?? 0);
    const count = Number(config.questionCount || config.question_count || 3);
    const pool = phasePools.get(phase) || [];
    const offset = phaseOffsets.get(phase) || 0;
    const records = [];
    for (let questionIndex = 0; questionIndex < count; questionIndex += 1) {
      const candidate = pool[offset + questionIndex];
      if (!candidate) break;
      const record = makeRecordFromDetailed(
        candidate.numbers,
        candidate.detailed,
        candidate.detailed[0].expression,
        index,
        questionIndex,
        Number(seedBase) + index * 9973 + questionIndex,
        config,
      );
      if (record && isRecordUsable(record)) {
        records.push(record);
        usedKeys.add(candidate.key);
      }
    }
    phaseOffsets.set(phase, offset + records.length);
    bank.push(records);
  });

  const total = bank.reduce((sum, records) => sum + records.length, 0);
  const verifiedCount = bank.reduce((sum, records) => sum + records.filter((record) => isVerifiedRecord(record)).length, 0);
  const difficultyTotal = bank.reduce((sum, records) => sum + records.reduce((inner, record) => inner + Number(record.difficultyScore || 0), 0), 0);
  const missingCount = Math.max(0, expected - total);
  const duplicateCount = total - usedKeys.size;
  const phaseStats = {};
  bank.forEach((records, index) => {
    const phase = Number(source[index].difficultyPhase ?? source[index].difficulty_phase ?? 0);
    const values = records.map((record) => Number(record.difficultyScore || 0));
    const entry = phaseStats[phase] || { total: 0, count: 0 };
    entry.total += values.reduce((sum, value) => sum + value, 0);
    entry.count += values.length;
    phaseStats[phase] = entry;
  });
  Object.keys(phaseStats).forEach((phase) => {
    phaseStats[phase].average = phaseStats[phase].count
      ? Math.round((phaseStats[phase].total / phaseStats[phase].count) * 10) / 10
      : 0;
  });
  return {
    bank,
    total,
    expected,
    duplicateCount,
    missingCount,
    verifiedCount,
    averageDifficulty: total ? Math.round((difficultyTotal / total) * 10) / 10 : 0,
    uniqueKeys: usedKeys.size,
    candidateCount: allCandidates.length,
    phaseStats,
    poolDiagnostics,
  };
}

function buildCampaignCandidates() {
  const candidates = [];
  for (let first = 1; first <= CAMPAIGN_DIGIT_MAX; first += 1) {
    for (let second = first; second <= CAMPAIGN_DIGIT_MAX; second += 1) {
      for (let third = second; third <= CAMPAIGN_DIGIT_MAX; third += 1) {
        for (let fourth = third; fourth <= CAMPAIGN_DIGIT_MAX; fourth += 1) {
          campaignPermutations([first, second, third, fourth]).forEach((numbers) => {
            const candidate = makeCampaignCandidate(numbers);
            if (candidate) candidates.push(candidate);
          });
        }
      }
    }
  }
  return candidates;
}

function candidateFitsPhase(candidate, phase, relaxed = false) {
  const values = candidate.numbers;
  const maxDigit = Math.max(...values);
  const minDigit = Math.min(...values);
  const solutionCount = candidate.detailed.length;
  if (phase === 0) {
    return maxDigit <= 9 && solutionCount >= 2 && candidate.score >= 1 && candidate.score <= (relaxed ? 9 : 7);
  }
  if (phase === 1) {
    return solutionCount >= 1 && solutionCount <= 24 && candidate.score >= (relaxed ? 3 : 4) && candidate.score <= (relaxed ? 14 : 12);
  }
  if (phase === 2) {
    return minDigit >= 2 && solutionCount >= 1 && solutionCount <= (relaxed ? 18 : 12)
      && candidate.score >= (relaxed ? 6 : 7) && candidate.score <= (relaxed ? 18 : 16);
  }
  // 大师阶段允许出现 1，但要求解法很少且难度分较高；真正的高数字题会
  // 由 maxDigit 和 score 共同提高难度，避免为了凑数量把后期题目放宽成简单题。
  return solutionCount >= 1 && solutionCount <= (relaxed ? 6 : 4)
    && candidate.score >= (relaxed ? 8 : 9) && candidate.score <= 20;
}

function selectCampaignCandidates(candidates, count, phase, random) {
  if (count <= 0 || !candidates.length) return [];
  const shuffled = candidates.slice();
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swapIndex = random.int(0, index);
    const temp = shuffled[index];
    shuffled[index] = shuffled[swapIndex];
    shuffled[swapIndex] = temp;
  }
  // 阶段内部仍按难度排序，保证玩家能感受到逐步变难；同分题目保持随机，避免题型排列机械。
  shuffled.sort((left, right) => {
    // 简单、进阶、困难阶段从低到高铺开；大师阶段保留最高难度题。
    // 这样困难阶段不会提前消耗掉大师阶段需要的少解法题目。
    const ascending = phase === 0 || phase === 1 || phase === 2;
    const scoreDifference = ascending ? left.score - right.score : right.score - left.score;
    return scoreDifference || (left.key < right.key ? -1 : 1);
  });
  return shuffled.slice(0, count);
}

function makeVerifiedRecord(numbers, levelIndex, questionIndex, minSolutions = 1, maxSolutions = MAX_SOLUTIONS, requiredOperator = '', forbiddenOperator = '', allowNegativeIntermediate = true) {
  const values = numbers.map((value) => Number(value));
  const ruleConfig = { requiredOperator, forbiddenOperator, allowNegativeIntermediate };
  const solutions = solveDetailed(values)
    .filter((item) => ruleCompatible(item.expression, ruleConfig) && executeSteps(values, item.steps, ruleConfig))
    .map((item) => item.expression);
  if (solutions.length < Number(minSolutions || 1) || solutions.length > Number(maxSolutions || MAX_SOLUTIONS)) return {};
  const record = makeRecord(values, solutions, levelIndex, questionIndex, 0, ruleConfig);
  return isRecordUsable(record) ? record : {};
}

function endlessConfig(questionIndex) {
  const stage = Math.floor(questionIndex / 3);
  return {
    stage,
    timeLimit: Math.max(48, 88 - stage * 1.55),
    minDigit: stage < 8 ? 1 : 2,
    maxDigit: stage < 4 ? 9 : 13,
    minSolutions: stage < 2 ? 2 : 1,
    maxSolutions: stage < 2 ? 999999 : Math.max(2, 10 - Math.floor(stage / 2)),
  };
}

module.exports = {
  OPERATORS,
  solve,
  solveDetailed,
  executeSteps,
  isVerifiedRecord,
  firstStep,
  makeVerifiedRecord,
  generatePuzzleSet,
  generateCampaignPuzzleBank,
  loadCampaignPuzzleBankFromData,
  createCampaignPuzzleBankBuilder,
  numberKey,
  difficultyScore,
  difficultyTier,
  endlessConfig,
};
