// Package markdown renders Markdown email content for contrib email workflows.
//
// Use it when an application wants Markdown-to-HTML rendering before sending
// through a ports.Sender implementation. Provider delivery remains owned by a
// separate adapter such as contrib/adapters/resend.
package markdown
