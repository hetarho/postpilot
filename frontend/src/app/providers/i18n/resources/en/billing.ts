export const billing = {
  title: 'Billing',
  description: 'See your subscription, payment method, and payment records in one place.',
  nav: 'Billing',
  subscription: {
    heading: 'Subscription',
    empty: 'There is no active subscription.',
    choosePlans: 'View plans',
  },
  paymentMethod: {
    heading: 'Payment method',
    empty: 'There is no registered payment method.',
  },
  history: {
    heading: 'Payment and grant history',
    empty: 'There is no payment history yet.',
  },
  purchases: {
    heading: 'Credit purchases',
    empty: 'There are no credit purchases yet.',
  },
  loadFailed: 'Billing information could not be loaded.',
} as const
