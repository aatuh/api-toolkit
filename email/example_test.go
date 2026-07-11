package email_test

import (
	"fmt"

	"github.com/aatuh/api-toolkit/v4/email"
)

func ExampleMessage() {
	message := email.Message{
		From:    "support@example.test",
		To:      []string{"user@example.test"},
		Subject: "Welcome",
		Text:    "Thanks for signing up.",
	}

	fmt.Println(message.To[0])
	fmt.Println(message.Subject)

	// Output:
	// user@example.test
	// Welcome
}
