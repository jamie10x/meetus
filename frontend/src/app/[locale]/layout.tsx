import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Script from "next/script";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, setRequestLocale } from "next-intl/server";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { routing, type AppLocale } from "@/i18n/routing";
import { AuthProvider } from "@/lib/auth-context";
import Header from "@/components/Header";
import TelegramChrome from "@/components/TelegramChrome";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: {
    default: "Meetus.uz — events and meetups in Uzbekistan",
    template: "%s · Meetus.uz",
  },
  description:
    "Discover events, join meetups, and grow communities across Uzbekistan.",
};

export function generateStaticParams() {
  return routing.locales.map((locale) => ({ locale }));
}

export default async function LocaleLayout({
  children,
  params,
}: Readonly<{
  children: React.ReactNode;
  params: Promise<{ locale: string }>;
}>) {
  const { locale } = await params;
  if (!routing.locales.includes(locale as AppLocale)) {
    notFound();
  }

  // Enables static rendering for this locale (next-intl requirement when
  // using the async params pattern in a Server Component).
  setRequestLocale(locale as AppLocale);

  const messages = await getMessages();

  return (
    <html
      lang={locale}
      className={`${geistSans.variable} ${geistMono.variable} h-full antialiased`}
      // The Telegram Mini App SDK (loaded below) sets --tg-viewport-*
      // CSS vars on this element as soon as it runs, on every browser —
      // an expected, benign mismatch against the server-rendered markup.
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col">
        {/*
          Telegram Mini App SDK. "beforeInteractive" guarantees
          window.Telegram.WebApp exists by the time AuthProvider's mount
          effect runs (no polling/guessing needed for auto-login). Its one
          side effect: the SDK sets viewport CSS vars on <html> as soon as
          it runs, regardless of context — that's a known, benign mismatch
          against the server-rendered <html>, suppressed via
          suppressHydrationWarning below rather than worked around here.
        */}
        <Script
          src="https://telegram.org/js/telegram-web-app.js"
          strategy="beforeInteractive"
        />
        <NextIntlClientProvider messages={messages}>
          <AuthProvider>
            <TelegramChrome />
            <Header />
            <div className="flex-1">{children}</div>
          </AuthProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
