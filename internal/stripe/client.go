// Package stripe provides a thin wrapper around the Stripe SDK.
package stripe

import (
	"context"
	"fmt"
	"strconv"

	gostripe "github.com/stripe/stripe-go/v84"
	"github.com/stripe/stripe-go/v84/webhook"
)

// Config holds Stripe configuration.
type Config struct {
	SecretKey      string
	WebhookSecret  string
	PublishableKey string
}

// Client wraps the Stripe API.
type Client struct {
	config Config
	client *gostripe.Client
}

// New creates a new Stripe client and sets the global API key.
func New(cfg Config) *Client {
	stripeClient := gostripe.NewClient(cfg.SecretKey)
	return &Client{config: cfg, client: stripeClient}
}

// CreateCustomer creates a Stripe Customer.
func (c *Client) CreateCustomer(ctx context.Context, email, name string, metadata map[string]string) (*gostripe.Customer, error) {
	params := &gostripe.CustomerCreateParams{
		Email: gostripe.String(email),
		Name:  gostripe.String(name),
	}
	for k, v := range metadata {
		params.AddMetadata(k, v)
	}
	return c.client.V1Customers.Create(ctx, params)
}

// CreateSubscription creates a new Stripe Subscription.
func (c *Client) CreateSubscription(ctx context.Context, customerID, priceID string, seatCount int) (*gostripe.Subscription, error) {
	params := &gostripe.SubscriptionCreateParams{
		Customer: gostripe.String(customerID),
		Items: []*gostripe.SubscriptionCreateItemParams{
			{
				Price:    gostripe.String(priceID),
				Quantity: gostripe.Int64(int64(seatCount)),
			},
		},
	}
	return c.client.V1Subscriptions.Create(ctx, params)
}

// GetSubscription retrieves a Stripe subscription by ID.
func (c *Client) GetSubscription(ctx context.Context, subID string) (*gostripe.Subscription, error) {
	return c.client.V1Subscriptions.Retrieve(ctx, subID, nil)
}

// UpdateSubscription changes the price/seats on an existing subscription.
func (c *Client) UpdateSubscription(ctx context.Context, subID, newPriceID string, seatCount int) (*gostripe.Subscription, error) {
	// First retrieve the subscription to get the item ID.
	sub, err := c.client.V1Subscriptions.Retrieve(ctx, subID, nil)
	if err != nil {
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	if len(sub.Items.Data) == 0 {
		return nil, fmt.Errorf("subscription %s has no items", subID)
	}
	itemID := sub.Items.Data[0].ID

	params := &gostripe.SubscriptionUpdateParams{
		Items: []*gostripe.SubscriptionUpdateItemParams{
			{
				ID:       gostripe.String(itemID),
				Price:    gostripe.String(newPriceID),
				Quantity: gostripe.Int64(int64(seatCount)),
			},
		},
		ProrationBehavior: gostripe.String("create_prorations"),
	}
	return c.client.V1Subscriptions.Update(ctx, subID, params)
}

// CancelSubscription cancels a subscription, optionally at period end.
func (c *Client) CancelSubscription(ctx context.Context, subID string, atPeriodEnd bool) (*gostripe.Subscription, error) {
	if atPeriodEnd {
		params := &gostripe.SubscriptionUpdateParams{
			CancelAtPeriodEnd: gostripe.Bool(true),
		}
		return c.client.V1Subscriptions.Update(ctx, subID, params)
	}
	return c.client.V1Subscriptions.Cancel(ctx, subID, nil)
}

// ReportUsage reports metered usage via the Billing Meter Events API.
func (c *Client) ReportUsage(ctx context.Context, eventName, customerID string, quantity int64, timestamp int64) (*gostripe.BillingMeterEvent, error) {
	params := &gostripe.BillingMeterEventCreateParams{
		EventName: gostripe.String(eventName),
		Payload: map[string]string{
			"stripe_customer_id": customerID,
			"value":              strconv.FormatInt(quantity, 10),
		},
	}
	if timestamp > 0 {
		params.Timestamp = gostripe.Int64(timestamp)
	}
	return c.client.V1BillingMeterEvents.Create(ctx, params)
}

// CreatePortalSession creates a Stripe Billing Portal session for self-service management.
func (c *Client) CreatePortalSession(ctx context.Context, customerID, returnURL string) (*gostripe.BillingPortalSession, error) {
	params := &gostripe.BillingPortalSessionCreateParams{
		Customer:  gostripe.String(customerID),
		ReturnURL: gostripe.String(returnURL),
	}
	return c.client.V1BillingPortalSessions.Create(ctx, params)
}

// CreateCheckoutSession creates a Stripe Checkout session for initial subscription.
func (c *Client) CreateCheckoutSession(ctx context.Context, customerID, priceID, successURL, cancelURL string) (*gostripe.CheckoutSession, error) {
	params := &gostripe.CheckoutSessionCreateParams{
		Customer: gostripe.String(customerID),
		Mode:     gostripe.String(string(gostripe.CheckoutSessionModeSubscription)),
		LineItems: []*gostripe.CheckoutSessionCreateLineItemParams{
			{
				Price:    gostripe.String(priceID),
				Quantity: gostripe.Int64(1),
			},
		},
		SuccessURL: gostripe.String(successURL),
		CancelURL:  gostripe.String(cancelURL),
	}
	return c.client.V1CheckoutSessions.Create(ctx, params)
}

// VerifyWebhookSignature verifies a Stripe webhook payload signature and returns the event.
func (c *Client) VerifyWebhookSignature(payload []byte, signature string) (*gostripe.Event, error) {
	event, err := webhook.ConstructEvent(payload, signature, c.config.WebhookSecret)
	if err != nil {
		return nil, fmt.Errorf("webhook signature verification failed: %w", err)
	}
	return &event, nil
}

// PublishableKey returns the publishable key for frontend use.
func (c *Client) PublishableKey() string {
	return c.config.PublishableKey
}
