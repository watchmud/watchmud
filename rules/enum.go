package rules

import (
	"fmt"
	"strings"
)

type enum[T ~string] struct {
	name   string
	err    error // the sentinel, so errors.Is keeps working
	values []T
}

// parse matches case-insensitivity and returns the declared value,
// so "FollowPath" is WanderFollowPath and the constant's spelling wins.
func (e enum[T]) parse(s string) (T, error) {
	for _, v := range e.values {
		if strings.EqualFold(string(v), s) {
			return v, nil
		}
	}
	var none T
	return none, fmt.Errorf("%w: %q", e.err, s)
}

func (e enum[T]) unmarshal(dst *T, b []byte) error {
	v, err := e.parse(string(b))
	if err == nil {
		*dst = v
	}
	return err
}
