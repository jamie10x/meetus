import { ImageResponse } from "next/og";

// Generated at build time (no request-time APIs used), so this never
// costs a render on the visitor's request path. Only 192/512 exist —
// the two sizes manifest.ts actually references — anything else falls
// back to 192 rather than erroring.
export function generateStaticParams() {
  return [{ size: "192" }, { size: "512" }];
}

export async function GET(
  _req: Request,
  { params }: { params: Promise<{ size: string }> },
) {
  const { size } = await params;
  const dimension = size === "512" ? 512 : 192;

  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "#160f16",
          borderRadius: dimension * 0.18,
        }}
      >
        <div
          style={{
            display: "flex",
            fontSize: dimension * 0.56,
            fontWeight: 900,
            fontFamily: "sans-serif",
            fontStyle: "italic",
            color: "#3fd8c9",
          }}
        >
          M
        </div>
      </div>
    ),
    { width: dimension, height: dimension },
  );
}
