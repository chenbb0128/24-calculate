// 生成两首儿童益智小游戏风格的试听音乐。
// 试听资源独立放在 assets/audio/previews，不会改变游戏当前使用的音乐。
const fs = require('fs');
const path = require('path');

const SAMPLE_RATE = 22050;
const TAU = Math.PI * 2;
const OUTPUT_DIR = path.resolve(__dirname, '../assets/audio/previews');

function midiToFrequency(midi) {
  return 440 * Math.pow(2, (midi - 69) / 12);
}

function clamp(value, min, max) {
  return Math.max(min, Math.min(max, value));
}

function wave(phase, kind) {
  const normalized = phase - Math.floor(phase);
  if (kind === 'triangle') return 1 - 4 * Math.abs(Math.round(normalized) - normalized);
  if (kind === 'square') return normalized < 0.5 ? 1 : -1;
  return Math.sin(TAU * normalized);
}

function envelope(localTime, duration, attack, release, decay = 0.12) {
  const attackPart = Math.min(1, localTime / Math.max(0.001, attack));
  const releasePart = Math.min(1, (duration - localTime) / Math.max(0.001, release));
  const decayPart = localTime < decay ? 1 : 0.82 + 0.18 * Math.max(0, 1 - (localTime - decay) / Math.max(0.001, duration - decay));
  return Math.max(0, attackPart * releasePart * decayPart);
}

function createTrack(bpm, bars) {
  const beat = 60 / bpm;
  const duration = bars * 4 * beat;
  const count = Math.ceil(duration * SAMPLE_RATE);
  return { beat, duration, left: new Float32Array(count), right: new Float32Array(count) };
}

function addTone(track, start, duration, midi, volume, kind, pan = 0, options = {}) {
  const startIndex = Math.max(0, Math.floor(start * SAMPLE_RATE));
  const endIndex = Math.min(track.left.length, Math.ceil((start + duration) * SAMPLE_RATE));
  const frequency = midiToFrequency(midi);
  const attack = options.attack ?? 0.012;
  const release = options.release ?? Math.min(0.16, duration * 0.35);
  const decay = options.decay ?? 0.08;
  const harmonics = options.harmonics || [{ ratio: 1, gain: 1 }];
  const vibrato = options.vibrato || 0;
  const vibratoRate = options.vibratoRate || 5.2;
  const leftGain = Math.cos((pan + 1) * Math.PI / 4);
  const rightGain = Math.sin((pan + 1) * Math.PI / 4);

  for (let index = startIndex; index < endIndex; index += 1) {
    const time = index / SAMPLE_RATE - start;
    const currentFrequency = frequency * (1 + vibrato * Math.sin(TAU * vibratoRate * time));
    let sample = 0;
    harmonics.forEach(({ ratio, gain }) => {
      sample += wave(currentFrequency * ratio * time, kind) * gain;
    });
    sample /= harmonics.reduce((sum, item) => sum + Math.abs(item.gain), 0) || 1;
    sample *= volume * envelope(time, duration, attack, release, decay);
    track.left[index] += sample * leftGain;
    track.right[index] += sample * rightGain;
  }
}

function addPluck(track, start, duration, midi, volume, pan = 0) {
  addTone(track, start, duration, midi, volume, 'triangle', pan, {
    attack: 0.004,
    release: Math.min(0.22, duration * 0.72),
    decay: 0.035,
    harmonics: [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.24 }, { ratio: 3, gain: 0.08 }],
  });
}

function addSoftKick(track, start, volume = 0.13) {
  const startIndex = Math.floor(start * SAMPLE_RATE);
  const endIndex = Math.min(track.left.length, startIndex + Math.floor(0.18 * SAMPLE_RATE));
  for (let index = startIndex; index < endIndex; index += 1) {
    const time = (index - startIndex) / SAMPLE_RATE;
    const frequency = 135 - 70 * Math.min(1, time / 0.18);
    const sample = Math.sin(TAU * frequency * time) * volume * Math.exp(-18 * time);
    track.left[index] += sample;
    track.right[index] += sample;
  }
}

function addShaker(track, start, volume = 0.025) {
  const startIndex = Math.floor(start * SAMPLE_RATE);
  const endIndex = Math.min(track.left.length, startIndex + Math.floor(0.045 * SAMPLE_RATE));
  let seed = (startIndex * 17 + 31) >>> 0;
  for (let index = startIndex; index < endIndex; index += 1) {
    seed = (1664525 * seed + 1013904223) >>> 0;
    const noise = ((seed / 0xffffffff) * 2 - 1) * volume;
    const time = (index - startIndex) / SAMPLE_RATE;
    const sample = noise * Math.exp(-55 * time);
    track.left[index] += sample * 0.75;
    track.right[index] += sample;
  }
}

function addBell(track, start, midi, volume = 0.12, pan = 0) {
  addTone(track, start, 0.62, midi, volume, 'sine', pan, {
    attack: 0.002,
    release: 0.52,
    decay: 0.03,
    harmonics: [{ ratio: 1, gain: 1 }, { ratio: 2.01, gain: 0.28 }, { ratio: 3.98, gain: 0.11 }],
  });
}

function addToyPiano(track, start, duration, midi, volume = 0.14, pan = 0) {
  addTone(track, start, duration, midi, volume, 'triangle', pan, {
    attack: 0.003,
    release: Math.min(0.18, duration * 0.48),
    decay: 0.025,
    harmonics: [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.32 }, { ratio: 3, gain: 0.1 }],
  });
}

function addChord(track, start, notes, volume, style) {
  notes.forEach((midi, index) => {
    if (style === 'home') {
      addTone(track, start, track.beat * 3.7, midi, volume, 'triangle', (index - 1.5) * 0.12, {
        attack: 0.08,
        release: 0.38,
        decay: 0.2,
        harmonics: [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.12 }],
      });
    } else {
      addPluck(track, start, track.beat * 0.92, midi, volume, (index - 1.5) * 0.18);
      addPluck(track, start + track.beat * 2, track.beat * 0.92, midi, volume * 0.72, (index - 1.5) * 0.18);
    }
  });
}

function finishTrack(track) {
  let peak = 0;
  for (let index = 0; index < track.left.length; index += 1) {
    peak = Math.max(peak, Math.abs(track.left[index]), Math.abs(track.right[index]));
  }
  const gain = peak > 0 ? Math.min(0.88 / peak, 1.8) : 1;
  const fadeSamples = Math.floor(0.12 * SAMPLE_RATE);
  for (let index = 0; index < track.left.length; index += 1) {
    const edgeFade = index < fadeSamples
      ? index / fadeSamples
      : index > track.left.length - fadeSamples ? (track.left.length - index) / fadeSamples : 1;
    track.left[index] = Math.tanh(track.left[index] * gain * 1.15) * edgeFade;
    track.right[index] = Math.tanh(track.right[index] * gain * 1.15) * edgeFade;
  }
}

function writeWav(fileName, track) {
  const sampleCount = track.left.length;
  const output = Buffer.alloc(44 + sampleCount * 4);
  output.write('RIFF', 0, 'ascii');
  output.writeUInt32LE(36 + sampleCount * 4, 4);
  output.write('WAVE', 8, 'ascii');
  output.write('fmt ', 12, 'ascii');
  output.writeUInt32LE(16, 16);
  output.writeUInt16LE(1, 20);
  output.writeUInt16LE(2, 22);
  output.writeUInt32LE(SAMPLE_RATE, 24);
  output.writeUInt32LE(SAMPLE_RATE * 4, 28);
  output.writeUInt16LE(4, 32);
  output.writeUInt16LE(16, 34);
  output.write('data', 36, 'ascii');
  output.writeUInt32LE(sampleCount * 4, 40);
  for (let index = 0; index < sampleCount; index += 1) {
    output.writeInt16LE(clamp(Math.round(track.left[index] * 32767), -32768, 32767), 44 + index * 4);
    output.writeInt16LE(clamp(Math.round(track.right[index] * 32767), -32768, 32767), 46 + index * 4);
  }
  fs.writeFileSync(path.join(OUTPUT_DIR, fileName), output);
}

function makeHomeMusic() {
  const track = createTrack(88, 16);
  const chords = [[60, 64, 67], [55, 59, 62], [57, 60, 64], [53, 57, 60]];
  const motifs = [
    [72, 76, 79, 76], [74, 77, 81, 77], [72, 76, 79, 84], [81, 79, 76, 74],
  ];
  for (let bar = 0; bar < 16; bar += 1) {
    const barStart = bar * 4 * track.beat;
    addChord(track, barStart, chords[bar % chords.length], 0.06, 'home');
    addPluck(track, barStart + track.beat * 0.5, track.beat * 0.8, chords[bar % 4][1] + 12, 0.055, -0.5);
    addPluck(track, barStart + track.beat * 2.5, track.beat * 0.8, chords[bar % 4][2] + 12, 0.05, 0.45);
    const motif = motifs[bar % motifs.length];
    motif.forEach((note, index) => {
      addToyPiano(track, barStart + track.beat * (index + 0.12), track.beat * 0.62, note, 0.12, index % 2 ? 0.12 : -0.12);
    });
    addBell(track, barStart + track.beat * 3.45, [84, 83, 81, 79][bar % 4], 0.04, 0.3);
    for (let beat = 0; beat < 4; beat += 1) addShaker(track, barStart + beat * track.beat, 0.014);
  }
  finishTrack(track);
  return track;
}

function makeLevelMusic() {
  const track = createTrack(112, 16);
  const chords = [[60, 64, 67], [57, 60, 64], [53, 57, 60], [55, 59, 62]];
  const motifs = [
    [72, 72, 76, 79, 79, 76, 74, 72],
    [76, 76, 79, 84, 81, 79, 76, 74],
    [72, 76, 79, 84, 81, 79, 76, 72],
    [74, 74, 77, 81, 79, 77, 74, 72],
  ];
  for (let bar = 0; bar < 16; bar += 1) {
    const barStart = bar * 4 * track.beat;
    addChord(track, barStart, chords[bar % chords.length], 0.052, 'level');
    addTone(track, barStart, track.beat * 3.8, chords[bar % 4][0] - 12, 0.07, 'triangle', 0, {
      attack: 0.02,
      release: 0.15,
      harmonics: [{ ratio: 1, gain: 1 }, { ratio: 2, gain: 0.12 }],
    });
    addSoftKick(track, barStart, 0.1);
    addSoftKick(track, barStart + track.beat * 2, 0.085);
    for (let beat = 0; beat < 4; beat += 1) {
      addShaker(track, barStart + beat * track.beat, 0.022);
      addShaker(track, barStart + (beat + 0.5) * track.beat, 0.016);
    }
    const motif = motifs[bar % motifs.length];
    motif.forEach((note, index) => {
      const start = barStart + index * track.beat * 0.5 + track.beat * 0.08;
      addToyPiano(track, start, track.beat * 0.34, note, 0.15, index % 2 ? 0.2 : -0.2);
      if (index === 7) addBell(track, start + track.beat * 0.34, note + 12, 0.05, -0.25);
    });
  }
  finishTrack(track);
  return track;
}

fs.mkdirSync(OUTPUT_DIR, { recursive: true });
writeWav('preview_home_childlike_v2.wav', makeHomeMusic());
writeWav('preview_level_childlike_v2.wav', makeLevelMusic());
console.log(`Generated childlike music previews in ${OUTPUT_DIR}`);
