import { LiveMatchList } from "@/components/LiveMatchList";
import { listMatches } from "@/lib/gateway/client";

export const dynamic = "force-dynamic";

export default async function Home() {
  const matches = await listMatches();
  return (
    <main className="p-8">
      <h1 className="text-2xl font-bold mb-4">MatchFlow</h1>
      <LiveMatchList initialMatches={matches} />
    </main>
  );
}
