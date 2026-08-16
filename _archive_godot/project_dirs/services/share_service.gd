class_name ShareService
extends RefCounted

## 微信分享适配接口。桌面版不调用微信 API，只生成可验证的分享参数。

static func create_friend_room_payload(room: Dictionary) -> Dictionary:
	var room_code := str(room.get("room_code", ""))
	var room_seed := int(room.get("room_seed", 0))
	return {
		"title": "来和我挑战《24点挑战》！",
		"path": "/pages/index/index?mode=friend&room=%s&seed=%d" % [room_code, room_seed],
		"query": "mode=friend&room=%s&seed=%d" % [room_code, room_seed],
		"room_code": room_code,
		"room_seed": room_seed,
		"supported": false,
	}


static func build_invite_text(room: Dictionary) -> String:
	return "来和我比一局《24点挑战》！房间号：%s" % str(room.get("room_code", ""))


static func parse_launch_params(params: Dictionary) -> Dictionary:
	return {
		"mode": str(params.get("mode", "")),
		"room_code": str(params.get("room", "")),
		"room_seed": int(params.get("seed", 0)),
	}


static func create_match_result_payload(result: Dictionary, room: Dictionary) -> Dictionary:
	var outcome := str(result.get("outcome", "draw"))
	var outcome_text := "赢下了对战" if outcome == "win" else ("打成平局" if outcome == "draw" else "完成了挑战")
	var room_code := str(room.get("room_code", ""))
	var score := int(result.get("player_score", 0))
	return {
		"title": "我在《24点挑战》中%s！" % outcome_text,
		"path": "/pages/index/index?mode=friend&room=%s" % room_code,
		"query": "mode=friend&room=%s" % room_code,
		"room_code": room_code,
		"score": score,
		"outcome": outcome,
		"supported": false,
	}


static func build_result_card_text(result: Dictionary) -> String:
	var outcome := str(result.get("outcome", "draw"))
	var outcome_text := "击败好友" if outcome == "win" else ("和好友打平" if outcome == "draw" else "完成对战")
	return "我在《24点挑战》中%s！\n答对 %d 题 · %d 分 · 用时 %.1f 秒" % [outcome_text, int(result.get("player_solved", 0)), int(result.get("player_score", 0)), float(result.get("player_elapsed", 0.0))]
