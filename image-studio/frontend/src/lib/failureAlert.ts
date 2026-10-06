// Alert on the first failure of a submission, including a loop or folder batch.
// A new submission gets a new gate, so later failures are never permanently muted.
export function createFailureAlertGate(): (message: string) => boolean {
  let alerted = false;
  return (message) => {
    if (alerted || /\b(?:context canceled|cancelled|canceled|AbortError)\b|已取消|用户取消/i.test(message)) {
      return false;
    }
    alerted = true;
    return true;
  };
}
