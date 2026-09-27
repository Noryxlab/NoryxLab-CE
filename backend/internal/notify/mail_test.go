package notify

import "testing"

// A mailbox is a destination in its own right.
//
// EMSE recorded alert conditions for weeks and told nobody, because the only
// transport was a webhook and the site runs no receiver for one. What it does
// have is a mail path that already works - the bridge relaying over HTTPS,
// built because every outbound SMTP port there is closed.
func TestAMailboxAloneEnablesAlerting(t *testing.T) {
	notifier := NewDynamic(func() (string, string) { return "", "emse" }).
		WithMail(func() MailDestination {
			return MailDestination{Host: "noryx-mail-bridge", Port: "1025", From: "support@noryxlab.ai", To: "ops@example.org"}
		})
	if !notifier.Enabled() {
		t.Fatal("a configured mailbox must enable alerting on its own")
	}
}

// And neither destination is silence rather than a failure: an installation
// that configured nothing must not have its operations broken by the alerting.
func TestNeitherDestinationIsSilence(t *testing.T) {
	notifier := NewDynamic(func() (string, string) { return "", "" })
	if notifier.Enabled() {
		t.Fatal("no destination must not report itself as enabled")
	}
	if (MailDestination{Host: "relay", From: "a@b"}).usable() {
		t.Fatal("a destination with no recipient is not usable")
	}
	if (MailDestination{Host: "relay", To: "a@b"}).usable() {
		t.Fatal("a destination with no sender is not usable")
	}
}
