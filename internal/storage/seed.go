package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/core"
)

// MaybeSeedDemoData seeds sample events if KALYX_SEED_DEMO=true,
// or when ListEvents returns 0 and KALYX_DEV_MODE=true.
func MaybeSeedDemoData(ctx context.Context, store Storage) error {
	seedEnv := strings.EqualFold(os.Getenv("KALYX_SEED_DEMO"), "true")
	devMode := strings.EqualFold(os.Getenv("KALYX_DEV_MODE"), "true")

	if seedEnv {
		return SeedDemoData(ctx, store)
	}

	if devMode {
		events, err := store.ListEvents(ctx)
		if err == nil && len(events) == 0 {
			return SeedDemoData(ctx, store)
		}
	}

	return nil
}

// SeedDemoData inserts standard sample events (OrderCompleted, PaymentProcessed, UserRegistered)
// and their 1.0.0 and 1.2.0 contract versions into storage so users launching Kalyx have sample contracts available immediately.
func SeedDemoData(ctx context.Context, store Storage) error {
	now := time.Now().UTC()

	demoEvents := []struct {
		event    core.EventDefinition
		versions []core.EventVersion
	}{
		{
			event: core.EventDefinition{
				ID:          "orders.order.completed",
				Domain:      "Orders",
				Name:        "OrderCompleted",
				Type:        "orders.order.completed.v1",
				Description: "Emitted when a customer order completes payment authorization and inventory capture.",
				OwnerTeam:   "Checkout Core",
				CreatedAt:   now.AddDate(0, -6, 0),
				UpdatedAt:   now,
			},
			versions: []core.EventVersion{
				{
					ID:         "ver_orders_order_completed_1_0_0",
					EventID:    "orders.order.completed",
					SemVer:     "1.0.0",
					Status:     core.StatusDeprecated,
					CreatedAt:  now.AddDate(0, -6, 0),
					CreatedBy:  "alex.rivera@checkout.company.com",
					ApprovedBy: "sarah.chen@platform.company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -6, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_orders_completed",
						PartitionBy: []string{"order_id"},
						PIIFields:   []string{"customer_email"},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/orders.order.completed/v1.0.0.json",
  "title": "OrderCompleted",
  "type": "object",
  "properties": {
    "order_id": { "type": "string", "format": "uuid", "description": "Primary transaction reference" },
    "customer_email": { "type": "string", "format": "email", "description": "Customer contact email" },
    "amount_cents": { "type": "integer", "minimum": 0, "description": "Gross completed transaction amount in cents" },
    "currency": { "type": "string", "enum": ["USD", "EUR", "GBP"], "description": "ISO-4217 standard currency code" }
  },
  "required": ["order_id", "customer_email", "amount_cents", "currency"]
}`,
				},
				{
					ID:         "ver_orders_order_completed_1_2_0",
					EventID:    "orders.order.completed",
					SemVer:     "1.2.0",
					Status:     core.StatusApproved,
					CreatedAt:  now.AddDate(0, -1, 0),
					CreatedBy:  "alex.rivera@checkout.company.com",
					ApprovedBy: "sarah.chen@platform.company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -1, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_orders_completed",
						PartitionBy: []string{"order_id"},
						PIIFields:   []string{"customer_email"},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/orders.order.completed/v1.2.0.json",
  "title": "OrderCompleted",
  "type": "object",
  "properties": {
    "order_id": { "type": "string", "format": "uuid", "description": "Primary transaction reference" },
    "customer_id": { "type": "string", "format": "uuid", "description": "Foreign identifier linking to User Identity store" },
    "customer_email": { "type": "string", "format": "email", "description": "Customer notification email" },
    "amount_cents": { "type": "integer", "minimum": 0, "description": "Gross completed transaction amount in cents" },
    "currency": { "type": "string", "enum": ["USD", "EUR", "GBP"], "description": "ISO-4217 standard currency code" },
    "completed_at": { "type": "string", "format": "date-time", "description": "ISO-8601 UTC timestamp of payment settlement" }
  },
  "required": ["order_id", "customer_id", "customer_email", "amount_cents", "currency", "completed_at"]
}`,
				},
			},
		},
		{
			event: core.EventDefinition{
				ID:          "payments.payment.processed",
				Domain:      "Payments",
				Name:        "PaymentProcessed",
				Type:        "payments.payment.processed.v1",
				Description: "Emitted when Stripe/Adyen webhook acknowledges settled credit capture.",
				OwnerTeam:   "Billing Team",
				CreatedAt:   now.AddDate(0, -5, 0),
				UpdatedAt:   now,
			},
			versions: []core.EventVersion{
				{
					ID:         "ver_payments_payment_processed_1_0_0",
					EventID:    "payments.payment.processed",
					SemVer:     "1.0.0",
					Status:     core.StatusApproved,
					CreatedAt:  now.AddDate(0, -5, 0),
					CreatedBy:  "billing.dev@company.com",
					ApprovedBy: "marcus.vance@company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -5, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_payments_processed",
						PartitionBy: []string{"payment_id"},
						PIIFields:   []string{},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/payments.payment.processed/v1.0.0.json",
  "title": "PaymentProcessed",
  "type": "object",
  "properties": {
    "payment_id": { "type": "string", "format": "uuid", "description": "Unique transaction payment identifier" },
    "order_id": { "type": "string", "format": "uuid", "description": "Associated order ID" },
    "amount_cents": { "type": "integer", "minimum": 0, "description": "Captured amount in cents" },
    "currency": { "type": "string", "enum": ["USD", "EUR", "GBP"], "description": "ISO currency code" },
    "payment_method": { "type": "string", "description": "Payment instrument type" }
  },
  "required": ["payment_id", "order_id", "amount_cents", "currency", "payment_method"]
}`,
				},
				{
					ID:         "ver_payments_payment_processed_1_2_0",
					EventID:    "payments.payment.processed",
					SemVer:     "1.2.0",
					Status:     core.StatusApproved,
					CreatedAt:  now.AddDate(0, -2, 0),
					CreatedBy:  "billing.dev@company.com",
					ApprovedBy: "marcus.vance@company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -2, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_payments_processed",
						PartitionBy: []string{"payment_id"},
						PIIFields:   []string{},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/payments.payment.processed/v1.2.0.json",
  "title": "PaymentProcessed",
  "type": "object",
  "properties": {
    "payment_id": { "type": "string", "format": "uuid", "description": "Unique transaction payment identifier" },
    "order_id": { "type": "string", "format": "uuid", "description": "Associated order ID" },
    "amount_cents": { "type": "integer", "minimum": 0, "description": "Captured amount in cents" },
    "currency": { "type": "string", "enum": ["USD", "EUR", "GBP"], "description": "ISO currency code" },
    "payment_method": { "type": "string", "description": "Payment instrument type" },
    "fee_cents": { "type": "integer", "minimum": 0, "description": "Gateway processing fee" },
    "processed_at": { "type": "string", "format": "date-time", "description": "Settlement timestamp" }
  },
  "required": ["payment_id", "order_id", "amount_cents", "currency", "payment_method", "processed_at"]
}`,
				},
			},
		},
		{
			event: core.EventDefinition{
				ID:          "users.user.registered",
				Domain:      "Users",
				Name:        "UserRegistered",
				Type:        "users.user.registered.v1",
				Description: "Emitted upon successful OIDC onboarding or passwordless verification.",
				OwnerTeam:   "Identity IAM",
				CreatedAt:   now.AddDate(0, -4, 0),
				UpdatedAt:   now,
			},
			versions: []core.EventVersion{
				{
					ID:         "ver_users_user_registered_1_0_0",
					EventID:    "users.user.registered",
					SemVer:     "1.0.0",
					Status:     core.StatusApproved,
					CreatedAt:  now.AddDate(0, -4, 0),
					CreatedBy:  "iam.engineer@company.com",
					ApprovedBy: "marcus.vance@company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -4, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_users_registered",
						PartitionBy: []string{"user_id"},
						PIIFields:   []string{"email"},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/users.user.registered/v1.0.0.json",
  "title": "UserRegistered",
  "type": "object",
  "properties": {
    "user_id": { "type": "string", "format": "uuid", "description": "Internal unique user identifier" },
    "email": { "type": "string", "format": "email", "description": "Primary user email" }
  },
  "required": ["user_id", "email"]
}`,
				},
				{
					ID:         "ver_users_user_registered_1_2_0",
					EventID:    "users.user.registered",
					SemVer:     "1.2.0",
					Status:     core.StatusApproved,
					CreatedAt:  now.AddDate(0, -1, 0),
					CreatedBy:  "iam.engineer@company.com",
					ApprovedBy: "marcus.vance@company.com",
					ApprovedAt: ptrTime(now.AddDate(0, -1, 0)),
					DataLakeMetadata: core.DataLakeMetadata{
						TargetTable: "lake_users_registered",
						PartitionBy: []string{"user_id"},
						PIIFields:   []string{"email", "full_name"},
					},
					JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "$id": "https://schema.kalyx.io/events/users.user.registered/v1.2.0.json",
  "title": "UserRegistered",
  "type": "object",
  "properties": {
    "user_id": { "type": "string", "format": "uuid", "description": "Internal unique user identifier" },
    "email": { "type": "string", "format": "email", "description": "Primary user email" },
    "full_name": { "type": "string", "description": "User display name" },
    "locale": { "type": "string", "description": "User preferred locale" },
    "registered_at": { "type": "string", "format": "date-time", "description": "Registration timestamp" }
  },
  "required": ["user_id", "email", "registered_at"]
}`,
				},
			},
		},
	}

	for _, item := range demoEvents {
		// Create event if not present
		_, err := store.GetEvent(ctx, item.event.ID)
		if err != nil {
			if errors.Is(err, ErrEventNotFound) {
				if err := store.CreateEvent(ctx, item.event); err != nil && !errors.Is(err, ErrConflict) {
					return fmt.Errorf("failed to seed demo event %s: %w", item.event.ID, err)
				}
			} else {
				return fmt.Errorf("failed to check demo event %s: %w", item.event.ID, err)
			}
		}

		// Create versions if not present
		for _, v := range item.versions {
			_, err := store.GetEventVersion(ctx, v.EventID, v.SemVer)
			if err != nil {
				if errors.Is(err, ErrVersionNotFound) {
					if err := store.CreateEventVersion(ctx, v); err != nil && !errors.Is(err, ErrConflict) {
						return fmt.Errorf("failed to seed demo version %s for event %s: %w", v.SemVer, v.EventID, err)
					}
				} else {
					return fmt.Errorf("failed to check demo version %s for event %s: %w", v.SemVer, v.EventID, err)
				}
			}
		}
	}

	return nil
}

func ptrTime(t time.Time) *time.Time {
	return &t
}
