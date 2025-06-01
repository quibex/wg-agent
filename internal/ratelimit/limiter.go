package ratelimit

import (
	"context"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Limiter rate limiter для gRPC
type Limiter struct {
	limiter *rate.Limiter
}

// NewLimiter создает новый rate limiter
// limit - количество запросов в секунду
func NewLimiter(limit int) *Limiter {
	return &Limiter{
		limiter: rate.NewLimiter(rate.Limit(limit), limit),
	}
}

// UnaryInterceptor возвращает gRPC interceptor для rate limiting
func (l *Limiter) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !l.limiter.Allow() {
			return nil, status.Error(codes.ResourceExhausted, "rate limit exceeded")
		}
		return handler(ctx, req)
	}
}

// Wait ждет разрешения на выполнение запроса
func (l *Limiter) Wait(ctx context.Context) error {
	return l.limiter.Wait(ctx)
}

// Allow проверяет, можно ли выполнить запрос сейчас
func (l *Limiter) Allow() bool {
	return l.limiter.Allow()
}
