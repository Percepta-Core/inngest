package resolvers

import (
	"context"
	"fmt"
	"time"

	loader "github.com/inngest/inngest/pkg/coreapi/graph/loaders"
	"github.com/inngest/inngest/pkg/coreapi/graph/models"
	"github.com/inngest/inngest/pkg/cqrs"
	"github.com/inngest/inngest/pkg/logger"
)

func (r *functionRunV2Resolver) App(
	ctx context.Context,
	run *models.FunctionRunV2,
) (*cqrs.App, error) {
	t0 := time.Now()
	l := logger.StdlibLogger(ctx)
	defer func() {
		l.Info("[perf] field resolver: App", "duration_ms", time.Since(t0).Milliseconds(), "app_id", run.AppID)
	}()

	if cache := loader.GetLookupCache(ctx); cache != nil {
		v, err := cache.GetOrLoad("app", run.AppID, func() (interface{}, error) {
			return r.Data.GetAppByID(ctx, run.AppID)
		})
		if err != nil {
			return nil, err
		}
		return v.(*cqrs.App), nil
	}
	return r.Data.GetAppByID(ctx, run.AppID)
}

func (r *functionRunV2Resolver) Function(ctx context.Context, fn *models.FunctionRunV2) (*models.Function, error) {
	t0 := time.Now()
	l := logger.StdlibLogger(ctx)
	defer func() {
		l.Info("[perf] field resolver: Function", "duration_ms", time.Since(t0).Milliseconds(), "function_id", fn.FunctionID)
	}()

	if cache := loader.GetLookupCache(ctx); cache != nil {
		v, err := cache.GetOrLoad("func", fn.FunctionID, func() (interface{}, error) {
			fun, err := r.Data.GetFunctionByInternalUUID(ctx, fn.FunctionID)
			if err != nil {
				return nil, fmt.Errorf("error retrieving function: %w", err)
			}
			return models.MakeFunction(fun)
		})
		if err != nil {
			return nil, err
		}
		return v.(*models.Function), nil
	}

	fun, err := r.Data.GetFunctionByInternalUUID(ctx, fn.FunctionID)
	if err != nil {
		return nil, fmt.Errorf("error retrieving function: %w", err)
	}
	return models.MakeFunction(fun)
}

func (r *functionRunV2Resolver) Trace(ctx context.Context, fn *models.FunctionRunV2, preview *bool) (*models.RunTraceSpan, error) {
	targetLoader := loader.FromCtx(ctx).LegacyRunTraceLoader
	if preview != nil && *preview {
		targetLoader = loader.FromCtx(ctx).RunTraceLoader
	}

	return loader.LoadOne[models.RunTraceSpan](
		ctx,
		targetLoader,
		&loader.TraceRequestKey{
			TraceRunIdentifier: &cqrs.TraceRunIdentifier{
				AppID:      fn.AppID,
				FunctionID: fn.FunctionID,
				RunID:      fn.ID,
				TraceID:    fn.TraceID,
			},
		},
	)
}
