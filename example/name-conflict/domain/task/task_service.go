package task

import (
	"fmt"

	"github.com/mickamy/injector/example/name-conflict/domain/user"
)

type Service interface {
	Create(userID string, task Task) error
}

type service struct {
	taskRepository Repository
	userRepository user.Repository
}

func (s service) Create(userID string, task Task) error {
	u, err := s.userRepository.Get(userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	task.UserID = u.ID
	if err := s.taskRepository.Create(task); err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func NewService(taskRepository Repository, userRepository user.Repository) Service {
	return &service{
		taskRepository: taskRepository,
		userRepository: userRepository,
	}
}
