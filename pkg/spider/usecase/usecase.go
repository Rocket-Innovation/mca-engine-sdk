package usecase

import (
	"github.com/Rocket-Innovation/spider-go/pkg/spider"
)

type Usecase struct {
	storage spider.WorkflowStorageAdapter
}

func NewUsecase(storage spider.WorkflowStorageAdapter) *Usecase {
	return &Usecase{
		storage: storage,
	}
}
