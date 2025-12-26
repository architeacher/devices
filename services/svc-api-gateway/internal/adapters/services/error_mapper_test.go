package services

import (
	"errors"
	"testing"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMapGRPCError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		inputErr    error
		expectedErr error
		checkType   bool
	}{
		{
			name:        "nil error returns nil",
			inputErr:    nil,
			expectedErr: nil,
			checkType:   false,
		},
		{
			name:        "NotFound maps to ErrDeviceNotFound",
			inputErr:    status.Error(codes.NotFound, "device not found"),
			expectedErr: model.ErrDeviceNotFound,
			checkType:   false,
		},
		{
			name:        "FailedPrecondition with update message",
			inputErr:    status.Error(codes.FailedPrecondition, "cannot update name or brand of in-use device"),
			expectedErr: model.ErrCannotUpdateInUseDevice,
			checkType:   false,
		},
		{
			name:        "FailedPrecondition with delete message",
			inputErr:    status.Error(codes.FailedPrecondition, "cannot delete in-use device"),
			expectedErr: model.ErrCannotDeleteInUseDevice,
			checkType:   false,
		},
		{
			name:        "FailedPrecondition with other message passes through",
			inputErr:    status.Error(codes.FailedPrecondition, "other precondition"),
			expectedErr: status.Error(codes.FailedPrecondition, "other precondition"),
			checkType:   false,
		},
		{
			name:        "InvalidArgument maps to ValidationErrors",
			inputErr:    status.Error(codes.InvalidArgument, "name is required"),
			expectedErr: nil,
			checkType:   true,
		},
		{
			name:        "Unavailable maps to ErrServiceUnavailable",
			inputErr:    status.Error(codes.Unavailable, "service unavailable"),
			expectedErr: model.ErrServiceUnavailable,
			checkType:   false,
		},
		{
			name:        "DeadlineExceeded maps to ErrTimeout",
			inputErr:    status.Error(codes.DeadlineExceeded, "context deadline exceeded"),
			expectedErr: model.ErrTimeout,
			checkType:   false,
		},
		{
			name:        "Other codes pass through",
			inputErr:    status.Error(codes.Internal, "internal error"),
			expectedErr: status.Error(codes.Internal, "internal error"),
			checkType:   false,
		},
		{
			name:        "Non-gRPC error passes through",
			inputErr:    errors.New("some error"),
			expectedErr: errors.New("some error"),
			checkType:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := mapGRPCError(tc.inputErr)

			if tc.checkType {
				var validationErr *model.ValidationErrors
				require.ErrorAs(t, result, &validationErr)

				return
			}

			if tc.expectedErr == nil {
				require.NoError(t, result)

				return
			}

			require.Error(t, result)

			if errors.Is(tc.expectedErr, model.ErrDeviceNotFound) ||
				errors.Is(tc.expectedErr, model.ErrCannotUpdateInUseDevice) ||
				errors.Is(tc.expectedErr, model.ErrCannotDeleteInUseDevice) ||
				errors.Is(tc.expectedErr, model.ErrServiceUnavailable) ||
				errors.Is(tc.expectedErr, model.ErrTimeout) {
				require.ErrorIs(t, result, tc.expectedErr)
			}
		})
	}
}
