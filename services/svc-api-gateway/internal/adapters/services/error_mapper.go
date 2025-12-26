package services

import (
	"strings"

	"github.com/architeacher/devices/services/svc-api-gateway/internal/domain/model"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func mapGRPCError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.NotFound:
		return model.ErrDeviceNotFound

	case codes.FailedPrecondition:
		msg := st.Message()
		if strings.Contains(msg, "cannot update") {
			return model.ErrCannotUpdateInUseDevice
		}
		if strings.Contains(msg, "cannot delete") {
			return model.ErrCannotDeleteInUseDevice
		}

		return err

	case codes.InvalidArgument:
		return &model.ValidationErrors{
			Errors: []model.ValidationError{
				{
					Message: st.Message(),
					Code:    "invalid_argument",
				},
			},
		}

	case codes.Unavailable:
		return model.ErrServiceUnavailable

	case codes.DeadlineExceeded:
		return model.ErrTimeout

	default:
		return err
	}
}
