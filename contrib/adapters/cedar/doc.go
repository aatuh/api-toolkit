// Package cedar adapts cedar-go policy evaluation to api-toolkit authorization ports.
//
// Use New with a Cedar policy set when an application wants policy-backed
// authorization through authorization.NewPolicyAuthorizer. The adapter is in the
// contrib module and remains outside the stable core API promise.
package cedar
