/**
 * Translates error messages from backend.
 * If the error message starts with "error.", it's treated as an i18n key.
 * Otherwise, the raw message is returned as-is.
 */
export function translateError(err: unknown, t: (key: string) => string): string {
  const msg = String(err)
  if (msg.startsWith('error.')) {
    return t(msg)
  }
  return msg
}
