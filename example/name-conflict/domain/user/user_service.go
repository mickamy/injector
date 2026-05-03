package user

import "fmt"

type Service interface {
	Create(user User) error
}

type service struct {
	repository Repository
}

func (s *service) Create(user User) error {
	if err := s.repository.Create(user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func NewService(repository Repository) Service {
	return &service{repository: repository}
}
