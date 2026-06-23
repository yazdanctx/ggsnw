package source

import "context"

type Source interface {
	Name() string
	Expand(ctx context.Context, shortname string) ([]string, error)
}
