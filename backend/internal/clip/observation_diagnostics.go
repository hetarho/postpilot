package clip

import "math"

// Geometry is recorded as bounded millionths, never provider text.
func observationGeometry(focal Point, subject Region) map[string]int {
	values := map[string]int{}
	for key, value := range map[string]float64{"focal_x_ppm": focal.X, "focal_y_ppm": focal.Y, "subject_x_ppm": subject.X, "subject_y_ppm": subject.Y, "subject_width_ppm": subject.Width, "subject_height_ppm": subject.Height} {
		if !math.IsNaN(value) && !math.IsInf(value, 0) && math.Abs(value) <= 180 {
			values[key] = int(math.Round(value * 1000000))
		}
	}
	return values
}

func observationViolation(check string, segment int, values map[string]int) error {
	values = SafeAttemptValues(values)
	if segment > 0 {
		values["segment"] = segment
	}
	return WithAttemptDiagnostic(ErrInvalid, AttemptDiagnostic{Check: check, Phase: "observation", Values: values})
}
