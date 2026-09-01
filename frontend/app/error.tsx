"use client";

export default function ErrorBoundary({
  reset,
}: {
  error: Error;
  reset: () => void;
}) {
  return (
    <main className="p-8">
      <p>Something went wrong.</p>
      <button type="button" className="btn" onClick={() => reset()}>
        Try again
      </button>
    </main>
  );
}
