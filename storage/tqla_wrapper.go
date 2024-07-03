package storage

import (
	"github.com/VauntDev/tqla"
)

type TQLATemplate interface {
	Compile(template string, data interface{}) (string, []interface{}, error)
}

type tqlaWrapper struct {
	compileFunc func(template string, data interface{}) (string, []interface{}, error)
}

func (w *tqlaWrapper) Compile(template string, data interface{}) (string, []interface{}, error) {
	return w.compileFunc(template, data)
}

func NewTQLATemplate(opts ...tqla.Option) (TQLATemplate, error) {
	t, err := tqla.New(opts...)
	if err != nil {
		return nil, err
	}
	return &tqlaWrapper{
		compileFunc: t.Compile,
	}, nil
}
