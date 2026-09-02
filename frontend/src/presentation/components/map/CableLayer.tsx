import { Polyline } from "@vis.gl/react-google-maps";
import type { CableSegment } from "./cableSegments";

interface CableLayerProps {
  segments: CableSegment[];
  selectedId?: string;
  onSelectCable?: (segment: CableSegment) => void;
}

// Traced paths are drawn solid and bright; the straight lines nobody has traced
// are thinner and faded, so at a glance the map says which lengths were
// measured and which are only the gap between two points. No dashes: dash
// patterns need google.maps.SymbolPath, which exists only after the script has
// run, and weight carries the same distinction with nothing to load.
const TRACED_COLOR = "#60a5fa";
const STRAIGHT_COLOR = "#71717a";
const SELECTED_COLOR = "#3ecf8e";

export function CableLayer({
  segments,
  selectedId,
  onSelectCable,
}: CableLayerProps) {
  return (
    <>
      {segments.map((segment) => {
        const selected = segment.id === selectedId;
        return (
          <Polyline
            key={segment.id}
            path={segment.path.map((point) => ({
              lat: point.lat,
              lng: point.lng,
            }))}
            strokeColor={
              selected
                ? SELECTED_COLOR
                : segment.traced
                  ? TRACED_COLOR
                  : STRAIGHT_COLOR
            }
            strokeWeight={selected ? 5 : segment.traced ? 3 : 2}
            strokeOpacity={segment.traced || selected ? 0.95 : 0.55}
            onClick={onSelectCable ? () => onSelectCable(segment) : undefined}
          />
        );
      })}
    </>
  );
}
