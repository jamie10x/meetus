import { ImageResponse } from "next/og";

export const size = { width: 180, height: 180 };
export const contentType = "image/png";

export default function AppleIcon() {
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
        }}
      >
        <div
          style={{
            display: "flex",
            fontSize: 100,
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
    { ...size },
  );
}
