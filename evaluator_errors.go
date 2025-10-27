package opts

import (
	"fmt"
	"strings"
)

func wrapEvaluatorError(engine string, err error) error {
	if err == nil {
		return nil
	}
	if strings.HasPrefix(err.Error(), "opts:") {
		return err
	}
	return fmt.Errorf("opts: %s evaluator: %w", engine, err)
}
