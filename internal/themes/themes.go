package themes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type Theme struct {
	Name   string            `json:"name"`
	Values map[string]string `json:"values"`
}

var allowed = map[string]func(string) (string, bool){
	"bg": color, "surface": color, "text": color, "muted": color, "accent": color, "border": color,
	"radius": px(0, 48), "gap": px(4, 48), "padding": px(8, 40), "blur": px(0, 32),
	"font": font, "nav": enum("top", "side", "minimal", "hidden"), "background_image": background,
	"background_size": enum("cover", "contain", "auto"), "background_position": position,
	"overlay": decimal(0, 0.9), "card_opacity": decimal(0.15, 1), "shadow": enum("none", "soft", "strong"),
}

func color(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if len(v) == 4 || len(v) == 7 || len(v) == 9 {
		if strings.HasPrefix(v, "#") {
			for _, c := range v[1:] {
				if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
					return "", false
				}
			}
			return v, true
		}
	}
	return "", false
}
func px(lo, hi int) func(string) (string, bool) {
	return func(v string) (string, bool) {
		n, e := strconv.Atoi(strings.TrimSuffix(strings.TrimSpace(v), "px"))
		if e != nil || n < lo || n > hi {
			return "", false
		}
		return strconv.Itoa(n) + "px", true
	}
}
func decimal(lo, hi float64) func(string) (string, bool) {
	return func(v string) (string, bool) {
		n, e := strconv.ParseFloat(v, 64)
		if e != nil || n < lo || n > hi {
			return "", false
		}
		return strconv.FormatFloat(n, 'f', 2, 64), true
	}
}
func enum(xs ...string) func(string) (string, bool) {
	return func(v string) (string, bool) {
		for _, x := range xs {
			if v == x {
				return v, true
			}
		}
		return "", false
	}
}
func font(v string) (string, bool) {
	v = strings.TrimSpace(v)
	switch v {
	case "system", "serif", "mono":
		return v, true
	}
	return "", false
}
func background(v string) (string, bool) {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(v, "/uploads/") || strings.HasPrefix(v, "https://") {
		if len(v) <= 500 {
			return v, true
		}
	}
	return "", false
}
func position(v string) (string, bool) {
	switch v {
	case "center", "top", "bottom", "left", "right":
		return v, true
	}
	return "", false
}
func Sanitize(in map[string]string) (map[string]string, error) {
	out := map[string]string{}
	for k, v := range in {
		f, ok := allowed[k]
		if !ok {
			return nil, fmt.Errorf("unknown theme property %q", k)
		}
		x, ok := f(v)
		if !ok {
			return nil, fmt.Errorf("invalid value for %q", k)
		}
		out[k] = x
	}
	return out, nil
}
func Presets() map[string]Theme {
	return map[string]Theme{
		"sidinet-dark":  {"SIDINET Dark", map[string]string{"bg": "#0d1117", "surface": "#161b22", "text": "#f0f6fc", "muted": "#8b949e", "accent": "#58a6ff", "border": "#30363d", "radius": "18px", "gap": "14px", "padding": "18px", "blur": "0px", "font": "system", "nav": "top", "card_opacity": "1.00", "shadow": "soft"}},
		"sidinet-light": {"SIDINET Light", map[string]string{"bg": "#f6f8fa", "surface": "#ffffff", "text": "#1f2328", "muted": "#656d76", "accent": "#0969da", "border": "#d0d7de", "radius": "16px", "gap": "14px", "padding": "18px", "blur": "0px", "font": "system", "nav": "top", "card_opacity": "1.00", "shadow": "soft"}},
		"minimal":       {"Minimal", map[string]string{"bg": "#111111", "surface": "#171717", "text": "#f5f5f5", "muted": "#999999", "accent": "#dddddd", "border": "#252525", "radius": "8px", "gap": "10px", "padding": "16px", "blur": "0px", "font": "system", "nav": "minimal", "card_opacity": "1.00", "shadow": "none"}},
		"glass":         {"Glass", map[string]string{"bg": "#101827", "surface": "#1b2940", "text": "#ffffff", "muted": "#b8c2d1", "accent": "#70b7ff", "border": "#52647a", "radius": "22px", "gap": "16px", "padding": "18px", "blur": "16px", "font": "system", "nav": "top", "card_opacity": "0.62", "shadow": "soft"}},
		"transparent":   {"Transparent", map[string]string{"bg": "#101418", "surface": "#101418", "text": "#ffffff", "muted": "#b0b7c0", "accent": "#75baff", "border": "#5b6570", "radius": "16px", "gap": "14px", "padding": "18px", "blur": "0px", "font": "system", "nav": "top", "card_opacity": "0.30", "shadow": "none"}},
		"oled":          {"OLED", map[string]string{"bg": "#000000", "surface": "#050505", "text": "#ffffff", "muted": "#aaaaaa", "accent": "#67b7ff", "border": "#202020", "radius": "14px", "gap": "12px", "padding": "16px", "blur": "0px", "font": "system", "nav": "top", "card_opacity": "1.00", "shadow": "none"}},
	}
}
func Encode(t Theme) string { b, _ := json.Marshal(t); return string(b) }
