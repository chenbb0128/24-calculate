package queue

import (
	"encoding/json"
	"fmt"

	"github.com/hibiken/asynq"
)

const TypeUserWelcome = "user.welcome"

func NewUserWelcomeTask(userID uint64) (*asynq.Task, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user id must be greater than zero")
	}
	payload, err := json.Marshal(UserWelcomePayload{Version: 1, UserID: userID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeUserWelcome, payload), nil
}
