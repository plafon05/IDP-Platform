package notification

import "testing"

func TestUnsubscribeTokenRoundTrip(t *testing.T) {
	secret := []byte("test-secret")
	token := signUnsubscribeToken("user-id", secret)
	userID, err := verifyUnsubscribeToken(token, secret)
	if err != nil || userID != "user-id" {
		t.Fatalf("token verification failed: user=%q err=%v", userID, err)
	}
	if _, err := verifyUnsubscribeToken(token+"x", secret); err == nil {
		t.Fatal("modified token must be rejected")
	}
}

func TestPreferenceColumn(t *testing.T) {
	if preferenceColumn(CommentCreatedTemplate) != "comments" || preferenceColumn(PasswordResetTemplate) != "" {
		t.Fatal("notification templates mapped to incorrect preferences")
	}
}
