import { useCallback, useEffect, useRef } from "react";

const REQUIRED_TAPS = 4;
const RESET_MS = 2000;
const DEFAULT_EDGE_PX = 40;

/**
 * Fires `onTriggered` when the user taps / clicks any screen edge
 * `REQUIRED_TAPS` times within `RESET_MS` milliseconds.
 *
 * "Edge" is defined as within `edgeThreshold` pixels of any of the four
 * sides of the viewport.
 *
 * @param onTriggered  Callback fired when the gesture is recognised.
 * @param edgeThreshold  Distance from any edge that counts as an edge tap (default 40 px).
 * @param enabled  Set to false to disable the listener (e.g. when PIN modal is open).
 */
export function useEdgeTap(
  onTriggered: () => void,
  edgeThreshold = DEFAULT_EDGE_PX,
  enabled = true,
) {
  const tapCountRef = useRef(0);
  const resetTimerRef = useRef<number | null>(null);

  const resetCount = useCallback(() => {
    tapCountRef.current = 0;
    if (resetTimerRef.current !== null) {
      clearTimeout(resetTimerRef.current);
      resetTimerRef.current = null;
    }
  }, []);

  useEffect(() => {
    if (!enabled) {
      resetCount();
      return;
    }

    const handlePointerDown = (e: PointerEvent) => {
      const { clientX, clientY } = e;
      const w = window.innerWidth;
      const h = window.innerHeight;

      const isEdge =
        clientX <= edgeThreshold ||
        clientY <= edgeThreshold ||
        clientX >= w - edgeThreshold ||
        clientY >= h - edgeThreshold;

      if (!isEdge) {
        resetCount();
        return;
      }

      tapCountRef.current += 1;

      // Reset the decay timer on every valid edge tap
      if (resetTimerRef.current !== null) {
        clearTimeout(resetTimerRef.current);
      }
      resetTimerRef.current = window.setTimeout(resetCount, RESET_MS);

      if (tapCountRef.current >= REQUIRED_TAPS) {
        resetCount();
        onTriggered();
      }
    };

    window.addEventListener("pointerdown", handlePointerDown);
    return () => {
      window.removeEventListener("pointerdown", handlePointerDown);
    };
  }, [enabled, edgeThreshold, onTriggered, resetCount]);
}
