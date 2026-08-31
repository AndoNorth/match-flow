import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "MatchFlow",
  description: "Live match scores and events",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" data-theme="night">
      <body>{children}</body>
    </html>
  );
}
