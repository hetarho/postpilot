package clip

import "context"

type correctionObserverKey struct{}
type ResponseCorrectionObserver func(int, int, AttemptDiagnostic) error

func WithResponseCorrectionObserver(ctx context.Context, fn ResponseCorrectionObserver) context.Context {
	return context.WithValue(ctx, correctionObserverKey{}, fn)
}
func ReportResponseCorrection(ctx context.Context, attempt, limit int, d AttemptDiagnostic) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if fn, ok := ctx.Value(correctionObserverKey{}).(ResponseCorrectionObserver); ok {
		return fn(attempt, limit, d)
	}
	return nil
}
