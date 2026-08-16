/*
 * Build the static campaign puzzle bank.
 * Run from this directory: node tools/build_campaign_puzzle_data.js
 */
const fs = require('fs');
const path = require('path');
const puzzle = require('../src/core/puzzle_generator.js');
const levelCatalog = require('../src/core/level_catalog.js');

const SEED = 240000;
const MULTIPLY = puzzle.OPERATORS[2];
const DIVIDE = puzzle.OPERATORS[3];

function numberKey(numbers) {
  // 闯关题库按“实际牌面顺序”去重，允许同一组数字以不同顺序出现，
  // 这样在数字限定为 1～10 时仍能生成完整的 200 关固定题库。
  return numbers.join(',');
}

function permutations(values) {
  const result = [];
  const visit = (items, path) => {
    if (items.length === 0) {
      result.push(path.slice());
      return;
    }
    const used = new Set();
    items.forEach((value, index) => {
      if (used.has(value)) return;
      used.add(value);
      visit(items.slice(0, index).concat(items.slice(index + 1)), path.concat(value));
    });
  };
  visit(values.slice(), []);
  return result;
}

function hash(text, seed) {
  let value = (Number(seed) >>> 0) ^ 2166136261;
  for (const char of text) {
    value ^= char.charCodeAt(0);
    value = Math.imul(value, 16777619);
  }
  return value >>> 0;
}

function makeCandidates() {
  const result = [];
  for (let first = 1; first <= 10; first += 1) {
    for (let second = first; second <= 10; second += 1) {
      for (let third = second; third <= 10; third += 1) {
        for (let fourth = third; fourth <= 10; fourth += 1) {
          const numbers = [first, second, third, fourth];
          const detailed = puzzle.solveDetailed(numbers);
          if (!detailed.length) continue;
          permutations(numbers).forEach((orderedNumbers) => result.push({
            numbers: orderedNumbers,
            key: numberKey(orderedNumbers),
            detailed: puzzle.solveDetailed(orderedNumbers),
            solutions: puzzle.solveDetailed(orderedNumbers).map((item) => item.expression),
            solutionCount: puzzle.solveDetailed(orderedNumbers).length,
          }));
        }
      }
    }
  }
  return result;
}

function policyFor(index, questionIndex) {
  const phase = index < 20 ? 0 : index < 50 ? 1 : index < 100 ? 2 : 3;
  const chapterLevel = index % 20;
  if (phase === 0) {
    const requiredByLevel = [
      [MULTIPLY, '+', MULTIPLY],
      ['+', MULTIPLY, '+'],
      [MULTIPLY, '+', '-'],
      ['+', MULTIPLY, '+'],
      [MULTIPLY, '+', '-'],
    ];
    return {
      phase,
      minDigit: 1,
      maxDigit: 9,
      minSolutions: 2,
      maxSolutions: 999,
      // 前 20 关仍然负责教学，但不再被“全加法”题目占满。
      // 四个 5 关小阶段逐步抬高最低难度，且每题至少使用两种运算符。
      minScore: 2 + Math.floor(chapterLevel / 5),
      maxScore: 8,
      requiredOperator: requiredByLevel[chapterLevel % 5][questionIndex],
      minOperatorKinds: 2,
      positiveOnly: true,
      targetScore: 2.5 + Math.floor(chapterLevel / 5) * 1.1,
    };
  }
  if (phase === 1) {
    return {
      phase,
      minDigit: 1,
      maxDigit: 10,
      minSolutions: 1,
      maxSolutions: 24,
      minScore: 4,
      maxScore: 12,
      minOperatorKinds: 2,
      requiredOperator: '',
      positiveOnly: false,
      targetScore: 5 + (chapterLevel / 19) * 4,
    };
  }
  if (phase === 2) {
    return {
      phase,
      minDigit: 2,
      maxDigit: 10,
      minSolutions: 1,
      maxSolutions: 12,
      minScore: 7,
      maxScore: 16,
      minOperatorKinds: 2,
      requiredOperator: questionIndex === 2 ? DIVIDE : '',
      positiveOnly: false,
      targetScore: 9 + (chapterLevel / 19) * 4,
    };
  }
  return {
    phase,
    minDigit: 1,
    maxDigit: 10,
    minSolutions: 1,
    maxSolutions: 4,
    minScore: 9,
    maxScore: 20,
    minOperatorKinds: 2,
    requiredOperator: questionIndex === 2 ? DIVIDE : '',
    positiveOnly: false,
    targetScore: 12 + (chapterLevel / 19) * 5,
  };
}

function variantsFor(candidate, policy) {
  return candidate.detailed.map((item) => {
    const operators = new Set(item.steps.map((step) => step.operator));
    const score = puzzle.difficultyScore(candidate.numbers, candidate.solutions, item);
    return { item, operators, score };
  }).filter((variant) => {
    if (variant.score < policy.minScore || variant.score > policy.maxScore) return false;
    if (variant.operators.size < policy.minOperatorKinds) return false;
    if (policy.requiredOperator && !variant.operators.has(policy.requiredOperator)) return false;
    if (policy.positiveOnly && !puzzle.executeSteps(candidate.numbers, variant.item.steps, { allowNegativeIntermediate: false })) return false;
    return true;
  });
}

function encode(candidate, variant) {
  const steps = variant.item.steps.map((step) => [
    step.firstIndices,
    step.secondIndices,
    step.first,
    step.second,
    step.operator,
  ]);
  return [candidate.numbers, steps, candidate.solutionCount, variant.score];
}

function build() {
  const levels = levelCatalog.all();
  const candidates = makeCandidates();
  const used = new Set();
  const bank = Array.from({ length: levels.length }, () => []);
  const failures = [];
  const order = Array.from({ length: levels.length }, (_, index) => index).sort((a, b) => b - a);
  const recentNumberKeys = [];

  for (const index of order) {
    const level = levels[index];
    const levelNumberKeys = new Set();
    for (let questionIndex = 0; questionIndex < level.questionCount; questionIndex += 1) {
      const policy = policyFor(index, questionIndex);
      const options = [];
      for (const candidate of candidates) {
        if (used.has(candidate.key)) continue;
        const numberSetKey = candidate.numbers.slice().sort((a, b) => a - b).join(',');
        if (levelNumberKeys.has(numberSetKey)) continue;
        if (recentNumberKeys.includes(numberSetKey)) continue;
        if (Math.min(...candidate.numbers) < policy.minDigit || Math.max(...candidate.numbers) > policy.maxDigit) continue;
        if (candidate.solutionCount < policy.minSolutions || candidate.solutionCount > policy.maxSolutions) continue;
        variantsFor(candidate, policy).forEach((variant) => options.push({
          candidate,
          variant,
          rank: Math.abs(variant.score - policy.targetScore),
          tie: hash(candidate.key, SEED + index * 9973 + questionIndex),
        }));
      }
      options.sort((left, right) => left.rank - right.rank || left.tie - right.tie);
      const selected = options[0];
      if (!selected) {
        failures.push({ level: index + 1, question: questionIndex + 1 });
        continue;
      }
      used.add(selected.candidate.key);
      levelNumberKeys.add(selected.candidate.numbers.slice().sort((a, b) => a - b).join(','));
      bank[index].push(encode(selected.candidate, selected.variant));
    }
    levelNumberKeys.forEach((numberSetKey) => recentNumberKeys.push(numberSetKey));
    while (recentNumberKeys.length > 24) recentNumberKeys.shift();
  }

  if (failures.length || bank.some((records, index) => records.length !== levels[index].questionCount)) {
    throw new Error(`campaign bank generation failed: ${JSON.stringify(failures.slice(0, 5))}`);
  }
  const output = `// Generated by tools/build_campaign_puzzle_data.js\nmodule.exports = ${JSON.stringify({ version: 2, seed: SEED, levels: bank })};\n`;
  fs.writeFileSync(path.join(__dirname, '..', 'src', 'core', 'campaign_puzzle_data.js'), output, 'utf8');
  console.log(JSON.stringify({ levels: levels.length, questions: used.size, candidates: candidates.length, output: 'src/core/campaign_puzzle_data.js' }));
}

build();
