// Copyright (c) 2021 The Jaeger Authors.
// SPDX-License-Identifier: Apache-2.0

package apiv3

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gogo/protobuf/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/jaegertracing/jaeger/cmd/query/app/internal/api_v3"
	"github.com/jaegertracing/jaeger/cmd/query/app/querysvc"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/spanstore"
)

// Handler implements api_v3.QueryServiceServer
type Handler struct {
	QueryService *querysvc.QueryService
}

// remove me
var _ api_v3.QueryServiceServer = (*Handler)(nil)

// GetTrace implements api_v3.QueryServiceServer's GetTrace
func (h *Handler) GetTrace(request *api_v3.GetTraceRequest, stream api_v3.QueryService_GetTraceServer) error {
	traceID, err := model.TraceIDFromString(request.GetTraceId())
	if err != nil {
		return fmt.Errorf("malform trace ID: %w", err)
	}

	query := spanstore.TraceGetParameters{
		TraceID: traceID,
	}

	startTime := request.GetStartTime()
	if startTime != nil {
		ts := time.Unix(startTime.GetSeconds(), int64(startTime.GetNanos()))
		query.StartTime = &ts
	}
	endTime := request.GetEndTime()
	if endTime != nil {
		ts := time.Unix(endTime.GetSeconds(), int64(endTime.GetNanos()))
		query.EndTime = &ts
	}

	trace, err := h.QueryService.GetTrace(stream.Context(), query)
	if err != nil {
		return fmt.Errorf("cannot retrieve trace: %w", err)
	}
	td, err := modelToOTLP(trace.GetSpans())
	if err != nil {
		return err
	}
	tracesData := api_v3.TracesData(td)
	return stream.Send(&tracesData)
}

// FindTraces implements api_v3.QueryServiceServer's FindTraces
func (h *Handler) FindTraces(request *api_v3.FindTracesRequest, stream api_v3.QueryService_FindTracesServer) error {
	query := request.GetQuery()
	if query == nil {
		return status.Error(codes.InvalidArgument, "missing query")
	}
	if query.GetStartTimeMin() == nil ||
		query.GetStartTimeMax() == nil {
		return errors.New("start time min and max are required parameters")
	}

	queryParams := &spanstore.TraceQueryParameters{
		ServiceName:   query.GetServiceName(),
		OperationName: query.GetOperationName(),
		Tags:          query.GetAttributes(),
		NumTraces:     int(query.GetNumTraces()),
	}
	if query.GetStartTimeMin() != nil {
		startTimeMin, err := types.TimestampFromProto(query.GetStartTimeMin())
		if err != nil {
			return err
		}
		queryParams.StartTimeMin = startTimeMin
	}
	if query.GetStartTimeMax() != nil {
		startTimeMax, err := types.TimestampFromProto(query.GetStartTimeMax())
		if err != nil {
			return err
		}
		queryParams.StartTimeMax = startTimeMax
	}
	if query.GetDurationMin() != nil {
		durationMin, err := types.DurationFromProto(query.GetDurationMin())
		if err != nil {
			return err
		}
		queryParams.DurationMin = durationMin
	}
	if query.GetDurationMax() != nil {
		durationMax, err := types.DurationFromProto(query.GetDurationMax())
		if err != nil {
			return err
		}
		queryParams.DurationMax = durationMax
	}

	traces, err := h.QueryService.FindTraces(stream.Context(), queryParams)
	if err != nil {
		return err
	}
	for _, t := range traces {
		td, err := modelToOTLP(t.GetSpans())
		if err != nil {
			return err
		}
		tracesData := api_v3.TracesData(td)
		stream.Send(&tracesData)
	}
	return nil
}

// GetServices implements api_v3.QueryServiceServer's GetServices
func (h *Handler) GetServices(ctx context.Context, _ *api_v3.GetServicesRequest) (*api_v3.GetServicesResponse, error) {
	services, err := h.QueryService.GetServices(ctx)
	if err != nil {
		return nil, err
	}
	return &api_v3.GetServicesResponse{
		Services: services,
	}, nil
}

// GetOperations implements api_v3.QueryService's GetOperations
func (h *Handler) GetOperations(ctx context.Context, request *api_v3.GetOperationsRequest) (*api_v3.GetOperationsResponse, error) {
	operations, err := h.QueryService.GetOperations(ctx, spanstore.OperationQueryParameters{
		ServiceName: request.GetService(),
		SpanKind:    request.GetSpanKind(),
	})
	if err != nil {
		return nil, err
	}
	apiOperations := make([]*api_v3.Operation, len(operations))
	for i := range operations {
		apiOperations[i] = &api_v3.Operation{
			Name:     operations[i].Name,
			SpanKind: operations[i].SpanKind,
		}
	}
	return &api_v3.GetOperationsResponse{
		Operations: apiOperations,
	}, nil
}
