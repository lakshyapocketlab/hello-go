package hello

import (
	"context"
	"net/http"
)

// PhoneNumbersService lists the WhatsApp numbers connected to the organization.
type PhoneNumbersService struct{ c *Client }

// PhoneNumber is a connected WhatsApp number (GET /v1/phone_numbers).
type PhoneNumber struct {
	// PhoneNumberID is what Messages.Send and Media.Upload take.
	PhoneNumberID      string `json:"phone_number_id"`
	DisplayPhoneNumber string `json:"display_phone_number"`
	VerifiedName       string `json:"verified_name,omitempty"`
	// WabaID is what the Templates methods take.
	WabaID             string `json:"waba_id"`
	BusinessName       string `json:"business_name,omitempty"`
	Status             string `json:"status"`
	CanSend            bool   `json:"can_send"`
	QualityRating      string `json:"quality_rating,omitempty"`
	MessagingLimitTier string `json:"messaging_limit_tier,omitempty"`
	NameStatus         string `json:"name_status,omitempty"`
	IsCoexistence      bool   `json:"is_coexistence"`
}

// List returns the organization's numbers, newest first (GET /v1/phone_numbers).
func (s *PhoneNumbersService) List(ctx context.Context) ([]PhoneNumber, error) {
	var out struct {
		Data []PhoneNumber `json:"data"`
	}
	if err := s.c.do(ctx, request{method: http.MethodGet, path: "/v1/phone_numbers", retry: true}, &out); err != nil {
		return nil, err
	}
	return out.Data, nil
}

// WABAPhoneNumber is a number in Meta's Graph API shape, as returned by ListForWABA.
type WABAPhoneNumber struct {
	ID                 string `json:"id"`
	DisplayPhoneNumber string `json:"display_phone_number,omitempty"`
	VerifiedName       string `json:"verified_name,omitempty"`
	QualityRating      string `json:"quality_rating,omitempty"`
	// Raw holds every field of the response item.
	Raw map[string]any `json:"-"`
}

// ListForWABA returns the numbers of one WhatsApp Business Account in Meta's
// Graph API shape (GET /v1/{waba_id}/phone_numbers).
func (s *PhoneNumbersService) ListForWABA(ctx context.Context, wabaID string) ([]WABAPhoneNumber, error) {
	id, err := pathSegment("wabaID", wabaID)
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []map[string]any `json:"data"`
	}
	if err := s.c.do(ctx, request{method: http.MethodGet, path: "/v1/" + id + "/phone_numbers", retry: true}, &out); err != nil {
		return nil, err
	}
	nums := make([]WABAPhoneNumber, len(out.Data))
	for i, m := range out.Data {
		nums[i] = WABAPhoneNumber{
			ID: str(m, "id"), DisplayPhoneNumber: str(m, "display_phone_number"),
			VerifiedName: str(m, "verified_name"), QualityRating: str(m, "quality_rating"), Raw: m,
		}
	}
	return nums, nil
}
