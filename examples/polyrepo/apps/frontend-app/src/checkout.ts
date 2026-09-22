export async function submitCheckout() {
  return fetch('/api/checkout', { method: 'POST' });
}
