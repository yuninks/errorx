package errorx

import (
	"encoding/json"
	"errors"

	"google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const defaultCode = 400

type Errorx struct {
	list []CodeError
}

type CodeError struct {
	*status.Status
	// Code    int    `json:"code"`
	// Message string `json:"message"`
}

func NewCodeError(code int, msg string) error {
	return &Errorx{
		list: []CodeError{
			{
				status.New(codes.Code(code), msg),
				// Code:    code,
				// Message: msg,
			},
		},
	}
}

func New(msg string) error {
	return NewCodeError(defaultCode, msg)
}

func WithError(err error, msg string) error {
	if err == nil {
		return NewCodeError(defaultCode, msg)
	}
	e, ok := err.(*Errorx)
	if ok {
		e.list = append(e.list, CodeError{
			status.New(codes.Code(defaultCode), msg),
			// Code:    defaultCode,
			// Message: msg,
		})
		return e
	} else {
		return &Errorx{
			list: []CodeError{
				{
					status.New(codes.Code(defaultCode), msg),
					// Code:    defaultCode,
					// Message: err.Error(),
				},
				{
					status.New(codes.Code(defaultCode), msg),
					// Code:    defaultCode,
					// Message: msg,
				},
			},
		}
	}
}

func (e *Errorx) Error() string {
	c := len(e.list)
	if c == 0 {
		return ""
	}

	return e.list[c-1].Message()
}

func (e *Errorx) Code() int {
	c := len(e.list)
	if c == 0 {
		return 200
	}
	return int(e.list[c-1].Code())
}

func FromError(err error) (s *status.Status, ok bool) {
	if err == nil {
		return nil, true
	}
	type grpcstatus interface{ GRPCStatus() *status.Status }
	if gs, ok := err.(grpcstatus); ok {
		if gs.GRPCStatus() == nil {
			// Error has status nil, which maps to codes.OK. There
			// is no sensible behavior for this, so we turn it into
			// an error with codes.Unknown and discard the existing
			// status.
			return status.New(codes.Unknown, err.Error()), false
		}
		return gs.GRPCStatus(), true
	}
	var gs grpcstatus
	if errors.As(err, &gs) {
		if gs.GRPCStatus() == nil {
			// Error wraps an error that has status nil, which maps
			// to codes.OK.  There is no sensible behavior for this,
			// so we turn it into an error with codes.Unknown and
			// discard the existing status.
			return status.New(codes.Unknown, err.Error()), false
		}
		p := gs.GRPCStatus().Proto()
		p.Message = err.Error()
		return status.FromProto(p), true
	}
	return status.New(codes.Unknown, err.Error()), false
}

func (e *Errorx) GRPCStatus() *status.Status {
	c := len(e.list)
	if c == 0 {
		return nil
	}
	return e.list[c-1].Status
}
func (e *Errorx) Is(target error) bool {
	return true
}

func (e *Errorx) Data() []CodeError {
	return e.list
}

func (e *Errorx) String() string {
	b, _ := json.Marshal(e.list)
	return string(b)
}
