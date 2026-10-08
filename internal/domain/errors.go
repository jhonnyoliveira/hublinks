package domain

import "errors"

var (
	ErrNotFound = errors.New("não encontrado")
	ErrConflict = errors.New("conflito")
)

type ValidationError struct{ Fields map[string]string }

func (e ValidationError) Error() string { return "dados inválidos" }
