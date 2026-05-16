package adapters

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/duclm99/bookstore-backend-v2/internal/modules/identity/app/ports"
	event "github.com/duclm99/bookstore-backend-v2/internal/modules/identity/domain/event"
	util "github.com/duclm99/bookstore-backend-v2/internal/platform/idempotency"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/kafka"
	"github.com/duclm99/bookstore-backend-v2/internal/platform/outbox"
	"go.uber.org/zap"
)

type OutboxEventPublisher struct {
	recorder outbox.Recorder
	log      *zap.Logger
}

func NewOutboxEventPublisher(recorder outbox.Recorder, log *zap.Logger) ports.EventPublisher {
	return &OutboxEventPublisher{
		recorder: recorder,
		log:      log,
	}
}

func (p *OutboxEventPublisher) PublishUserRegistered(ctx context.Context, payload ports.UserRegisteredPayload) error {
	p.log.Info("event: UserRegistered (Outbox)",
		zap.Int64("user_id", payload.UserID),
		zap.String("email", payload.Email),
	)

	if payload.Token == "" {
		return fmt.Errorf("UserRegistered event: verification token is empty")
	}

	aggIDStr := strconv.FormatInt(payload.UserID, 10)

	// Prepare the payload to be published
	eventPayload := map[string]any{
		"user_id": payload.UserID,
		"email":   payload.Email,
		"token":   payload.Token,
	}

	request_id := util.GetRequestIdKey(ctx)
	idempotency_key := util.GetIdempotencyKey(ctx)
	// Use outbox recorder to record the event within the current transaction
	err := p.recorder.Record(ctx, outbox.RecordParams{
		Topic:          string(kafka.TopicNotificationCmds), // Assuming notification commands handle email sending
		EventKey:       kafka.EventKey(kafka.AggUser, aggIDStr),
		AggregateType:  string(kafka.AggUser),
		AggregateID:    &aggIDStr,
		EventType:      event.RegisterEvent, // or whatever specific event type
		OccurredAt:     time.Now(),
		SchemaVersion:  1,
		Payload:        eventPayload,
		TraceID:        request_id,
		CorrelationID:  request_id,
		CausationID:    request_id,
		IdempotencyKey: idempotency_key,
	})

	if err != nil {
		return fmt.Errorf("outbox recorder failed: %w", err)
	}

	return nil
}

func (p *OutboxEventPublisher) PublishResetPasswordCompleted(ctx context.Context, payload ports.ResetPasswordCompletedPayload) error {
	p.log.Info("event: ResetPasswordCompleted (Outbox)",
		zap.Int64("user_id", payload.UserID),
		zap.String("email", payload.Email),
	)

	aggIDStr := strconv.FormatInt(payload.UserID, 10)
	eventPayload := map[string]any{
		"user_id": payload.UserID,
		"email":   payload.Email,
	}

	requestID := util.GetRequestIdKey(ctx)
	err := p.recorder.Record(ctx, outbox.RecordParams{
		Topic:         string(kafka.TopicNotificationCmds),
		EventKey:      kafka.EventKey(kafka.AggUser, aggIDStr),
		AggregateType: string(kafka.AggUser),
		AggregateID:   &aggIDStr,
		EventType:     event.ResetPasswordCompletedEvent,
		OccurredAt:    time.Now(),
		SchemaVersion: 1,
		Payload:       eventPayload,
		TraceID:       requestID,
		CorrelationID: requestID,
		CausationID:   requestID,
	})
	if err != nil {
		return fmt.Errorf("outbox recorder failed: %w", err)
	}
	return nil
}

func (p *OutboxEventPublisher) PublishResetPasswordRequested(ctx context.Context, payload ports.ResetPasswordRequestedPayload) error {
	p.log.Info("event: ResetPasswordRequested (Outbox)",
		zap.Int64("user_id", payload.UserID),
		zap.String("email", payload.Email),
	)

	if payload.Token == "" {
		return fmt.Errorf("ResetPasswordRequested event: reset token is empty")
	}

	aggIDStr := payload.Email

	// Prepare the payload to be published
	eventPayload := map[string]any{
		"user_id": payload.UserID,
		"email":   payload.Email,
		"token":   payload.Token,
	}

	request_id := util.GetRequestIdKey(ctx)
	idempotency_key := util.GetIdempotencyKey(ctx)
	// Use outbox recorder to record the event within the current transaction
	err := p.recorder.Record(ctx, outbox.RecordParams{
		Topic:          string(kafka.TopicNotificationCmds), // Assuming notification commands handle email sending
		EventKey:       kafka.EventKey(kafka.AggUser, aggIDStr),
		AggregateType:  string(kafka.AggUser),
		AggregateID:    &aggIDStr,
		EventType:      event.ResetPasswordRequestedEvent, // or whatever specific event type
		OccurredAt:     time.Now(),
		SchemaVersion:  1,
		Payload:        eventPayload,
		TraceID:        request_id,
		CorrelationID:  request_id,
		CausationID:    request_id,
		IdempotencyKey: idempotency_key,
	})
	if err != nil {
		return fmt.Errorf("outbox recorder failed: %w", err)
	}

	return nil

}
