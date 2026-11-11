package destination

import (
	"net/url"
	"testing"
)

func TestMobileEncodedURLWithLiteralPercentRejected(t *testing.T) {
	input := "https://maps.google.com/maps?ftid=0xa:0xb&q=" + url.QueryEscape("50% https%3A%2F%2Fevil.test")
	_, _, err := Normalize(input)
	if err == nil {
		t.Fatal("encoded nested URL accepted")
	}
}

func TestMobileStructuralRejections(t *testing.T) {
	for _, raw := range []string{
		"q=91%2C2", "q=1%2C181", "q=1%2C%2C2", "q=1%2C", "q=%FF", "q=Rua%00X", "q=Rua%0AX",
		"q=Rua&q=Outra", "q=Rua&ftid=0xc:0xd", "q=Rua&query=Outra", "q=https%253A%252F%252Fevil.test", "q=%252F%252Fevil.test",
	} {
		t.Run(raw, func(t *testing.T) {
			_, warning, err := Normalize("https://maps.google.com/maps?ftid=0xa:0xb&" + raw)
			if err == nil && warning == "" {
				t.Fatal("unsafe input accepted")
			}
		})
	}
}

func TestMobileTextPreserved(t *testing.T) {
	for _, text := range []string{"123", "Avenida 50% Legal", "  Praça １２３, São João  ", "12, Loja 34"} {
		t.Run(text, func(t *testing.T) {
			input := "https://maps.google.com/maps?ftid=0xa:0xb&q=" + url.QueryEscape(text)
			got, warning, err := Normalize(input)
			if err != nil || warning != "" {
				t.Fatalf("Normalize: %v warning %q", err, warning)
			}
			u, err := url.Parse(got)
			if err != nil {
				t.Fatal(err)
			}
			if actual := u.Query().Get("q"); actual != text {
				t.Fatalf("got %q, want %q", actual, text)
			}
		})
	}
}
