package awsapi

import (
	"context"
	"errors"
	"fmt"
	"strings"

	awsmw "github.com/aws/aws-sdk-go-v2/aws/middleware"

	"github.com/aws/smithy-go/middleware"
)

var ErrNotReadOnly = errors.New("zombie-scanner is read-only")

// AssumeRole only fetches credentials for --accounts; it creates nothing
var allowedWrites = map[string]bool{"STS:AssumeRole": true}

func ReadOnly(stack *middleware.Stack) error {
	return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("ReadOnlyGuard",
		func(ctx context.Context, in middleware.InitializeInput, next middleware.InitializeHandler) (
			middleware.InitializeOutput, middleware.Metadata, error) {
			service, op := awsmw.GetServiceID(ctx), middleware.GetOperationName(ctx)
			if !isRead(op) && !allowedWrites[service+":"+op] {
				return middleware.InitializeOutput{}, middleware.Metadata{},
					fmt.Errorf("%w: refused %s:%s", ErrNotReadOnly, strings.ToLower(service), op)
			}
			return next.HandleInitialize(ctx, in)
		}), middleware.Before)
}

func isRead(op string) bool {
	for _, p := range []string{"Describe", "Get", "List"} {
		if strings.HasPrefix(op, p) {
			return true
		}
	}
	return false
}
