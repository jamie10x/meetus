import type { Metadata } from "next";
import { notFound } from "next/navigation";
import Script from "next/script";
import { NextIntlClientProvider } from "next-intl";
import { getMessages, setRequestLocale } from "next-intl/server";
import { Fraunces, Hanken_Grotesk, IBM_Plex_Mono } from "next/font/google";
import "./globals.css";
import { routing, type AppLocale } from "@/i18n/routing";
import { AuthProvider } from "@/lib/auth-context";
import Header from "@/components/Header";
import Footer from "@/components/Footer";
import TelegramChrome from "@/components/TelegramChrome";
import ServiceWorkerRegister from "@/components/ServiceWorkerRegister";

const fraunces = Fraunces({
  variable: "--font-fraunces",
  subsets: ["latin"],
  style: ["normal", "italic"],
});

const hanken = Hanken_Grotesk({
  variable: "--font-hanken",
  subsets: ["latin"],
});

const plexMono = IBM_Plex_Mono({
  variable: "--font-plex-mono",
  subsets: ["latin"],
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  title: {
    default: "Meetus.uz — events and meetups in Uzbekistan",
    template: "%s · Meetus.uz",
  },
  description:
    "Discover events, join meetups, and grow communities across Uzbekistan.",
};

export const viewport = {
  themeColor: "#070b16",
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
      className={`${fraunces.variable} ${hanken.variable} ${plexMono.variable} h-full antialiased`}
      // The Telegram Mini App SDK (loaded below) sets --tg-viewport-*
      // CSS vars on this element as soon as it runs, on every browser —
      // an expected, benign mismatch against the server-rendered markup.
      suppressHydrationWarning
    >
      <body className="min-h-full flex flex-col bg-ink text-bone">
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
            <ServiceWorkerRegister />
            <TelegramChrome />
            <Header />
            <div className="relative z-[1] flex-1">{children}</div>
            <Footer />
          </AuthProvider>
        </NextIntlClientProvider>
      </body>
    </html>
  );
}
