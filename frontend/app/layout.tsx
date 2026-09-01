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
      <body className="bg-base-100 text-base-content min-h-screen">
        {children}
      </body>
    </html>
  );
}
