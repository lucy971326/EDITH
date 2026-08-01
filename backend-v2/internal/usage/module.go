// Package usage 记录 Agent 运行产生的 Token 用量。
package usage

import (
	"context"
	"database/sql"
	"errors"
)

// Module 是用量模块对外提供的能力集合。
type Module struct {
	Recorder *Recorder
	Reader   *Reader
}

// Dependencies 是创建用量模块需要的长期依赖。
type Dependencies struct {
	DB *sql.DB
}

// New 创建用量模块，并在传入的数据库中创建自己的表。
func New(deps Dependencies) (*Module, error) {
	if deps.DB == nil {
		return nil, errors.New("usage database is required")
	}

	store := &store{db: deps.DB}
	if err := store.createSchema(context.Background()); err != nil {
		return nil, err
	}

	return &Module{
		Recorder: &Recorder{store: store},
		Reader:   &Reader{store: store},
	}, nil
}
