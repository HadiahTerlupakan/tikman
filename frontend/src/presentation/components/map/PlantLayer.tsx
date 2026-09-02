import { useState } from "react";
import { InfoWindow, Marker } from "@vis.gl/react-google-maps";
import type { Odc, Odp } from "@/domain/entities";
import {
  mappedPlant,
  odcSummaryLabel,
  odpOccupancyLabel,
  odpPinColor,
  type Placed,
} from "./plantMarkers";

interface PlantLayerProps {
  odcs: Odc[];
  odps: Odp[];
  onSelectOdp?: (odp: Odp) => void;
}

// An inline SVG rather than google.maps.SymbolPath: the symbol constants only
// exist once the Maps script has run, and a data URI needs nothing loaded.
function pinIcon(color: string, letter: string): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="26" height="26">
    <circle cx="13" cy="13" r="11" fill="${color}" stroke="#0a0a0a" stroke-width="2"/>
    <text x="13" y="17" font-family="sans-serif" font-size="11" font-weight="bold"
      text-anchor="middle" fill="#0a0a0a">${letter}</text>
  </svg>`;
  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`;
}

const ODC_COLOR = "#60a5fa";

type Selection =
  | { kind: "odc"; item: Placed<Odc> }
  | { kind: "odp"; item: Placed<Odp> };

/**
 * Draws the fibre plant on the map: cabinets, and the distribution boxes a
 * subscriber's drop lands in.
 *
 * A box's colour carries how much room is left in it, so the question a
 * technician actually has — where can this new subscriber go — is answered by
 * looking, before anything is clicked.
 */
export function PlantLayer({ odcs, odps, onSelectOdp }: PlantLayerProps) {
  const [selected, setSelected] = useState<Selection | null>(null);
  const cabinets = mappedPlant(odcs);
  const boxes = mappedPlant(odps);

  return (
    <>
      {cabinets.map((odc) => (
        <Marker
          key={odc.id}
          position={{ lat: odc.latitude, lng: odc.longitude }}
          title={odc.name}
          icon={pinIcon(ODC_COLOR, "C")}
          onClick={() => setSelected({ kind: "odc", item: odc })}
        />
      ))}

      {boxes.map((odp) => (
        <Marker
          key={odp.id}
          position={{ lat: odp.latitude, lng: odp.longitude }}
          title={`${odp.name} — ${odpOccupancyLabel(odp)}`}
          icon={pinIcon(odpPinColor(odp), "P")}
          onClick={() => setSelected({ kind: "odp", item: odp })}
        />
      ))}

      {selected && (
        <InfoWindow
          position={{
            lat: selected.item.latitude,
            lng: selected.item.longitude,
          }}
          onCloseClick={() => setSelected(null)}
        >
          {selected.kind === "odc" ? (
            <PlantSummary
              title={selected.item.name}
              code={selected.item.code}
              detail={odcSummaryLabel(selected.item)}
            />
          ) : (
            <PlantSummary
              title={selected.item.name}
              code={selected.item.code}
              detail={odpOccupancyLabel(selected.item)}
              onOpen={
                onSelectOdp
                  ? () => onSelectOdp(selected.item as Odp)
                  : undefined
              }
            />
          )}
        </InfoWindow>
      )}
    </>
  );
}

function PlantSummary({
  title,
  code,
  detail,
  onOpen,
}: {
  title: string;
  code: string;
  detail: string;
  onOpen?: () => void;
}) {
  return (
    <div style={{ color: "#18181b", minWidth: 170 }}>
      <div style={{ fontWeight: 600 }}>{title}</div>
      {code && <div style={{ fontSize: 12 }}>{code}</div>}
      <div style={{ fontSize: 12, marginTop: 6 }}>{detail}</div>
      {onOpen && (
        <button
          type="button"
          onClick={onOpen}
          style={{
            marginTop: 8,
            border: "none",
            background: "transparent",
            color: "#1d4ed8",
            cursor: "pointer",
            padding: 0,
            fontSize: 12,
          }}
        >
          Lihat pelanggan
        </button>
      )}
    </div>
  );
}
