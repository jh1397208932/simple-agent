package errorUtil

import (
	"fmt"
	"log"
)

type logError struct {
	errStr string
}

func (e *logError) Error() string {
	return e.errStr
}
func New(errStr string) error {
	log.Printf("error: %s", errStr)
	return &logError{errStr: errStr}
}
func NewF(format string, v ...any) error {
	return New(fmt.Sprintf(format, v...))
}
