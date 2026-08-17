// 生成《三火算术练习》的原创程序化音频资源。
// 参数与 Godot 原型中的 AudioService.gd 保持一致，不依赖第三方素材。
const fs = require('fs');
const path = require('path');

const SAMPLE_RATE = 22050;
const TAU = Math.PI * 2;
const OUTPUT_DIR = path.resolve(__dirname, '../assets/audio');

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function toPcm16(value) {
  return Math.round(clamp(value, -0.999969, 0.999969) * 32767);
}

function writeWav(fileName, samples) {
  const output = Buffer.alloc(44 + samples.length * 2);
  output.write('RIFF', 0, 'ascii');
  output.writeUInt32LE(36 + samples.length * 2, 4);
  output.write('WAVE', 8, 'ascii');
  output.write('fmt ', 12, 'ascii');
  output.writeUInt32LE(16, 16);
  output.writeUInt16LE(1, 20); // PCM
  output.writeUInt16LE(1, 22); // mono
  output.writeUInt32LE(SAMPLE_RATE, 24);
  output.writeUInt32LE(SAMPLE_RATE * 2, 28);
  output.writeUInt16LE(2, 32);
  output.writeUInt16LE(16, 34);
  output.write('data', 36, 'ascii');
  output.writeUInt32LE(samples.length * 2, 40);
  samples.forEach((sample, index) => output.writeInt16LE(toPcm16(sample), 44 + index * 2));
  fs.writeFileSync(path.join(OUTPUT_DIR, fileName), output);
}

function makeMusic(notes, noteLength, trackIndex) {
  const sampleCount = Math.floor(notes.length * noteLength * SAMPLE_RATE);
  const samples = new Array(sampleCount);
  for (let index = 0; index < sampleCount; index += 1) {
    const time = index / SAMPLE_RATE;
    const noteIndex = Math.min(notes.length - 1, Math.floor(time / noteLength));
    const localTime = time % noteLength;
    const durationLeft = noteLength - localTime;
    const attackTime = trackIndex === 1 ? 0.012 : 0.035;
    const releaseTime = trackIndex === 2 ? 0.22 : 0.12;
    const envelope = Math.min(1, localTime / attackTime) * Math.min(1, durationLeft / releaseTime);
    const frequency = notes[noteIndex];
    let wave;
    if (trackIndex === 1) {
      const beatPhase = time % 0.5;
      const beatPulse = beatPhase < 0.08 ? 1 : 0.62;
      let lead = Math.sin(TAU * frequency * time) * 0.17;
      lead += Math.sin(TAU * frequency * 2 * time) * 0.09;
      lead += Math.sin(TAU * frequency * 3 * time) * 0.035;
      const bass = Math.sin(TAU * frequency * 0.5 * time) * 0.12 * beatPulse;
      const bounce = Math.sin(TAU * 110 * time) * 0.025 * (1 - beatPulse * 0.45);
      wave = lead + bass + bounce;
    } else if (trackIndex === 2) {
      let pad = Math.sin(TAU * frequency * time) * 0.13;
      pad += Math.sin(TAU * frequency * 0.5 * time) * 0.085;
      pad += Math.sin(TAU * frequency * 1.5 * time) * 0.045;
      const detuned = Math.sin(TAU * (frequency + 2.2) * time) * 0.035;
      const sparkle = Math.sin(TAU * frequency * 2 * time) * 0.025;
      wave = pad + detuned + sparkle;
    } else {
      let lead = Math.sin(TAU * frequency * time) * 0.22;
      lead += Math.sin(TAU * frequency * 2 * time) * 0.055;
      const bass = Math.sin(TAU * frequency * 0.5 * time) * 0.07;
      const shimmer = Math.sin(TAU * frequency * 1.5 * time) * 0.025;
      wave = lead + bass + shimmer;
    }
    samples[index] = wave * envelope;
  }
  return samples;
}

function makeTone(startFrequency, duration, volume, frequencyDelta) {
  const sampleCount = Math.max(1, Math.floor(duration * SAMPLE_RATE));
  const samples = new Array(sampleCount);
  for (let index = 0; index < sampleCount; index += 1) {
    const time = index / SAMPLE_RATE;
    const progress = index / sampleCount;
    const envelope = Math.min(1, time / 0.008) * Math.min(1, (duration - time) / 0.035);
    const frequency = startFrequency + frequencyDelta * progress;
    let wave = Math.sin(TAU * frequency * time) * 0.82;
    wave += Math.sin(TAU * frequency * 2 * time) * 0.12;
    samples[index] = wave * volume * envelope;
  }
  return samples;
}

function makeMelody(notes, noteDuration, volume) {
  const sampleCount = Math.max(1, Math.floor(notes.length * noteDuration * SAMPLE_RATE));
  const samples = new Array(sampleCount);
  for (let index = 0; index < sampleCount; index += 1) {
    const time = index / SAMPLE_RATE;
    const noteIndex = Math.min(notes.length - 1, Math.floor(time / noteDuration));
    const localTime = time % noteDuration;
    const durationLeft = noteDuration - localTime;
    const envelope = Math.min(1, localTime / 0.008) * Math.min(1, durationLeft / 0.035);
    const frequency = notes[noteIndex];
    let wave = Math.sin(TAU * frequency * time) * 0.78;
    wave += Math.sin(TAU * frequency * 2 * time) * 0.16;
    samples[index] = wave * volume * envelope;
  }
  return samples;
}

// 背景音乐由 generate_music_previews.js 生成；本脚本只保留音效生成。
const tracks = [];

fs.mkdirSync(OUTPUT_DIR, { recursive: true });
tracks.forEach(([fileName, notes, length, index]) => writeWav(fileName, makeMusic(notes, length, index)));
writeWav('click.wav', makeTone(620, 0.045, 0.34, 90));
writeWav('card.wav', makeTone(720, 0.055, 0.30, 150));
writeWav('operator.wav', makeTone(480, 0.075, 0.28, 180));
writeWav('merge.wav', makeTone(380, 0.16, 0.34, 520));
writeWav('success.wav', makeMelody([523.25, 659.25, 783.99], 0.09, 0.30));
writeWav('error.wav', makeTone(260, 0.12, 0.24, -100));
writeWav('countdown_tick.wav', makeTone(880, 0.045, 0.20, 0));
writeWav('countdown_urgent.wav', makeTone(700, 0.075, 0.25, -90));

console.log(`Generated ${tracks.length + 8} WAV sound-effect files in ${OUTPUT_DIR}`);
