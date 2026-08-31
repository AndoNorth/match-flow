import Link from "next/link";

export default function MatchNotFound() {
  return (
    <main className="p-8">
      <p>Match not found.</p>
      <Link href="/" className="link">
        Back to matches
      </Link>
    </main>
  );
}
