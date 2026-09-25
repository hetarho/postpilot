/** The full gift link for a voucher token, on the origin the operator is using. */
export function giftLinkFor(token: string, origin: string): string {
  return `${origin}/gift/${encodeURIComponent(token)}`
}
