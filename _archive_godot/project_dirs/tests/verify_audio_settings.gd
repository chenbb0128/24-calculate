extends SceneTree

const AudioServiceScript = preload("res://services/audio_service.gd")
const SaveServiceScript = preload("res://services/save_service.gd")

func _init() -> void:
	var service := AudioServiceScript.new()
	root.add_child(service)
	await process_frame
	assert(service.get_music_track_count() == 3, "应有 3 首原创背景音乐")
	assert(service.music_streams[0].data != service.music_streams[1].data, "第 1、2 首音乐数据不能相同")
	assert(service.music_streams[1].data != service.music_streams[2].data, "第 2、3 首音乐数据不能相同")
	assert(service.music_streams[0].data.size() != service.music_streams[2].data.size(), "慢速音乐应有不同长度")
	assert(service.get_music_track_name() == "晨光算术", "默认音乐名称错误")
	service.set_music_track(1)
	assert(service.get_music_track() == 1, "音乐切换失败")
	service.set_music_track(99)
	assert(service.get_music_track() == 0, "音乐索引应循环")
	service.set_music_volume(-100.0)
	assert(is_equal_approx(service.get_music_volume(), -36.0), "背景音乐音量下限错误")
	service.set_sfx_volume(100.0)
	assert(is_equal_approx(service.get_sfx_volume(), 0.0), "按键音效音量上限错误")
	service.apply_settings({
		"music_enabled": false,
		"sfx_enabled": false,
		"music_track": 2,
		"music_volume_db": -18.0,
		"sfx_volume_db": -6.0,
	})
	assert(service.get_music_track() == 2, "设置应用失败")
	assert(is_equal_approx(service.get_music_volume(), -18.0), "背景音乐设置应用失败")
	assert(is_equal_approx(service.get_sfx_volume(), -6.0), "按键音效设置应用失败")
	var default_progress := SaveServiceScript._default_progress()
	assert(default_progress.has("audio"), "新存档缺少音频设置")
	var old_progress := {"version": 5, "coins": 10}
	var loaded := SaveServiceScript._migrate_for_test(old_progress)
	assert(loaded["audio"].has("music_enabled"), "旧存档无法补全音频设置")
	print("Audio settings verification passed: 3 tracks, independent toggles and volume persistence")
	quit()
