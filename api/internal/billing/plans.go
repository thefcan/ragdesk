// Package billing defines subscription plans and their usage limits, and the
// payment-provider abstraction (Stripe in production, a no-op in the local/$0
// path). Plan enforcement and metering live in the store and HTTP handlers.
package billing

// Plan identifiers, stored verbatim in workspaces.plan.
const (
	PlanFree = "free"
	PlanPro  = "pro"
)

// Plan is a subscription tier and its monthly limits. A negative limit means
// unlimited.
type Plan struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	PriceCents      int    `json:"price_cents"`
	MaxDocuments    int    `json:"max_documents"`
	MaxChatPerMonth int    `json:"max_chat_per_month"`
}

var plans = map[string]Plan{
	PlanFree: {ID: PlanFree, Name: "Free", PriceCents: 0, MaxDocuments: 25, MaxChatPerMonth: 100},
	PlanPro:  {ID: PlanPro, Name: "Pro", PriceCents: 2900, MaxDocuments: 1000, MaxChatPerMonth: 5000},
}

// PlanByID returns the plan for id, falling back to Free for an unknown value so
// a stale or corrupt plan never grants unlimited access.
func PlanByID(id string) Plan {
	if p, ok := plans[id]; ok {
		return p
	}
	return plans[PlanFree]
}

// Catalog returns the plans in display order (Free first).
func Catalog() []Plan {
	return []Plan{plans[PlanFree], plans[PlanPro]}
}
