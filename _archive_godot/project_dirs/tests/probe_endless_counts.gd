extends SceneTree

const PuzzleGeneratorScript = preload("res://core/puzzle_generator.gd")
const EndlessModeScript = preload("res://core/endless_mode.gd")

func _init() -> void:
	var generator := PuzzleGeneratorScript.new()
	var candidates: Array = EndlessModeScript.SIMPLE_CANDIDATES + EndlessModeScript.ADVANCED_CANDIDATES
	for candidate in candidates:
		var solutions: Array = generator.solve(candidate)
		print("%s => %d" % [candidate, solutions.size()])
	quit()
