import type { I18nFragment } from '@/shared/lib'

/** This slice's share of the `plans` namespace (ARCH-16). */
export const i18n = {
  namespace: 'plans',
  ko: {
    issueVoucher: {
      heading: '이용권 발급',
      contents: '내용',
      preset: '{{plan}} · {{credits}} 크레딧 · {{days}}일',
      custom: '직접 입력',
      credits: '크레딧',
      days: '사용 기간(일)',
      kind: '구분',
      sold: '판매',
      given: '선물',
      amount: '받은 금액(원)',
      payer: '입금자',
      message: '메시지',
      submit: '발급',
      issued: '이용권을 발급했어요. 링크를 복사해 보내 주세요.',
      error: {
        credits: '크레딧은 1에서 {{max}} 사이로 입력해 주세요.',
        days: '사용 기간은 1에서 {{max}}일 사이로 입력해 주세요.',
        amount: '받은 금액은 1원에서 {{max}}원 사이로 입력해 주세요.',
        payer: '입금자를 {{max}}자 이내로 입력해 주세요.',
        message: '메시지는 {{max}}자 이내로 입력해 주세요.',
      },
    },
  },
  en: {
    issueVoucher: {
      heading: 'Issue a voucher',
      contents: 'Contents',
      preset: '{{plan}} · {{credits}} credits · {{days}} days',
      custom: 'Custom',
      credits: 'Credits',
      days: 'Days of use',
      kind: 'Type',
      sold: 'Sold',
      given: 'Gift',
      amount: 'Amount received (KRW)',
      payer: 'Payer',
      message: 'Message',
      submit: 'Issue',
      issued: 'Voucher issued. Copy the link and send it.',
      error: {
        credits: 'Enter between 1 and {{max}} credits.',
        days: 'Enter between 1 and {{max}} days.',
        amount: 'Enter an amount between 1 and {{max}} KRW.',
        payer: 'Enter a payer name of up to {{max}} characters.',
        message: 'Keep the message within {{max}} characters.',
      },
    },
  },
} as const satisfies I18nFragment
