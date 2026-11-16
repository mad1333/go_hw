package domain

import (
	"context"
	"errors"
)

type Movie struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Year     int32  `json:"year"`
	Director string `json:"director"`
}
type Failure struct {
	ID      int    `json:"id"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
type Batch struct {
	Movies []Movie   `json:"movies"`
	Errors []Failure `json:"errors"`
}
type Error struct{ Code, Message string }

func (e *Error) Error() string { return e.Message }
func Code(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Code
	}
	return "upstream"
}

type PartialError struct{ Failures []Failure }

func (e *PartialError) Error() string { return "some movies could not be loaded" }
