package themes

import "testing"

func TestSanitize(t *testing.T) {
	if _, e := Sanitize(map[string]string{"bg": "#112233", "radius": "20px"}); e != nil {
		t.Fatal(e)
	}
	if _, e := Sanitize(map[string]string{"evil": "body{}"}); e == nil {
		t.Fatal("expected rejection")
	}
	if _, e := Sanitize(map[string]string{"background_image": "file:///etc/passwd"}); e == nil {
		t.Fatal("expected URL rejection")
	}
}
func TestPresets(t *testing.T) {
	if len(Presets()) < 6 {
		t.Fatal("missing presets")
	}
	for _, p := range Presets() {
		if _, e := Sanitize(p.Values); e != nil {
			t.Fatal(e)
		}
	}
}
