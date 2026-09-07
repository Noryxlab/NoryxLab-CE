package handlers

import "testing"

// Leaving the date blank was free, so it was always left blank, and a
// repository stopped cloning one morning with nothing anywhere having said it
// would.
func TestATokenHasToAnswerTheLifetimeQuestion(t *testing.T) {
	for _, kind := range []string{"pat", "prat", "PERSAT", "token"} {
		if err := requireTokenExpiry(upsertSecretRequest{Type: kind}); err == nil {
			t.Errorf("%s with no date and no answer should be refused", kind)
		}
		if err := requireTokenExpiry(upsertSecretRequest{Type: kind, ExpiresAt: "2027-01-01"}); err != nil {
			t.Errorf("%s with a date should be accepted: %v", kind, err)
		}
		// A classic GitHub token really can be perpetual, and refusing to
		// record that would push people to invent a date they do not believe.
		if err := requireTokenExpiry(upsertSecretRequest{Type: kind, NeverExpires: true}); err != nil {
			t.Errorf("%s declared perpetual should be accepted: %v", kind, err)
		}
	}
}

func TestAnyOtherSecretIsNotAskedForADate(t *testing.T) {
	if err := requireTokenExpiry(upsertSecretRequest{Type: "generic"}); err != nil {
		t.Errorf("a plain value needs no expiry: %v", err)
	}
	if err := requireTokenExpiry(upsertSecretRequest{}); err != nil {
		t.Errorf("an unspecified kind needs no expiry: %v", err)
	}
}
