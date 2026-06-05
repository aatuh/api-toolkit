package jwt_test

import (
	"context"
	"fmt"

	"github.com/aatuh/api-toolkit/v3/middleware/auth/jwt"
)

func ExampleWithSubject() {
	ctx := jwt.WithSubject(context.Background(), jwt.Subject{
		UserID: "user-1",
		Email:  "user@example.test",
	})

	subject, ok := jwt.SubjectFromContext(ctx)
	fmt.Println(ok)
	fmt.Println(subject.UserID)

	// Output:
	// true
	// user-1
}
