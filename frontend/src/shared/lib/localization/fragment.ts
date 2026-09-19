/** One slice's share of one i18n namespace (ARCH-16).
 *
 *  Namespaces are domain-shaped and stay so — `useTranslation('posts')` is unchanged — but the
 *  STRINGS live with the slice that renders them, so a feature's copy change edits that feature
 *  rather than a 1,100-line file in `app/`. `app/providers/i18n` only assembles the fragments,
 *  and `resources.test.ts` refuses two fragments of one namespace that claim the same key. */
export interface I18nFragment {
  /** The namespace this fragment contributes to. */
  readonly namespace: string
  readonly ko: Readonly<Record<string, unknown>>
  readonly en: Readonly<Record<string, unknown>>
}
