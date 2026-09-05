import { useState } from "react";
import { AdvancedMarker, InfoWindow, Pin } from "@vis.gl/react-google-maps";
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

const ODC_COLOR = "#60a5fa";

// The pin's outline and its letter, dark against every fill the plant uses.
const PIN_INK = "#0a0a0a";

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
        <AdvancedMarker
          key={odc.id}
          position={{ lat: odc.latitude, lng: odc.longitude }}
          title={odc.code}
          onClick={() => setSelected({ kind: "odc", item: odc })}
        >
          <Pin
            background={ODC_COLOR}
            borderColor={PIN_INK}
            glyphColor={PIN_INK}
          >
            C
          </Pin>
        </AdvancedMarker>
      ))}

      {boxes.map((odp) => (
        <AdvancedMarker
          key={odp.id}
          position={{ lat: odp.latitude, lng: odp.longitude }}
          title={`${odp.code} — ${odpOccupancyLabel(odp)}`}
          onClick={() => setSelected({ kind: "odp", item: odp })}
        >
          <Pin
            background={odpPinColor(odp)}
            borderColor={PIN_INK}
            glyphColor={PIN_INK}
          >
            P
          </Pin>
        </AdvancedMarker>
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
              title={selected.item.code}
              detail={odcSummaryLabel(selected.item)}
            />
          ) : (
            <PlantSummary
              title={selected.item.code}
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
  detail,
  onOpen,
}: {
  title: string;
  detail: string;
  onOpen?: () => void;
}) {
  return (
    <div style={{ color: "#18181b", minWidth: 170 }}>
      <div style={{ fontWeight: 600 }}>{title}</div>
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
