import { notFound } from "next/navigation";
import { LiveMatchDetail } from "@/components/LiveMatchDetail";
import { GatewayError, getMatch, listMatchEvents } from "@/lib/gateway/client";
import type { MatchBody } from "@/lib/gateway/types";

export const dynamic = "force-dynamic";

export default async function MatchDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let match: MatchBody;
  try {
    match = await getMatch(id);
  } catch (error) {
    if (error instanceof GatewayError && error.status === 404) {
      notFound();
    }
    throw error;
  }

  const events = await listMatchEvents(id);

  return (
    <main className="p-8">
      <LiveMatchDetail key={id} initialMatch={match} initialEvents={events} />
    </main>
  );
}
