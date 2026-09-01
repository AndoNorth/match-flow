import { LiveMatchList } from "@/components/LiveMatchList";
import { listMatches } from "@/lib/gateway/client";

export const dynamic = "force-dynamic";

export default async function Home() {
  const matches = await listMatches();
  return (
    <main className="p-8">
      <LiveMatchList initialMatches={matches} />
    </main>
  );
}
