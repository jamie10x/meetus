import { test, expect } from '@playwright/test';

test('worker purges legacy private data, bypasses APIs, and keeps the offline ticket shell', async ({ page, context }) => {
 await page.goto('/en/tickets');
 await page.evaluate(async () => {
  const old = await caches.open('meetus-v1');
  await old.put('/api/me/tickets', new Response('private old account'));
  await navigator.serviceWorker.register('/sw.js');
  await navigator.serviceWorker.ready;
 });
 await page.waitForFunction(() => !!navigator.serviceWorker.controller);
 await expect.poll(() => page.evaluate(() => caches.keys())).toEqual([]);
 const accounts = await page.evaluate(async () => {
  const first = await (await fetch('/api/me/tickets', { headers: { Authorization: 'Bearer A' } })).json();
  const second = await (await fetch('/api/me/tickets', { headers: { Authorization: 'Bearer B' } })).json();
  return [first.account, second.account];
 });
 expect(accounts).toEqual(['Bearer A', 'Bearer B']);
 await page.reload();
 await context.setOffline(true);
 await page.reload();
 await expect(page.locator('h1')).toHaveText('Ticket shell');
 const leaked = await page.evaluate(async () => {
  for (const name of await caches.keys()) {
   for (const request of await (await caches.open(name)).keys()) if (new URL(request.url).pathname.startsWith('/api/')) return true;
  }
  try { await fetch('/api/me/tickets'); return true; } catch { return false; }
 });
 expect(leaked).toBe(false);
});
