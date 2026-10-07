package design

// CaptionEffectPaint exports the native painters' fixed effect palette. Time
// state remains executable product math; neither palette nor shader comes from AI.
func CaptionEffectPaint() map[string]any {
	return map[string]any{
		"version": "caption-filters-v1-ember-v1",
		"ember":   map[string]any{"wide": "#FF5A1F", "tight": "#FFB443", "spark": "#FFC978", "stops": []map[string]any{{"at": 0, "hex": "#FFF6DC", "alpha": 1}, {"at": 0.28, "hex": "#FFB443", "alpha": 1}, {"at": 0.62, "hex": "#FF6A2B", "alpha": 0.85}, {"at": 1, "hex": "#E0431B", "alpha": 0}}},
		"ambient": map[string]any{
			"a": []map[string]any{{"at": 0, "hex": "#8B5CF6", "alpha": 0.95}, {"at": 0.55, "hex": "#6D3BF5", "alpha": 0.42}, {"at": 1, "hex": "#4C1D95", "alpha": 0}},
			"b": []map[string]any{{"at": 0, "hex": "#4AD9E8", "alpha": 0.9}, {"at": 0.5, "hex": "#22B8CF", "alpha": 0.34}, {"at": 1, "hex": "#0E7490", "alpha": 0}},
		},
		"neon": map[string]any{"wide": "#22D3EE", "tight": "#7DF9FF"},
		"iridescent": map[string]any{
			"glow":  "#7C3AED",
			"stops": []map[string]any{{"at": -0.45, "hex": "#5EEAD4"}, {"at": -0.18, "hex": "#818CF8"}, {"at": 0.1, "hex": "#F472B6"}, {"at": 0.38, "hex": "#FDBA74"}, {"at": 0.66, "hex": "#5EEAD4"}, {"at": 0.95, "hex": "#818CF8"}},
		},
		"glitch": map[string]any{"red": "#FF2D55", "cyan": "#00E5FF"},
	}
}
