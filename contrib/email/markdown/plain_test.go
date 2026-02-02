package markdown

import "testing"

func TestHTMLToText(t *testing.T) {
	input := "<p>Hello</p><ul><li>Item one</li><li>Item two</li></ul><p>Go to <a href=\"https://example.com\">Example</a></p>"
	got, err := HTMLToText(input)
	if err != nil {
		t.Fatalf("HTMLToText error: %v", err)
	}
	want := "Hello\n\n- Item one\n- Item two\n\nGo to Example (https://example.com)"
	if got != want {
		t.Fatalf("unexpected output:\nwant: %q\ngot:  %q", want, got)
	}
}
