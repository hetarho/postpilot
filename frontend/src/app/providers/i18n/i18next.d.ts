import 'i18next'
import { defaultNS, resources } from './resources'

declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: typeof defaultNS
    resources: (typeof resources)['ko']
    // Both custom formats take the server's own string form — a micro-USD integer and an
    // RFC3339 instant — so a failure detail's params, which are always strings, can use them.
    interpolationFormatTypeMap: {
      microusd: string | number
      instant: string
      plan: string
    }
  }
}
