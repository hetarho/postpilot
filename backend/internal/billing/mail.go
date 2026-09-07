package billing

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/postpilot/backend/internal/plan"
)

type MailMessage struct {
	Subject string
	Text    string
}

// RenewalMail is Korean-first because the charged cards are Korean; the English copy
// follows in the same text-only message for accounts that use the English interface.
func RenewalMail(tier plan.Plan, term Term, quote Quote) MailMessage {
	detail := chargeDetail(tier, term, quote)
	return MailMessage{
		Subject: "Postpilot 구독이 갱신되었습니다 / Subscription renewed",
		Text: fmt.Sprintf(
			"Postpilot %s 구독이 갱신되었습니다.\n결제: %s\n\nYour Postpilot %s subscription was renewed.\nCharge: %s",
			tier, detail, tier, detail,
		),
	}
}

func RenewalFailedMail(tier plan.Plan, term Term, quote Quote) MailMessage {
	detail := chargeDetail(tier, term, quote)
	return MailMessage{
		Subject: "Postpilot 구독 갱신에 실패했습니다 / Renewal failed",
		Text: fmt.Sprintf(
			"Postpilot %s 구독의 갱신 결제에 실패해 플랜이 free로 변경되었습니다.\n시도한 결제: %s\n\nThe renewal charge for your Postpilot %s subscription failed, so your plan changed to free.\nAttempted charge: %s",
			tier, detail, tier, detail,
		),
	}
}

func CancellationMail(tier plan.Plan, term Term) MailMessage {
	return MailMessage{
		Subject: "Postpilot 구독이 종료되었습니다 / Subscription ended",
		Text: fmt.Sprintf(
			"Postpilot %s %s 구독의 예약된 취소가 적용되어 플랜이 free로 변경되었습니다.\n\nYour scheduled cancellation for the Postpilot %s %s subscription took effect, and your plan changed to free.",
			tier, term, tier, term,
		),
	}
}

func PurchaseMail(purchase Purchase) MailMessage {
	detail := purchaseDetail(purchase)
	return MailMessage{
		Subject: "Postpilot 크레딧 구매가 완료되었습니다 / Credit purchase complete",
		Text: fmt.Sprintf(
			"Postpilot 크레딧 %d개를 구매했습니다.\n결제: %s\n\nYou purchased %d Postpilot credits.\nCharge: %s",
			purchase.Credits, detail, purchase.Credits, detail,
		),
	}
}

func RefundMail(purchase Purchase) MailMessage {
	detail := purchaseDetail(purchase)
	return MailMessage{
		Subject: "Postpilot 크레딧 구매가 환불되었습니다 / Credit purchase refunded",
		Text: fmt.Sprintf(
			"Postpilot 크레딧 %d개 구매를 환불했습니다.\n환불: %s\n\nYour purchase of %d Postpilot credits was refunded.\nRefund: %s",
			purchase.Credits, detail, purchase.Credits, detail,
		),
	}
}

func purchaseDetail(purchase Purchase) string {
	return fmt.Sprintf(
		"%d credits · $%d.%02d · %s원 · %s원/$ (%s)",
		purchase.Credits, purchase.USDCents/100, purchase.USDCents%100,
		comma(int64(purchase.KRW)), rateString(purchase.RatePerUSDE4), purchase.RateDate,
	)
}

func chargeDetail(tier plan.Plan, term Term, quote Quote) string {
	return fmt.Sprintf(
		"%s %s · $%d.%02d · %s원 · %s원/$",
		tier, term, quote.USDCents/100, quote.USDCents%100,
		comma(int64(quote.KRW)), rateString(quote.RatePerUSDE4),
	)
}

func rateString(rateE4 int64) string {
	whole, fraction := rateE4/10_000, rateE4%10_000
	return fmt.Sprintf("%s.%02d", comma(whole), fraction/100)
}

func comma(value int64) string {
	raw := strconv.FormatInt(value, 10)
	var out strings.Builder
	for index, char := range raw {
		if index > 0 && (len(raw)-index)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(char)
	}
	return out.String()
}
