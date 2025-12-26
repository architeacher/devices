package decorator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/architeacher/devices/pkg/metrics"
)

type (
	commandMetricsDecorator[C Command, R any] struct {
		base   CommandHandler[C, R]
		client metrics.Client
	}

	queryMetricsDecorator[Q Query, R Result] struct {
		base   QueryHandler[Q, R]
		client metrics.Client
	}
)

func (d commandMetricsDecorator[C, R]) Handle(ctx context.Context, cmd C) (result R, err error) {
	start := time.Now()

	actionName := strings.ToLower(generateActionName(cmd))

	defer func() {
		if d.client == nil {
			return
		}

		end := time.Since(start)

		d.client.Inc(ctx, fmt.Sprintf("commands.%s.duration", actionName), int64(end.Seconds()))

		if err == nil {
			d.client.Inc(ctx, fmt.Sprintf("commands.%s.success", actionName), 1)
		} else {
			d.client.Inc(ctx, fmt.Sprintf("commands.%s.failure", actionName), 1)
		}
	}()

	return d.base.Handle(ctx, cmd)
}

func (d queryMetricsDecorator[Q, R]) Execute(ctx context.Context, query Q) (result R, err error) {
	start := time.Now()

	actionName := strings.ToLower(generateActionName(query))

	defer func() {
		if d.client == nil {
			return
		}

		end := time.Since(start)

		d.client.Inc(ctx, fmt.Sprintf("queries.%s.duration", actionName), int64(end.Seconds()))

		if err == nil {
			d.client.Inc(ctx, fmt.Sprintf("queries.%s.success", actionName), 1)
		} else {
			d.client.Inc(ctx, fmt.Sprintf("queries.%s.failure", actionName), 1)
		}
	}()

	return d.base.Execute(ctx, query)
}
