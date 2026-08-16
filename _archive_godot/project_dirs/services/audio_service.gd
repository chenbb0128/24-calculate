extends Node
class_name AudioService

## 原创程序化音频服务。
## 背景音乐和音效都由简单波形实时生成，不依赖第三方音乐文件，避免版权不明。

const SAMPLE_RATE := 22050
const MUSIC_VOLUME_DB := -25.0
const SFX_VOLUME_DB := -9.0
const MUSIC_TRACK_NAMES := ["晨光算术", "彩虹跳跳", "星星漫步"]

var music_player: AudioStreamPlayer
var music_streams: Array[AudioStreamWAV] = []
var sfx_players: Array[AudioStreamPlayer] = []
var sfx_streams: Dictionary = {}
var sfx_index := 0
var music_enabled := true
var sfx_enabled := true
var music_track_index := 0
var music_volume_db := MUSIC_VOLUME_DB
var sfx_volume_db := SFX_VOLUME_DB
var countdown_active := false
var countdown_last_second := -1


func _ready() -> void:
	process_mode = Node.PROCESS_MODE_ALWAYS
	_build_music()
	_build_sfx()
	_start_music()


func apply_settings(settings: Dictionary) -> void:
	set_music_volume(float(settings.get("music_volume_db", MUSIC_VOLUME_DB)))
	set_sfx_volume(float(settings.get("sfx_volume_db", SFX_VOLUME_DB)))
	set_music_track(int(settings.get("music_track", 0)))
	music_enabled = bool(settings.get("music_enabled", true))
	sfx_enabled = bool(settings.get("sfx_enabled", true))
	if music_enabled:
		_start_music()
	else:
		music_player.stop()


func resume_music() -> void:
	# Web/微信小游戏可能拦截启动时的自动播放；首次点击后再尝试播放。
	_start_music()


func set_music_enabled(enabled: bool) -> void:
	music_enabled = enabled
	if music_player:
		if enabled:
			_start_music()
		else:
			music_player.stop()


func set_sfx_enabled(enabled: bool) -> void:
	sfx_enabled = enabled


func set_music_volume(volume_db: float) -> void:
	music_volume_db = clampf(volume_db, -36.0, -6.0)
	if music_player:
		music_player.volume_db = music_volume_db


func set_sfx_volume(volume_db: float) -> void:
	sfx_volume_db = clampf(volume_db, -24.0, 0.0)


func set_music_track(track_index: int) -> void:
	if music_streams.is_empty():
		return
	music_track_index = posmod(track_index, music_streams.size())
	if not music_player:
		return
	music_player.stream = music_streams[music_track_index]
	music_player.volume_db = music_volume_db
	if music_enabled:
		music_player.play()


func get_music_track() -> int:
	return music_track_index


func get_music_track_name() -> String:
	return str(MUSIC_TRACK_NAMES[music_track_index]) if music_track_index < MUSIC_TRACK_NAMES.size() else "原创旋律"


func get_music_track_count() -> int:
	return music_streams.size()


func get_music_volume() -> float:
	return music_volume_db


func get_sfx_volume() -> float:
	return sfx_volume_db


func play_click() -> void:
	_play_sfx("click")


func play_card() -> void:
	_play_sfx("card")


func play_operator() -> void:
	_play_sfx("operator")


func play_merge() -> void:
	_play_sfx("merge")


func play_success() -> void:
	_play_sfx("success")


func play_error() -> void:
	_play_sfx("error")


func start_countdown() -> void:
	countdown_active = true
	countdown_last_second = -1


func stop_countdown() -> void:
	countdown_active = false
	countdown_last_second = -1


func update_countdown(time_left: float) -> void:
	if not countdown_active:
		return
	var second := int(ceil(time_left))
	if second <= 10 and second >= 1 and second != countdown_last_second:
		countdown_last_second = second
		if second <= 3:
			_play_sfx("urgent")
		else:
			_play_sfx("tick")
	if time_left <= 0.0:
		stop_countdown()


func _start_music() -> void:
	if not music_enabled or not music_player:
		return
	if not music_player.playing:
		music_player.play()


func _play_sfx(kind: String) -> void:
	if not sfx_enabled or sfx_players.is_empty() or not sfx_streams.has(kind):
		return
	var player: AudioStreamPlayer = sfx_players[sfx_index]
	sfx_index = (sfx_index + 1) % sfx_players.size()
	player.stream = sfx_streams[kind]
	player.volume_db = sfx_volume_db
	player.play()


func _build_music() -> void:
	music_player = AudioStreamPlayer.new()
	music_player.name = "OriginalBackgroundMusic"
	music_player.volume_db = music_volume_db
	for track_index in range(MUSIC_TRACK_NAMES.size()):
		var music_stream := _make_music_stream(track_index)
		music_stream.loop_mode = AudioStreamWAV.LOOP_FORWARD
		music_stream.loop_begin = 0
		music_stream.loop_end = int(music_stream.data.size() / 2)
		music_streams.append(music_stream)
	music_player.stream = music_streams[0]
	add_child(music_player)


func _build_sfx() -> void:
	for index in range(4):
		var player := AudioStreamPlayer.new()
		player.name = "SfxPlayer%d" % index
		add_child(player)
		sfx_players.append(player)
	sfx_streams["click"] = _make_tone_stream(620.0, 0.045, 0.34, 90.0)
	sfx_streams["card"] = _make_tone_stream(720.0, 0.055, 0.30, 150.0)
	sfx_streams["operator"] = _make_tone_stream(480.0, 0.075, 0.28, 180.0)
	sfx_streams["merge"] = _make_tone_stream(380.0, 0.16, 0.34, 520.0)
	sfx_streams["success"] = _make_melody_stream([523.25, 659.25, 783.99], 0.09, 0.30)
	sfx_streams["error"] = _make_tone_stream(260.0, 0.12, 0.24, -100.0)
	sfx_streams["tick"] = _make_tone_stream(880.0, 0.045, 0.20, 0.0)
	sfx_streams["urgent"] = _make_tone_stream(700.0, 0.075, 0.25, -90.0)


func _make_music_stream(track_index: int) -> AudioStreamWAV:
	var notes: Array
	var note_length: float
	match track_index:
		1:
			# 彩虹跳跳：短音符、明亮高音和明显的跳跃节奏。
			notes = [523.25, 659.25, 783.99, 659.25, 587.33, 698.46, 880.0, 698.46, 523.25, 783.99, 987.77, 783.99, 659.25, 880.0, 1046.5, 880.0, 783.99, 659.25, 523.25, 659.25, 783.99, 987.77, 880.0, 783.99, 659.25, 587.33, 698.46, 880.0, 1046.5, 880.0, 783.99, 659.25]
			note_length = 0.25
		2:
			# 星星漫步：慢速长音、低音铺底和闪烁泛音，听感更舒缓。
			notes = [220.0, 261.63, 329.63, 392.0, 329.63, 293.66, 246.94, 196.0, 220.0, 293.66, 349.23, 440.0]
			note_length = 0.72
		_:
			# 晨光算术：轻柔的中速主旋律，作为默认背景音乐。
			notes = [261.63, 329.63, 392.0, 329.63, 293.66, 349.23, 440.0, 349.23, 261.63, 329.63, 392.0, 523.25, 440.0, 392.0, 329.63, 293.66]
			note_length = 0.5
	var sample_count := int(notes.size() * note_length * SAMPLE_RATE)
	var data := PackedByteArray()
	data.resize(sample_count * 2)
	var byte_index := 0
	for sample_index in range(sample_count):
		var time := float(sample_index) / float(SAMPLE_RATE)
		var note_index := mini(notes.size() - 1, int(time / note_length))
		var local_time := fmod(time, note_length)
		var duration_left := note_length - local_time
		var attack_time := 0.035 if track_index != 1 else 0.012
		var release_time := 0.12 if track_index != 2 else 0.22
		var envelope := minf(1.0, local_time / attack_time) * minf(1.0, duration_left / release_time)
		var frequency: float = float(notes[note_index])
		var wave: float
		if track_index == 1:
			var beat_phase := fmod(time, 0.5)
			var beat_pulse := 1.0 if beat_phase < 0.08 else 0.62
			var lead := sin(TAU * frequency * time) * 0.17
			lead += sin(TAU * frequency * 2.0 * time) * 0.09
			lead += sin(TAU * frequency * 3.0 * time) * 0.035
			var bass := sin(TAU * frequency * 0.5 * time) * 0.12 * beat_pulse
			var bounce := sin(TAU * 110.0 * time) * 0.025 * (1.0 - beat_pulse * 0.45)
			wave = lead + bass + bounce
		elif track_index == 2:
			var pad := sin(TAU * frequency * time) * 0.13
			pad += sin(TAU * frequency * 0.5 * time) * 0.085
			pad += sin(TAU * frequency * 1.5 * time) * 0.045
			var detuned := sin(TAU * (frequency + 2.2) * time) * 0.035
			var sparkle := sin(TAU * frequency * 2.0 * time) * 0.025
			wave = pad + detuned + sparkle
		else:
			var lead := sin(TAU * frequency * time) * 0.22
			lead += sin(TAU * frequency * 2.0 * time) * 0.055
			var bass := sin(TAU * frequency * 0.5 * time) * 0.07
			var shimmer := sin(TAU * frequency * 1.5 * time) * 0.025
			wave = lead + bass + shimmer
		_write_sample(data, byte_index, wave * envelope)
		byte_index += 2
	var stream := AudioStreamWAV.new()
	stream.format = AudioStreamWAV.FORMAT_16_BITS
	stream.mix_rate = SAMPLE_RATE
	stream.stereo = false
	stream.data = data
	return stream


func _make_tone_stream(start_frequency: float, duration: float, volume: float, frequency_delta: float) -> AudioStreamWAV:
	var sample_count := maxi(1, int(duration * SAMPLE_RATE))
	var data := PackedByteArray()
	data.resize(sample_count * 2)
	var byte_index := 0
	for sample_index in range(sample_count):
		var time := float(sample_index) / float(SAMPLE_RATE)
		var progress := float(sample_index) / float(sample_count)
		var envelope := minf(1.0, time / 0.008) * minf(1.0, (duration - time) / 0.035)
		var frequency := start_frequency + frequency_delta * progress
		var wave := sin(TAU * frequency * time) * 0.82
		wave += sin(TAU * frequency * 2.0 * time) * 0.12
		_write_sample(data, byte_index, wave * volume * envelope)
		byte_index += 2
	return _make_wav(data)


func _make_melody_stream(notes: Array, note_duration: float, volume: float) -> AudioStreamWAV:
	var sample_count := maxi(1, int(notes.size() * note_duration * SAMPLE_RATE))
	var data := PackedByteArray()
	data.resize(sample_count * 2)
	var byte_index := 0
	for sample_index in range(sample_count):
		var time := float(sample_index) / float(SAMPLE_RATE)
		var note_index := mini(notes.size() - 1, int(time / note_duration))
		var local_time := fmod(time, note_duration)
		var duration_left := note_duration - local_time
		var envelope := minf(1.0, local_time / 0.008) * minf(1.0, duration_left / 0.035)
		var frequency: float = float(notes[note_index])
		var wave := sin(TAU * frequency * time) * 0.78
		wave += sin(TAU * frequency * 2.0 * time) * 0.16
		_write_sample(data, byte_index, wave * volume * envelope)
		byte_index += 2
	return _make_wav(data)


func _make_wav(data: PackedByteArray) -> AudioStreamWAV:
	var stream := AudioStreamWAV.new()
	stream.format = AudioStreamWAV.FORMAT_16_BITS
	stream.mix_rate = SAMPLE_RATE
	stream.stereo = false
	stream.data = data
	return stream


func _write_sample(data: PackedByteArray, byte_index: int, sample: float) -> void:
	var value := clampi(int(sample * 32767.0), -32767, 32767)
	if value < 0:
		value += 65536
	data[byte_index] = value & 0xff
	data[byte_index + 1] = (value >> 8) & 0xff
