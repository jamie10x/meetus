import { test, expect, type Page, type BrowserContext } from '@playwright/test';
import { createHash, createHmac } from 'node:crypto';

const API = 'http://localhost:58080/api';
function signedUser(id: number, name: string) {
  const fields: Record<string, string> = { id: String(id), first_name: name, auth_date: String(Math.floor(Date.now() / 1000)) };
  fields.hash = createHmac('sha256', createHash('sha256').update('').digest())
    .update(Object.keys(fields).sort().map(key => `${key}=${fields[key]}`).join('\n')).digest('hex');
  return fields;
}
async function stubTelegram(context: BrowserContext) {
  // Exercise our widget callback and actual API verification without Telegram
  // network requests, accounts, or outgoing messages.
  await context.route('https://telegram.org/**', route => route.fulfill({ contentType: 'application/javascript', body: '' }));
}
async function widgetLogin(page: Page, id: number, name: string) {
  await page.waitForFunction(() => typeof window.onTelegramAuth === 'function');
  await page.evaluate(fields => window.onTelegramAuth!(fields), signedUser(id, name));
  await expect(page).not.toHaveURL(/\/login/);
}
async function tickets(page: Page) {
  const token = await page.evaluate(() => localStorage.getItem('meetus.accessToken'));
  const response = await page.request.get(`${API}/me/tickets`, { headers: { Authorization: `Bearer ${token}` } });
  expect(response.ok()).toBeTruthy();
  return (await response.json()).data as { eventId: number; qr: string }[];
}

test('organizer creates and publishes; attendees join, cancel, promote and check in', async ({ browser }) => {
  const stamp = Date.now();
  const contexts = await Promise.all([0, 1, 2].map(() => browser.newContext({ baseURL: 'http://localhost:3000', timezoneId: 'Asia/Tashkent' })));
  const errors: string[] = [];
  try {
    for (const context of contexts) await stubTelegram(context);
    const [owner, attendee, waiting] = await Promise.all(contexts.map(context => context.newPage()));
    for (const page of [owner, attendee, waiting]) page.on('pageerror', error => errors.push(error.message));
    await owner.goto('/en/login');
    await widgetLogin(owner, stamp, 'Journey organizer');
    await owner.goto('/en/organizer');
    await owner.getByLabel('Organizer name').fill(`Journey ${stamp}`);
    await owner.getByRole('button', { name: 'Create organizer profile', exact: true }).click();
    await owner.getByRole('link', { name: /New event/ }).click();
    const title = `Journey event ${stamp}`;
    await owner.getByLabel('Title', { exact: false }).fill(title);
    await owner.getByRole('button', { name: 'Online', exact: true }).click();
    await owner.getByLabel('Meeting link', { exact: false }).fill('https://example.com/private-meeting');
    await owner.getByRole('button', { name: 'Tech', exact: true }).click();
    // Setting time chooses today; then explicitly select the intended date
    // (including midnight/month-boundary runs) using the real calendar UI.
    const start = new Date(Date.now() + 60 * 60 * 1000);
    const date = new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Tashkent', year: 'numeric', month: '2-digit', day: '2-digit' }).format(start);
    const time = new Intl.DateTimeFormat('en-GB', { timeZone: 'Asia/Tashkent', hour: '2-digit', minute: '2-digit', hourCycle: 'h23' }).format(start);
    await owner.getByLabel('Starts at: time', { exact: true }).fill(time);
    await owner.getByRole('button', { name: 'Starts at: date', exact: true }).click();
    const day = owner.locator(`[data-day="${date}"] button`).first();
    if (!(await day.count())) await owner.getByRole('button', { name: /next month/i }).click();
    await day.click();
    await owner.getByRole('button', { name: 'Limited', exact: true }).click();
    await owner.locator('input[type="number"]').fill('1');
    await owner.getByText('Preview event', { exact: true }).click();
    await expect(owner.getByRole('heading', { name: title, exact: true })).toBeVisible();
    await owner.getByRole('button', { name: 'Create draft', exact: true }).click();
    await owner.getByRole('link', { name: title, exact: true }).click();
    await expect(owner).toHaveURL(/\/organizer\/events\/\d+\/edit$/);
    const eventId = Number(new URL(owner.url()).pathname.split('/').at(-2));
    expect(eventId).toBeGreaterThan(0);
    await owner.getByRole('button', { name: 'Publish', exact: true }).click();
    await expect(owner.getByRole('button', { name: 'Unpublish', exact: true })).toBeVisible();

    await attendee.goto(`/en/events/${eventId}`);
    await expect(attendee.getByRole('link', { name: 'Join the call' })).toHaveCount(0);
    await attendee.getByRole('link', { name: 'Sign in with Telegram', exact: true }).click();
    await widgetLogin(attendee, stamp + 1, 'Journey attendee');
    await expect(attendee).toHaveURL(new RegExp(`/en/events/${eventId}$`));
    await attendee.getByRole('button', { name: 'Join event', exact: true }).click();
    await expect(attendee.getByText("You're going!", { exact: false })).toBeVisible();
    await expect(attendee.getByRole('link', { name: 'Join the call' })).toHaveAttribute('href', 'https://example.com/private-meeting');
    const firstTicket = (await tickets(attendee)).find(ticket => ticket.eventId === eventId)!;

    await waiting.goto('/en/login');
    await widgetLogin(waiting, stamp + 2, 'Journey waitlisted');
    await waiting.goto(`/en/events/${eventId}`);
    await waiting.getByRole('button', { name: 'Join waitlist', exact: true }).click();
    await expect(waiting.getByText("You're on the waitlist", { exact: false })).toBeVisible();
    await expect(waiting.getByRole('link', { name: 'Join the call' })).toHaveCount(0);
    await attendee.getByRole('button', { name: 'Cancel', exact: true }).click();
    await expect(attendee.getByRole('link', { name: 'View your ticket', exact: true })).toHaveCount(0);
    await waiting.evaluate(() => window.dispatchEvent(new Event("focus")));
    await expect(waiting.getByText("You're going!", { exact: false })).toBeVisible();
    const promoted = (await tickets(waiting)).find(ticket => ticket.eventId === eventId)!;
    expect(promoted.qr).toBeTruthy();
    await waiting.getByRole('link', { name: 'View your ticket', exact: true }).click();
    await expect(waiting.getByRole('img', { name: `Ticket QR for ${title}`, exact: true })).toBeVisible();
    await waiting.waitForFunction(() => !!navigator.serviceWorker.controller);
    await waiting.reload(); // Cache a real navigation shell, not just an RSC transition.
    await expect(waiting.getByRole('img', { name: `Ticket QR for ${title}`, exact: true })).toBeVisible();
    await contexts[2].setOffline(true);
    await waiting.reload();
    await expect(waiting.getByRole('img', { name: `Ticket QR for ${title}`, exact: true })).toBeVisible();
    await contexts[2].setOffline(false);
    await waiting.getByRole('button', { name: 'Log out', exact: true }).click();
    await expect.poll(() => waiting.evaluate(() => localStorage.getItem('meetus.offline.v2'))).toBeNull();
    await waiting.goto('/en/login');
    await widgetLogin(waiting, stamp + 1, 'Journey attendee');
    await waiting.goto('/en/tickets');
    await expect(waiting.getByRole('img', { name: `Ticket QR for ${title}`, exact: true })).toHaveCount(0);

    await owner.goto(`/en/organizer/events/${eventId}/scan`);
    await owner.getByLabel('Paste ticket QR code').fill(firstTicket.qr);
    await owner.getByRole('button', { name: 'Check in', exact: true }).click();
    await expect(owner.getByText('Journey attendee', { exact: true })).toHaveCount(0);
    await expect(owner.getByRole('status')).toContainText(/canceled|active|not found/i);
    await owner.getByLabel('Paste ticket QR code').fill(promoted.qr);
    await owner.getByRole('button', { name: 'Check in', exact: true }).click();
    await expect(owner.getByRole('status')).toContainText('Journey waitlisted');
    await expect(owner.getByText('1 checked in this session')).toBeVisible();
    expect(errors).toEqual([]);
  } finally { for (const context of contexts) await context.close().catch(() => undefined); }
});

test('Explore map renders and its popup preserves the locale', async ({ browser, request }) => {
  const stamp = Date.now();
  const login = await request.post(`${API}/auth/telegram`, { data: signedUser(stamp, 'Map fixture') });
  expect(login.ok()).toBeTruthy();
  const headers = { Authorization: `Bearer ${(await login.json()).data.tokens.accessToken}` };
  expect((await request.post(`${API}/organizers`, { headers, data: { displayName: `Map fixture ${stamp}` } })).ok()).toBeTruthy();
  const title = `Map fixture ${stamp}`;
  const created = await request.post(`${API}/events`, { headers, data: {
    title, categoryId: 1, cityId: 1, isOnline: false, startsAt: new Date(Date.now() + 3_600_000).toISOString(), lat: 41.3, lng: 69.25,
  } });
  expect(created.ok()).toBeTruthy();
  const id = (await created.json()).data.id;
  expect((await request.post(`${API}/events/${id}/publish`, { headers })).ok()).toBeTruthy();
  const context = await browser.newContext({ baseURL: 'http://localhost:3000' });
  try {
    await stubTelegram(context);
    // Test real MapLibre rendering deterministically without depending on a
    // public tile service's availability or consuming its tile bandwidth.
    await context.route('https://tiles.openfreemap.org/styles/dark', route => route.fulfill({ json: {
      version: 8, sources: {}, layers: [{ id: 'background', type: 'background', paint: { 'background-color': '#111111' } }],
    } }));
    const page = await context.newPage();
    await page.goto('/en/events');
    const filtered = page.waitForResponse(response => response.url().includes('/api/explore/events?') && new URL(response.url()).searchParams.get('q') === title);
    await page.getByLabel('Search events…').fill(title);
    await filtered;
    await expect(page.getByRole('heading', { name: title, exact: true }).last()).toBeVisible();
    await page.getByRole('button', { name: 'Map', exact: true }).click();
    await expect(page.locator('.maplibregl-canvas')).toBeVisible();
    const size = await page.locator('.maplibregl-canvas').boundingBox();
    expect(size!.height).toBeGreaterThan(400);
    await page.locator('.map-pin').click();
    await page.getByRole('button', { name: title, exact: true }).click();
    await expect(page).toHaveURL(new RegExp(`/en/events/${id}$`));
    await page.goBack();
    await expect(page.getByLabel('Search events…')).toHaveValue(title);
    await expect(page.getByRole('button', { name: 'Map', exact: true })).toHaveAttribute('aria-pressed', 'true');
  } finally { await context.close().catch(() => undefined); }
});

test('mobile Mini App shares attendance and language with the server', async ({ browser, request }) => {
  const stamp = Date.now();
  const login = await request.post(`${API}/auth/telegram`, { data: signedUser(stamp, 'Mini App host') });
  const headers = { Authorization: `Bearer ${(await login.json()).data.tokens.accessToken}` };
  expect((await request.post(`${API}/organizers`, { headers, data: { displayName: `Mini host ${stamp}` } })).ok()).toBeTruthy();
  const title = `Mini App gathering ${stamp}`;
  const created = await request.post(`${API}/events`, { headers, data: {
    title, categoryId: 1, cityId: 1, isOnline: false, locationName: 'Tashkent community hall', startsAt: new Date(Date.now() + 3_600_000).toISOString(),
  } });
  expect(created.ok()).toBeTruthy();
  const id = (await created.json()).data.id;
  expect((await request.post(`${API}/events/${id}/publish`, { headers })).ok()).toBeTruthy();
  const fields = { auth_date: String(Math.floor(Date.now() / 1000)), user: JSON.stringify({ id: stamp + 1, first_name: 'Mini attendee', language_code: 'en' }) };
  const secret = createHmac('sha256', 'WebAppData').update('').digest();
  const hash = createHmac('sha256', secret).update(Object.entries(fields).sort(([a], [b]) => a.localeCompare(b)).map(([key, value]) => `${key}=${value}`).join('\n')).digest('hex');
  const initData = new URLSearchParams({ ...fields, hash }).toString();
  const context = await browser.newContext({ baseURL: 'http://localhost:3000', viewport: { width: 390, height: 844 }, isMobile: true });
  try {
    await stubTelegram(context);
    await context.addInitScript(data => {
      let onMain: (() => void) | undefined;
      const main = { text: '', isVisible: false, isActive: true, isProgressVisible: false,
        show() { this.isVisible = true; }, hide() { this.isVisible = false; }, enable() { this.isActive = true; }, disable() { this.isActive = false; },
        setText(text: string) { this.text = text; }, onClick(fn: () => void) { onMain = fn; }, offClick(fn: () => void) { if (fn === onMain) onMain = undefined; },
        showProgress() { this.isProgressVisible = true; }, hideProgress() { this.isProgressVisible = false; },
      };
      window.Telegram = { WebApp: { initData: data, ready() {}, expand() {}, colorScheme: 'dark', themeParams: {}, setHeaderColor() {}, setBackgroundColor() {}, MainButton: main,
        BackButton: { isVisible: false, show() {}, hide() {}, onClick() {}, offClick() {} },
      } };
      Object.assign(window, { clickTelegramMain: () => onMain?.() });
    }, initData);
    const page = await context.newPage();
    await page.route(`**/api/events/${id}/rsvp`, route => route.fulfill({ status: 503, json: { error: { code: 'unavailable', message: 'Fixture outage' } } }));
    await page.goto(`/en/events/${id}`);
    await expect(page.getByText('Could not verify your attendance.', { exact: false })).toBeVisible();
    expect(await page.evaluate(() => window.Telegram?.WebApp?.MainButton.isVisible)).toBe(false);
    await page.unroute(`**/api/events/${id}/rsvp`);
    await page.getByRole('button', { name: 'Try again', exact: true }).click();
    await expect.poll(() => page.evaluate(() => window.Telegram?.WebApp?.MainButton.text)).toBe('Join event');
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeHidden();
    await page.screenshot({ path: test.info().outputPath('mini-event.png'), fullPage: true });
    await page.evaluate(() => (window as unknown as { clickTelegramMain: () => void }).clickTelegramMain());
    await expect(page.getByText("You're going!", { exact: false })).toBeVisible();
    await expect(page.getByRole('navigation', { name: 'Main navigation' })).toBeVisible();
    await page.getByRole('link', { name: 'View your ticket', exact: true }).click();
    await expect(page.getByRole('img', { name: `Ticket QR for ${title}` })).toBeVisible();
    await page.screenshot({ path: test.info().outputPath('mobile-ticket.png'), fullPage: true });
    const token = await page.evaluate(() => localStorage.getItem('meetus.accessToken'));
    const attendeeHeaders = { Authorization: `Bearer ${token}` };
    await page.getByLabel('Language for website and bot').selectOption('ru');
    await expect(page).toHaveURL(/\/ru\/tickets$/);
    expect((await (await request.get(`${API}/me`, { headers: attendeeHeaders })).json()).data.language).toBe('ru');
    // Bot language changes use this same profile write; returning picks it up.
    expect((await request.patch(`${API}/me`, { headers: attendeeHeaders, data: { language: 'uz' } })).ok()).toBeTruthy();
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page).toHaveURL(/\/uz\/tickets$/);
    // Simulate another surface canceling this attendance, then returning here.
    expect((await request.delete(`${API}/events/${id}/rsvp`, { headers: attendeeHeaders })).ok()).toBeTruthy();
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page.locator('img[alt^="Ticket QR"]')).toHaveCount(0);
    await expect(page.locator('main img')).toHaveCount(0);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBeTruthy();
  } finally { await context.close(); }
});
