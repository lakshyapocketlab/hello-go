package hello

import (
	"context"
	"testing"
)

func TestPhoneNumbersList(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"data":[{"phone_number_id":"106","display_phone_number":"+1 555","waba_id":"102","status":"active","can_send":true,"quality_rating":"GREEN","is_coexistence":true}]}`))
	got, err := c.PhoneNumbers.List(context.Background())
	if err != nil || len(got) != 1 || got[0].PhoneNumberID != "106" || got[0].WabaID != "102" || !got[0].CanSend || !got[0].IsCoexistence {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/phone_numbers")

	_, c = newFake(t, jsonReply(401, `{"message":"x"}`))
	if _, err := c.PhoneNumbers.List(context.Background()); !IsAuthentication(err) {
		t.Fatal(err)
	}
}

func TestPhoneNumbersListForWABA(t *testing.T) {
	f, c := newFake(t, jsonReply(200, `{"data":[{"id":"106","display_phone_number":"+1 555","verified_name":"Acme","quality_rating":"GREEN","extra":1}]}`))
	got, err := c.PhoneNumbers.ListForWABA(context.Background(), "10/2")
	if err != nil || len(got) != 1 || got[0].ID != "106" || got[0].VerifiedName != "Acme" || got[0].Raw["extra"] != float64(1) {
		t.Fatalf("%+v %v", got, err)
	}
	expect(t, f.last(), "GET", "/v1/10%2F2/phone_numbers")

	if _, err := c.PhoneNumbers.ListForWABA(context.Background(), ""); err == nil {
		t.Fatal("empty waba")
	}
	_, c = newFake(t, jsonReply(404, `{"message":"x"}`))
	if _, err := c.PhoneNumbers.ListForWABA(context.Background(), "1"); !IsNotFound(err) {
		t.Fatal(err)
	}
}
