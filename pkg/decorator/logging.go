package decorator

import (
	"context"
	"fmt"

	"github.com/architeacher/devices/pkg/logger"
)

type commandLoggingDecorator[C Command, R any] struct {
	base   CommandHandler[C, R]
	logger logger.Logger
}

func (d commandLoggingDecorator[C, R]) Handle(ctx context.Context, cmd C) (result R, err error) {
	d.logger.Trace().
		Str("command", generateActionName(cmd)).
		Str("command_body", fmt.Sprintf("%#v", cmd))

	defer func() {
		if err == nil {
			d.logger.Trace().Msg("command executed successfully")

			return
		}

		d.logger.Error().Err(err).Msg("failed to execute command")
	}()

	return d.base.Handle(ctx, cmd)
}

type queryLoggingDecorator[Q Query, R Result] struct {
	base   QueryHandler[Q, R]
	logger logger.Logger
}

func (d queryLoggingDecorator[Q, R]) Execute(ctx context.Context, query Q) (result R, err error) {
	d.logger.Trace().
		Str("query", generateActionName(query)).
		Str("query_body", fmt.Sprintf("%#v", query))

	defer func() {
		if err == nil {
			d.logger.Trace().Msg("query executed successfully")

			return
		}

		d.logger.Error().Err(err).Msg("failed to execute query")
	}()

	return d.base.Execute(ctx, query)
}
