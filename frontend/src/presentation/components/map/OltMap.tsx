import { useState } from "react";
import {
  APIProvider,
  InfoWindow,
  Map,
  // Marker is the legacy pin on purpose: AdvancedMarker additionally needs a
  // Google Cloud Map ID, which this installation has not created, and without
  // one it renders nothing. Do not "modernise" this without creating the Map ID
  // first.
  Marker,
} from "@vis.gl/react-google-maps";
import { OltStatus, type Odc, type Odp, type Olt } from "@/domain/entities";
import { mappedOlts, type MappedOlt } from "./oltMapFilters";
import { PlantLayer } from "./PlantLayer";

interface OltMapProps {
  apiKey: string;
  olts: Olt[];
  /** Cabinets and distribution boxes, drawn alongside the OLTs. */
  odcs?: Odc[];
  odps?: Odp[];
  onSelectOdp?: (odp: Odp) => void;
  /** Where the operator clicked, when the map is being used to place plant. */
  onPlace?: (coordinates: { latitude: number; longitude: number }) => void;
}

// Indonesia, so an installation with no pins yet opens somewhere recognisable
// rather than in the Atlantic at 0,0.
const FALLBACK_CENTER = { lat: -2.5, lng: 118 };
const FALLBACK_ZOOM = 4;
// A single pin should not open at maximum zoom, where there is no context.
const SINGLE_PIN_ZOOM = 14;
// Keeps the outermost pins off the edge of the viewport.
const BOUNDS_PADDING = 48;

/**
 * Frames every pin. Depok and Bekasi are 40 km apart, so centring on one of
 * them at street zoom shows a single OLT and reads as though only one is
 * mapped — the very thing the unmapped panel exists to prevent.
 */
function framingOf(pins: MappedOlt[]) {
  if (pins.length === 0) {
    return { defaultCenter: FALLBACK_CENTER, defaultZoom: FALLBACK_ZOOM };
  }
  if (pins.length === 1) {
    return {
      defaultCenter: { lat: pins[0].latitude, lng: pins[0].longitude },
      defaultZoom: SINGLE_PIN_ZOOM,
    };
  }

  const latitudes = pins.map((pin) => pin.latitude);
  const longitudes = pins.map((pin) => pin.longitude);
  return {
    defaultBounds: {
      north: Math.max(...latitudes),
      south: Math.min(...latitudes),
      east: Math.max(...longitudes),
      west: Math.min(...longitudes),
      padding: BOUNDS_PADDING,
    },
  };
}

export function OltMap({
  apiKey,
  olts,
  odcs = [],
  odps = [],
  onSelectOdp,
  onPlace,
}: OltMapProps) {
  const [selected, setSelected] = useState<MappedOlt | null>(null);
  const pins = mappedOlts(olts);

  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={{ width: "100%", height: 520, borderRadius: 8 }}
        {...framingOf(pins)}
        gestureHandling="greedy"
        disableDefaultUI={false}
        onClick={(event) => {
          const point = event.detail.latLng;
          if (point && onPlace) {
            onPlace({ latitude: point.lat, longitude: point.lng });
          }
        }}
      >
        {pins.map((olt) => (
          <Marker
            key={olt.id}
            position={{ lat: olt.latitude, lng: olt.longitude }}
            title={olt.name}
            onClick={() => setSelected(olt)}
          />
        ))}

        <PlantLayer odcs={odcs} odps={odps} onSelectOdp={onSelectOdp} />

        {selected && (
          <InfoWindow
            position={{ lat: selected.latitude, lng: selected.longitude }}
            onCloseClick={() => setSelected(null)}
          >
            <OltSummary olt={selected} />
          </InfoWindow>
        )}
      </Map>
    </APIProvider>
  );
}

function OltSummary({ olt }: { olt: Olt }) {
  return (
    <div style={{ color: "#18181b", minWidth: 180 }}>
      <div style={{ fontWeight: 600 }}>{olt.name}</div>
      <div style={{ fontSize: 12 }}>Site: {olt.siteName}</div>
      <div style={{ fontSize: 12, marginTop: 6 }}>
        {olt.ipAddress} ·{" "}
        <span
          style={{
            color: olt.status === OltStatus.ONLINE ? "#15803d" : "#b91c1c",
          }}
        >
          {olt.status}
        </span>
      </div>
      <div style={{ fontSize: 12 }}>
        {olt.ontCount === 1 ? "1 ONT" : `${olt.ontCount ?? 0} ONTs`}
      </div>
    </div>
  );
}
