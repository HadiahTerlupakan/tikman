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
import { OltStatus, type Olt, type Site } from "@/domain/entities";
import { mappedSites, type MappedSite } from "./siteMapFilters";

interface SiteMapProps {
  apiKey: string;
  sites: Site[];
  olts: Olt[];
}

// Indonesia, so an installation with no pins yet opens somewhere recognisable
// rather than in the Atlantic at 0,0.
const FALLBACK_CENTER = { lat: -2.5, lng: 118 };
const FALLBACK_ZOOM = 4;
// A single pin should not open at maximum zoom, where there is no context.
const SINGLE_SITE_ZOOM = 14;
// Keeps the outermost pins off the edge of the viewport.
const BOUNDS_PADDING = 48;

/**
 * Frames every pin. Depok and Bekasi are 40 km apart, so centring on one of
 * them at street zoom shows a single site and reads as though only one is
 * mapped — the very thing the unmapped-sites panel exists to prevent.
 */
function framingOf(pins: MappedSite[]) {
  if (pins.length === 0) {
    return { defaultCenter: FALLBACK_CENTER, defaultZoom: FALLBACK_ZOOM };
  }
  if (pins.length === 1) {
    return {
      defaultCenter: { lat: pins[0].latitude, lng: pins[0].longitude },
      defaultZoom: SINGLE_SITE_ZOOM,
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

export function SiteMap({ apiKey, sites, olts }: SiteMapProps) {
  const [selected, setSelected] = useState<MappedSite | null>(null);
  const pins = mappedSites(sites);

  return (
    <APIProvider apiKey={apiKey}>
      <Map
        style={{ width: "100%", height: 520, borderRadius: 8 }}
        {...framingOf(pins)}
        gestureHandling="greedy"
        disableDefaultUI={false}
      >
        {pins.map((site) => (
          <Marker
            key={site.id}
            position={{ lat: site.latitude, lng: site.longitude }}
            title={site.name}
            onClick={() => setSelected(site)}
          />
        ))}

        {selected && (
          <InfoWindow
            position={{ lat: selected.latitude, lng: selected.longitude }}
            onCloseClick={() => setSelected(null)}
          >
            <SiteSummary site={selected} olts={olts} />
          </InfoWindow>
        )}
      </Map>
    </APIProvider>
  );
}

function SiteSummary({ site, olts }: { site: Site; olts: Olt[] }) {
  const owned = olts.filter((olt) => olt.siteId === site.id);
  const online = owned.filter((olt) => olt.status === OltStatus.ONLINE).length;

  return (
    <div style={{ color: "#18181b", minWidth: 160 }}>
      <div style={{ fontWeight: 600 }}>{site.name}</div>
      <div style={{ fontSize: 12 }}>{site.location || "no address"}</div>
      <div style={{ fontSize: 12, marginTop: 6 }}>
        {owned.length === 0
          ? "No OLTs at this site"
          : `${online} of ${owned.length} OLTs online`}
      </div>
    </div>
  );
}
